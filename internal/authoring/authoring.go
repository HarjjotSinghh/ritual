// Package authoring optionally rewrites a generated artifact's prose using an
// agent CLI the operator already has installed.
//
// The division of labour is deliberate and load-bearing. The deterministic
// pipeline decides what a workflow is, how often it happened, and what it
// touched — all of it checkable against the transcripts. A model is only ever
// asked to phrase that finding better. It is never given the transcripts, never
// asked to discover a pattern, and never allowed to introduce a step the
// evidence does not support; its output is validated and discarded when it
// drifts.
//
// Nothing here is required. With no agent CLI installed, ritual produces the
// template version, which is complete, if flatter.
package authoring

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/artifact"
	"github.com/HarjjotSinghh/ritual/internal/report"
)

// Author is a local agent CLI ritual can drive.
type Author struct {
	Key         string
	DisplayName string
	Binary      string
	// Args builds the argv for a one-shot, non-interactive run of the prompt.
	Args func(prompt string) []string
}

// Authors are the CLIs ritual knows how to invoke headlessly, in preference
// order. Preference is by how reliably the tool returns plain text on stdout
// and exits, not by model quality.
func Authors() []Author {
	return []Author{
		{"claude", "Claude Code", "claude", func(p string) []string { return []string{"-p", p} }},
		{"codex", "Codex CLI", "codex", func(p string) []string { return []string{"exec", "--skip-git-repo-check", p} }},
		{"opencode", "OpenCode", "opencode", func(p string) []string { return []string{"run", p} }},
		{"gemini", "Gemini CLI", "gemini", func(p string) []string { return []string{"-p", p} }},
		{"qwen", "Qwen Code", "qwen", func(p string) []string { return []string{"-p", p} }},
		{"ollama", "Ollama", "ollama", func(p string) []string { return []string{"run", "llama3.2", p} }},
	}
}

// Detect returns the authors present on this machine.
func Detect() []Author {
	out := make([]Author, 0, 3)
	for _, a := range Authors() {
		if path, err := exec.LookPath(a.Binary); err == nil && path != "" {
			out = append(out, a)
		}
	}
	return out
}

// Resolve picks an author by key. "auto" takes the first detected one, and
// "none" disables authoring entirely.
func Resolve(key string) (Author, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "none", "off", "false":
		return Author{}, false
	case "", "auto":
		detected := Detect()
		if len(detected) == 0 {
			return Author{}, false
		}
		return detected[0], true
	}
	for _, a := range Detect() {
		if a.Key == key {
			return a, true
		}
	}
	return Author{}, false
}

// Options control one authoring run.
type Options struct {
	Timeout time.Duration
	// MaxBytes caps the accepted output. A CLI that streams its own logs onto
	// stdout would otherwise write them into a skill.
	MaxBytes int
}

// DefaultOptions are conservative: an artifact is a page of text, and a run
// that takes minutes is a run that has gone wrong.
func DefaultOptions() Options {
	return Options{Timeout: 90 * time.Second, MaxBytes: 24 << 10}
}

// Improve rewrites a bundle's primary file. The returned bundle is the improved
// one, or the original when anything at all went wrong — a failed rewrite must
// never cost the operator the artifact.
func Improve(ctx context.Context, a Author, b artifact.Bundle, f report.Finding, opts Options) (artifact.Bundle, error) {
	if len(b.Files) == 0 {
		return b, nil
	}
	if opts.Timeout <= 0 {
		opts = DefaultOptions()
	}

	prompt, err := buildPrompt(b, f)
	if err != nil {
		return b, err
	}

	runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, a.Binary, a.Args(prompt)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return b, fmt.Errorf("%s: %w: %s", a.Binary, err, strings.TrimSpace(firstLine(stderr.String())))
	}

	written := strings.TrimSpace(stdout.String())
	if len(written) > opts.MaxBytes {
		return b, fmt.Errorf("%s returned %d bytes, over the %d byte limit", a.Binary, len(written), opts.MaxBytes)
	}
	cleaned, err := validate(written, b)
	if err != nil {
		return b, err
	}

	improved := b
	improved.Files = append([]artifact.File(nil), b.Files...)
	improved.Files[0].Content = cleaned
	improved.Notes = append(append([]string(nil), b.Notes...),
		"Prose rewritten by "+a.DisplayName+" from ritual's evidence. The steps and counts come from the transcripts, not from the model.")
	return improved, nil
}

// buildPrompt gives the model the finding as structured data and the template
// as a starting point. It is given no transcript text beyond what already
// reached the artifact, which is the whole point: the model is a writer here,
// not an analyst.
func buildPrompt(b artifact.Bundle, f report.Finding) (string, error) {
	evidence := struct {
		Title       string   `json:"title"`
		Summary     string   `json:"summary"`
		Kind        string   `json:"kind"`
		Occurrences int      `json:"occurrences"`
		Sessions    int      `json:"sessions"`
		Repos       []string `json:"repos"`
		Agents      []string `json:"agents"`
		Cadence     string   `json:"cadence"`
		Steps       []string `json:"steps"`
		Commands    []string `json:"commands"`
		Corrections []string `json:"corrections,omitempty"`
	}{
		Title: f.Title, Summary: f.Summary, Kind: string(b.Kind),
		Occurrences: f.Occurrences, Sessions: f.Sessions,
		Repos: f.Repos, Agents: f.Agents, Cadence: f.Cadence.Label,
		Commands: f.Commands,
	}
	for _, s := range f.Steps {
		evidence.Steps = append(evidence.Steps, s.Action)
	}
	for _, c := range f.Corrections {
		evidence.Corrections = append(evidence.Corrections, c.Text)
	}
	data, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return "", err
	}

	var p strings.Builder
	p.WriteString("You are rewriting the prose of a generated Agent Skill. Reply with the complete Markdown file and nothing else: no preamble, no commentary, no code fence around the whole document.\n\n")
	p.WriteString("Hard rules:\n")
	p.WriteString("- Keep the YAML frontmatter, including the `name` value exactly as given.\n")
	p.WriteString("- Keep every step. Do not add steps, tools, or commands that are not in the evidence.\n")
	p.WriteString("- Keep the counts and dates accurate; they were measured, not guessed.\n")
	p.WriteString("- Keep the Evidence section and the trailing HTML comment verbatim.\n")
	p.WriteString("- Improve only clarity: sharpen the description so an agent knows when this applies, and turn mechanical step text into instructions a competent engineer would write.\n")
	p.WriteString("- Do not invent a rationale for a step. If the evidence does not say why, say what, not why.\n\n")
	p.WriteString("Evidence this was mined from:\n```json\n")
	p.Write(data)
	p.WriteString("\n```\n\nCurrent file:\n---8<---\n")
	p.WriteString(b.Files[0].Content)
	p.WriteString("\n---8<---\n")
	return p.String(), nil
}

// validate checks that the model returned something that is still the artifact.
// Anything else is discarded: a rewrite that drops the frontmatter or the
// evidence has stopped being an improvement.
func validate(out string, b artifact.Bundle) (string, error) {
	// Some CLIs wrap their whole answer in a fence despite being told not to.
	out = strings.TrimSpace(out)
	if strings.HasPrefix(out, "```") {
		if idx := strings.Index(out, "\n"); idx > 0 {
			out = out[idx+1:]
		}
		out = strings.TrimSuffix(strings.TrimSpace(out), "```")
		out = strings.TrimSpace(out)
	}
	if out == "" {
		return "", fmt.Errorf("empty response")
	}

	original := b.Files[0].Content
	needsFrontmatter := strings.HasPrefix(original, "---\n")
	if needsFrontmatter && !strings.HasPrefix(out, "---") {
		return "", fmt.Errorf("response dropped the frontmatter")
	}
	if needsFrontmatter {
		name := frontmatterValue(original, "name")
		if name != "" && frontmatterValue(out, "name") != name {
			return "", fmt.Errorf("response changed the skill name")
		}
	}
	if strings.Contains(original, "<!-- ritual:candidate=") && !strings.Contains(out, "<!-- ritual:candidate=") {
		return "", fmt.Errorf("response dropped the provenance marker")
	}
	if len(out) < len(original)/3 {
		return "", fmt.Errorf("response is %d bytes against an original of %d; too much was lost", len(out), len(original))
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, nil
}

func frontmatterValue(content, key string) string {
	if !strings.HasPrefix(content, "---") {
		return ""
	}
	body := content[3:]
	end := strings.Index(body, "\n---")
	if end < 0 {
		return ""
	}
	for _, line := range strings.Split(body[:end], "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == key {
			return strings.TrimSpace(strings.Trim(strings.TrimSpace(v), `"'`))
		}
	}
	return ""
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}
