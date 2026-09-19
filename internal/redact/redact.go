// Package redact strips secrets and machine-identifying paths from transcript
// text at the ingest boundary.
//
// Everything ritual reads is somebody's working history: API keys pasted into a
// prompt, .env files echoed by a shell tool, customer records in a fixture,
// the operator's account name in every absolute path. None of that is needed to
// mine a workflow, and all of it is a liability the moment a report is written
// to disk or shown in a browser. Adapters run every string they emit through
// Text, so nothing downstream — the cache, the report, the generated skill, the
// local dashboard — can hold a credential that ritual itself introduced.
//
// This is a filter, not a guarantee. A secret with no recognizable shape (a
// bare password, an internal hostname) survives. The docs say so plainly, and
// the redaction rules are deliberately boring and inspectable rather than
// clever.
package redact

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Placeholder is what a redacted span is replaced with. It is distinctive so a
// reader can tell redaction happened rather than assuming the source said this.
const Placeholder = "[redacted]"

type rule struct {
	name string
	re   *regexp.Regexp
	// repl is the replacement template. Capture group 1, when the pattern has
	// one, is preserved so `export TOKEN=` keeps its assignment shape.
	repl string
}

var rules = []rule{
	// Provider-shaped keys first: they are unambiguous and worth naming.
	{"anthropic-key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_\-]{16,}`), "[redacted:anthropic-key]"},
	{"openai-key", regexp.MustCompile(`sk-(?:proj-|svcacct-|admin-)?[A-Za-z0-9_\-]{20,}`), "[redacted:openai-key]"},
	{"github-token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{16,}`), "[redacted:github-token]"},
	{"github-pat", regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`), "[redacted:github-token]"},
	{"gitlab-token", regexp.MustCompile(`glpat-[A-Za-z0-9_\-]{16,}`), "[redacted:gitlab-token]"},
	{"slack-token", regexp.MustCompile(`xox[baprs]-[A-Za-z0-9\-]{10,}`), "[redacted:slack-token]"},
	{"stripe-key", regexp.MustCompile(`(?:sk|rk|pk)_(?:live|test)_[A-Za-z0-9]{16,}`), "[redacted:stripe-key]"},
	{"shopify-token", regexp.MustCompile(`shp(?:at|ca|pa|ss)_[A-Fa-f0-9]{32}`), "[redacted:shopify-token]"},
	{"google-key", regexp.MustCompile(`AIza[A-Za-z0-9_\-]{35}`), "[redacted:google-key]"},
	{"aws-access-key", regexp.MustCompile(`(?:AKIA|ASIA|AGPA|AROA|AIDA)[A-Z0-9]{16}`), "[redacted:aws-key]"},
	{"npm-token", regexp.MustCompile(`npm_[A-Za-z0-9]{36}`), "[redacted:npm-token]"},
	{"vercel-token", regexp.MustCompile(`\bvercel_[A-Za-z0-9]{24,}\b`), "[redacted:vercel-token]"},
	{"hf-token", regexp.MustCompile(`hf_[A-Za-z0-9]{30,}`), "[redacted:hf-token]"},
	{"jwt", regexp.MustCompile(`eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}`), "[redacted:jwt]"},
	{"private-key", regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), "[redacted:private-key]"},
	{"basic-auth-url", regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://)[^\s/:@]+:[^\s/@]+@`), "${1}[redacted:credentials]@"},

	// Then assignment shapes: anything that calls itself a secret and is given
	// a value long enough to be one. The key name is kept; the value is not.
	{"assignment", regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:API[_-]?KEY|SECRET|TOKEN|PASSWORD|PASSWD|CREDENTIAL|PRIVATE[_-]?KEY|ACCESS[_-]?KEY|CLIENT[_-]?SECRET|AUTH)[A-Z0-9_]*)\s*[:=]\s*["']?([^\s"',;]{8,})["']?`), "${1}=[redacted]"},
	{"bearer", regexp.MustCompile(`(?i)\b(bearer|authorization:\s*bearer)\s+[A-Za-z0-9_\-\.=]{12,}`), "${1} [redacted]"},
}

var emailRE = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)

// Redactor rewrites text. The zero value is usable but does not know the home
// directory; use New.
type Redactor struct {
	home     string
	user     string
	keepMail bool
}

// New builds a Redactor for the current machine. Home-directory paths become
// ${HOME}-relative so a report can be read, shared, or diffed without carrying
// the operator's account name.
func New() *Redactor {
	r := &Redactor{}
	if home, err := os.UserHomeDir(); err == nil {
		r.home = filepath.Clean(home)
		r.user = filepath.Base(r.home)
	}
	return r
}

// KeepEmails disables email masking. Reports that stay on the machine that
// produced them may want the addresses; anything exported should not.
func (r *Redactor) KeepEmails(keep bool) *Redactor { r.keepMail = keep; return r }

// Text applies every rule in order and returns the rewritten string.
func (r *Redactor) Text(in string) string {
	if in == "" {
		return in
	}
	out := in
	for _, rl := range rules {
		out = rl.re.ReplaceAllString(out, rl.repl)
	}
	if !r.keepMail {
		out = emailRE.ReplaceAllString(out, "[redacted:email]")
	}
	out = r.Path(out)
	return out
}

// Path rewrites absolute home paths into ${HOME}/… form. It runs over free text
// too, so a shell transcript mentioning /Users/someone/code/app is rewritten in
// place rather than only when the path is the whole value.
func (r *Redactor) Path(in string) string {
	if r.home == "" || in == "" {
		return in
	}
	return strings.ReplaceAll(in, r.home, "${HOME}")
}

// Args redacts a tool-call argument map, dropping values that are too long to
// be a workflow signal. A 40 KB file body in a Write call teaches the miner
// nothing and is the single largest source of accidental content capture.
func (r *Redactor) Args(in map[string]string, maxValue int) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		v = r.Text(v)
		if len(v) > maxValue {
			v = v[:maxValue] + "…"
		}
		out[k] = v
	}
	return out
}

var defaultOnce struct {
	sync.Once
	r *Redactor
}

// Default is the process-wide Redactor. Adapters use it so a caller cannot
// forget to construct one.
func Default() *Redactor {
	defaultOnce.Do(func() { defaultOnce.r = New() })
	return defaultOnce.r
}

// Text redacts with the default Redactor.
func Text(in string) string { return Default().Text(in) }

// Path redacts with the default Redactor.
func Path(in string) string { return Default().Path(in) }
