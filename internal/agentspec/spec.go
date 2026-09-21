// Package agentspec is the catalog of coding agents ritual can read, and the
// filesystem discovery that turns a catalog entry into a list of session files.
//
// Storage layouts here were verified against a real machine and cross-checked
// against the agent-storage research in reinstate (github.com/HarjjotSinghh/
// reinstate), which maintains per-agent storage documentation. When a vendor
// moves its store, this file is the only place that has to change.
package agentspec

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Layout names the on-disk shape of an agent's session store. Readers switch on
// it, so adding an agent that reuses an existing layout costs one entry here.
type Layout string

const (
	// LayoutClaudeJSONL is ~/.claude/projects/<path-slug>/<uuid>.jsonl.
	LayoutClaudeJSONL Layout = "claude-jsonl"
	// LayoutCodexRollout is ~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl.
	LayoutCodexRollout Layout = "codex-rollout"
	// LayoutCursorTranscript is
	// ~/.cursor/projects/<slug>/agent-transcripts/<id>/<id>.jsonl.
	LayoutCursorTranscript Layout = "cursor-transcript"
	// LayoutOpenCodeSQLite is the embedded opencode.db session store.
	LayoutOpenCodeSQLite Layout = "opencode-sqlite"
	// LayoutGeminiChat is ~/.gemini/tmp/<project>/chats/session-*.json.
	LayoutGeminiChat Layout = "gemini-chat"
	// LayoutGrokSession is ~/.grok/sessions/<url-encoded-cwd>/<id>/.
	LayoutGrokSession Layout = "grok-session"
	// LayoutQwenJSONL is ~/.qwen/projects/<slug>/chats/*.jsonl.
	LayoutQwenJSONL Layout = "qwen-jsonl"
	// LayoutKimiState is ~/.kimi-code/sessions/<workspace>/<id>/state.json.
	LayoutKimiState Layout = "kimi-state"
	// LayoutCopilotEvents is ~/.copilot/session-state/<id>/events.jsonl.
	LayoutCopilotEvents Layout = "copilot-events"
	// LayoutClineTask is ~/.cline/tasks/<id>/api_conversation_history.json.
	LayoutClineTask Layout = "cline-task"
	// LayoutPiJSONL is ~/.pi/agent/sessions/<slug>/*.jsonl.
	LayoutPiJSONL Layout = "pi-jsonl"
	// LayoutAiderMarkdown is a per-repository .aider.chat.history.md.
	LayoutAiderMarkdown Layout = "aider-markdown"
)

// Spec describes one agent's session store.
type Spec struct {
	// Key is the stable identifier used in config, flags, and reports.
	Key string
	// DisplayName is what a human sees.
	DisplayName string
	// Vendor is the publisher, shown by `ritual agents`.
	Vendor string
	// RootEnv, when set, names an environment variable that relocates the
	// whole store. RootEnvSuffix is appended to it when the variable names a
	// parent directory rather than the store itself.
	RootEnv       string
	RootEnvSuffix string
	// Roots are the default locations, most likely first.
	Roots []string
	// Marker is a child of the root that must exist for the root to count as
	// this agent's store. It keeps an empty ~/.gemini from claiming sessions.
	Marker string
	// Glob is the session file pattern relative to the root, using ** for any
	// number of path segments.
	Glob string
	// Layout selects the reader.
	Layout Layout
	// Excluded are root-relative directory or file names never walked. These
	// are credential files and multi-hundred-megabyte install trees, not
	// sessions.
	Excluded []string
	// WholeStore marks a layout where one file holds every session rather
	// than one session per file. The scan's per-file size limit does not
	// apply to it: an OpenCode database is hundreds of megabytes because it
	// is the entire history, not because one session is pathological.
	WholeStore bool
	// Note explains anything surprising about this entry.
	Note string
}

// Catalog is every agent ritual knows how to read, in a stable order.
func Catalog() []Spec {
	home, _ := os.UserHomeDir()
	j := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }

	return []Spec{
		{
			Key: "claude", DisplayName: "Claude Code", Vendor: "Anthropic",
			RootEnv: "CLAUDE_CONFIG_DIR",
			Roots:   []string{j(".claude"), j(".config", "claude")},
			Marker:  "projects", Glob: "projects/**/*.jsonl", Layout: LayoutClaudeJSONL,
			Excluded: []string{".credentials.json", "statsig", "shell-snapshots", "todos", "file-history", "plugins", "ide"},
		},
		{
			Key: "codex", DisplayName: "Codex CLI", Vendor: "OpenAI",
			RootEnv: "CODEX_HOME",
			Roots:   []string{j(".codex"), j(".config", "codex")},
			Marker:  "sessions", Glob: "sessions/**/*.jsonl", Layout: LayoutCodexRollout,
			Excluded: []string{"auth.json", "log", "cache", "archived_sessions"},
		},
		{
			Key: "cursor", DisplayName: "Cursor CLI", Vendor: "Anysphere",
			RootEnv: "CURSOR_CONFIG_DIR",
			Roots:   []string{j(".cursor")},
			Marker:  "projects", Glob: "projects/**/agent-transcripts/**/*.jsonl", Layout: LayoutCursorTranscript,
			Excluded: []string{"extensions", "plugins", "browser-logs", "node_modules", "ai-tracking", "canvases", "terminals", "assets"},
			Note:     "The CLI writes transcripts under ~/.cursor/projects; the IDE keeps its own SQLite store that ritual does not read.",
		},
		{
			Key: "opencode", DisplayName: "OpenCode", Vendor: "SST",
			RootEnv: "XDG_DATA_HOME", RootEnvSuffix: "opencode",
			Roots:  []string{j(".local", "share", "opencode")},
			Marker: "opencode.db", Glob: "opencode.db", Layout: LayoutOpenCodeSQLite, WholeStore: true,
			Excluded: []string{"auth.json", "mcp-auth.json", "snapshot", "repos", "log", "tool-output"},
		},
		{
			Key: "gemini", DisplayName: "Gemini CLI", Vendor: "Google",
			RootEnv: "GEMINI_CLI_HOME",
			Roots:   []string{j(".gemini")},
			Marker:  "tmp", Glob: "tmp/**/chats/*.json", Layout: LayoutGeminiChat,
			Excluded: []string{"antigravity", "antigravity-browser-profile", "antigravity-cli", "oauth_creds.json", "google_accounts.json", "skills", "history", "logs"},
		},
		{
			Key: "grok", DisplayName: "Grok CLI", Vendor: "xAI",
			RootEnv: "GROK_HOME",
			Roots:   []string{j(".grok")},
			Marker:  "sessions", Glob: "sessions/**/chat_history.jsonl", Layout: LayoutGrokSession,
			Excluded: []string{"bundled", "marketplace-cache", "bin", "downloads", "docs", "skills", "auth.json", "mcp_credentials.json", "grove", "memtrace", "completions"},
		},
		{
			Key: "qwen", DisplayName: "Qwen Code", Vendor: "Alibaba",
			RootEnv: "QWEN_HOME",
			Roots:   []string{j(".qwen")},
			Marker:  "projects", Glob: "projects/**/chats/*.jsonl", Layout: LayoutQwenJSONL,
			Excluded: []string{"oauth_creds.json", "skills", "tmp"},
		},
		{
			Key: "kimi", DisplayName: "Kimi Code", Vendor: "Moonshot AI",
			RootEnv: "KIMI_CODE_HOME",
			Roots:   []string{j(".kimi-code"), j(".kimi")},
			Marker:  "sessions", Glob: "sessions/**/state.json", Layout: LayoutKimiState,
			Excluded: []string{"auth.json", "cache"},
		},
		{
			Key: "copilot", DisplayName: "GitHub Copilot CLI", Vendor: "GitHub",
			RootEnv: "COPILOT_HOME",
			Roots:   []string{j(".copilot")},
			Marker:  "session-state", Glob: "session-state/**/events.jsonl", Layout: LayoutCopilotEvents,
			Excluded: []string{"logs", "cache"},
		},
		{
			Key: "cline", DisplayName: "Cline CLI", Vendor: "Cline",
			RootEnv: "CLINE_DATA_DIR",
			Roots:   []string{j(".cline", "data"), j(".cline")},
			Marker:  "sessions", Glob: "sessions/**/*.messages.json", Layout: LayoutClineTask,
			Excluded: []string{"settings", "locks", "cache", "logs", "cron", "db"},
			Note:     "The CLI keeps sessions at ~/.cline/data/sessions/<id>/<id>.messages.json; the tasks/ layout in older builds is gone.",
		},
		{
			Key: "pi", DisplayName: "Pi", Vendor: "Pi Labs",
			RootEnv: "PI_CODING_AGENT_DIR",
			Roots:   []string{j(".pi", "agent")},
			Marker:  "sessions", Glob: "sessions/**/*.jsonl", Layout: LayoutPiJSONL,
			Excluded: []string{"extensions", "skills", "auth.json"},
		},
	}
}

// Lookup returns the spec for a key.
func Lookup(key string) (Spec, bool) {
	for _, s := range Catalog() {
		if s.Key == key {
			return s, true
		}
	}
	return Spec{}, false
}

// Keys returns every catalog key in order.
func Keys() []string {
	specs := Catalog()
	out := make([]string, 0, len(specs))
	for _, s := range specs {
		out = append(out, s.Key)
	}
	return out
}

// ResolvedRoots returns every location on this machine that holds this agent's
// sessions, most authoritative first.
//
// It returns all of them rather than the first, which is a deliberate change
// from honouring the environment override alone. A relocated store is where the
// agent writes *now*; the default path often still holds months of history from
// before the variable was set, and a mining tool that ignored it would silently
// drop the larger half of the record. Both are read-only, so reading both costs
// nothing and the file walk de-duplicates.
func (s Spec) ResolvedRoots() []string {
	candidates := make([]string, 0, len(s.Roots)+1)
	if s.RootEnv != "" {
		if v := strings.TrimSpace(os.Getenv(s.RootEnv)); v != "" {
			if s.RootEnvSuffix != "" {
				v = filepath.Join(v, s.RootEnvSuffix)
			}
			candidates = append(candidates, v)
		}
	}
	candidates = append(candidates, s.Roots...)

	seen := make(map[string]struct{}, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if c == "" {
			continue
		}
		c = filepath.Clean(c)
		if _, dup := seen[c]; dup {
			continue
		}
		if s.Marker != "" {
			if _, err := os.Stat(filepath.Join(c, s.Marker)); err != nil {
				continue
			}
		} else if _, err := os.Stat(c); err != nil {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

// Root returns the most authoritative store location, or ok=false when the
// agent is not installed here.
func (s Spec) Root() (root string, ok bool) {
	roots := s.ResolvedRoots()
	if len(roots) == 0 {
		return "", false
	}
	return roots[0], true
}

// Discovery is the result of walking one agent's store.
type Discovery struct {
	Spec Spec
	// Root is the most authoritative location; Roots is every location walked.
	Root  string
	Roots []string
	Files []string
	// Skipped counts entries the walk refused: excluded trees and files that
	// could not be read. It is reported rather than hidden so a surprising
	// session count has an explanation.
	Skipped int
}

// Discover finds every session file for a spec. A missing store is not an
// error: it means the agent is not installed, and ok is false.
func Discover(s Spec) (Discovery, bool, error) {
	roots := s.ResolvedRoots()
	if len(roots) == 0 {
		return Discovery{Spec: s}, false, nil
	}
	d := Discovery{Spec: s, Root: roots[0], Roots: roots}

	excluded := make(map[string]struct{}, len(s.Excluded))
	for _, e := range s.Excluded {
		excluded[e] = struct{}{}
	}

	seen := make(map[string]struct{}, 256)
	for _, root := range roots {
		if err := s.walkRoot(root, excluded, seen, &d); err != nil {
			return d, true, err
		}
	}
	sort.Strings(d.Files)
	return d, true, nil
}

// walkRoot collects the session files under one root.
func (s Spec) walkRoot(root string, excluded map[string]struct{}, seen map[string]struct{}, d *Discovery) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			d.Skipped++
			// A permission error on one subtree must not abort the walk: a
			// single unreadable directory should cost that directory, not the
			// agent.
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		if rel == "." {
			return nil
		}
		name := entry.Name()
		if _, bad := excluded[name]; bad {
			d.Skipped++
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if strings.HasPrefix(name, ".") && name != "." {
				return fs.SkipDir
			}
			return nil
		}
		if matchGlob(s.Glob, filepath.ToSlash(rel)) {
			if _, dup := seen[path]; !dup {
				seen[path] = struct{}{}
				d.Files = append(d.Files, path)
			}
		}
		return nil
	})
}

// DiscoverAll walks every catalog entry, or only the given keys when any are
// supplied. Agents that are not installed are simply absent from the result.
func DiscoverAll(keys []string) ([]Discovery, error) {
	want := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		want[strings.ToLower(strings.TrimSpace(k))] = struct{}{}
	}
	out := make([]Discovery, 0, len(Catalog()))
	for _, s := range Catalog() {
		if len(want) > 0 {
			if _, ok := want[s.Key]; !ok {
				continue
			}
		}
		d, installed, err := Discover(s)
		if err != nil {
			return out, err
		}
		if !installed {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// matchGlob matches a slash-separated relative path against a pattern that may
// contain ** (any number of segments), * (any run within a segment), and ?.
func matchGlob(pattern, path string) bool {
	if pattern == "" {
		return false
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(path, "/"))
}

func matchSegments(pat, seg []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			// ** may consume zero or more segments; try every split.
			rest := pat[1:]
			if len(rest) == 0 {
				return true
			}
			for i := 0; i <= len(seg); i++ {
				if matchSegments(rest, seg[i:]) {
					return true
				}
			}
			return false
		}
		if len(seg) == 0 {
			return false
		}
		ok, err := filepath.Match(pat[0], seg[0])
		if err != nil || !ok {
			return false
		}
		pat, seg = pat[1:], seg[1:]
	}
	return len(seg) == 0
}
