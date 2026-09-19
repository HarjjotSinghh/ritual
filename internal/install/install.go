// Package install writes generated bundles into the places agents read them.
//
// Installation is planned before it is performed. Every target resolves to an
// explicit list of writes, the plan can be printed without touching anything,
// and an existing file is never silently overwritten — these directories hold
// work the operator wrote by hand, and a tool that clobbered a skill because a
// slug collided would deserve to be uninstalled.
package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/artifact"
	"github.com/HarjjotSinghh/ritual/internal/classify"
)

// Target is one harness ritual can install into.
type Target struct {
	Key         string
	DisplayName string
	// SkillDir is where an Agent Skill directory goes.
	SkillDir string
	// CommandDir is where a slash command file goes. Empty means the harness
	// has no command concept and commands fall back to skills.
	CommandDir string
	// RulesFile is the file standing preferences are appended to.
	RulesFile string
}

// Targets returns every installable harness for this machine, whether or not
// the harness is installed: an operator may be setting one up.
func Targets() []Target {
	home, _ := os.UserHomeDir()
	j := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	return []Target{
		{"claude", "Claude Code", j(".claude", "skills"), j(".claude", "commands"), j(".claude", "CLAUDE.md")},
		{"codex", "Codex CLI", j(".codex", "skills"), j(".codex", "prompts"), j(".codex", "AGENTS.md")},
		{"cursor", "Cursor CLI", j(".cursor", "skills"), "", j("AGENTS.md")},
		{"opencode", "OpenCode", j(".config", "opencode", "skills"), j(".config", "opencode", "command"), j(".config", "opencode", "AGENTS.md")},
		{"gemini", "Gemini CLI", j(".gemini", "skills"), j(".gemini", "commands"), j(".gemini", "GEMINI.md")},
		{"grok", "Grok CLI", j(".grok", "skills"), "", j(".grok", "AGENTS.md")},
		{"qwen", "Qwen Code", j(".qwen", "skills"), "", j(".qwen", "AGENTS.md")},
		{"pi", "Pi", j(".pi", "agent", "skills"), "", j(".pi", "agent", "AGENTS.md")},
		{"shared", "Shared (~/.agents)", j(".agents", "skills"), "", j(".agents", "AGENTS.md")},
	}
}

// ProjectTarget builds a target rooted in a repository, for a skill that should
// travel with the code rather than with the operator.
func ProjectTarget(root string) Target {
	return Target{
		Key: "project", DisplayName: "This project",
		SkillDir:   filepath.Join(root, ".claude", "skills"),
		CommandDir: filepath.Join(root, ".claude", "commands"),
		RulesFile:  filepath.Join(root, "AGENTS.md"),
	}
}

// Lookup finds a target by key.
func Lookup(key string) (Target, bool) {
	for _, t := range Targets() {
		if t.Key == key {
			return t, true
		}
	}
	return Target{}, false
}

// Action is what will happen to one path.
type Action string

const (
	ActionCreate Action = "create"
	ActionAppend Action = "append"
	// ActionSkip means the file exists and ritual will not touch it without
	// --force.
	ActionSkip Action = "skip"
	// ActionOverwrite is a create over an existing file, only planned when the
	// caller asked for it.
	ActionOverwrite Action = "overwrite"
)

// Write is one planned filesystem change.
type Write struct {
	Path    string `json:"path"`
	Action  Action `json:"action"`
	Content string `json:"-"`
	Bytes   int    `json:"bytes"`
	Reason  string `json:"reason,omitempty"`
}

// Plan is everything one install would do.
type Plan struct {
	Target Target   `json:"-"`
	Kind   string   `json:"kind"`
	Name   string   `json:"name"`
	Writes []Write  `json:"writes"`
	Notes  []string `json:"notes,omitempty"`
}

// Options control planning.
type Options struct {
	// Force allows overwriting an existing file.
	Force bool
}

// PlanInstall resolves a bundle against a target.
func PlanInstall(b artifact.Bundle, t Target, opts Options) (Plan, error) {
	plan := Plan{Target: t, Kind: string(b.Kind), Name: b.Name, Notes: b.Notes}

	for _, f := range b.Files {
		dest, err := destinationFor(b, f, t)
		if err != nil {
			return plan, err
		}
		w := Write{Path: dest, Content: f.Content, Bytes: len(f.Content)}
		switch {
		case b.Append:
			w.Action = ActionAppend
		default:
			if _, statErr := os.Stat(dest); statErr == nil {
				if opts.Force {
					w.Action = ActionOverwrite
					w.Reason = "exists; --force given"
				} else {
					w.Action = ActionSkip
					w.Reason = "already exists; re-run with --force to replace it"
				}
			} else {
				w.Action = ActionCreate
			}
		}
		plan.Writes = append(plan.Writes, w)
	}
	sort.SliceStable(plan.Writes, func(i, j int) bool { return plan.Writes[i].Path < plan.Writes[j].Path })
	return plan, nil
}

func destinationFor(b artifact.Bundle, f artifact.File, t Target) (string, error) {
	switch b.Kind {
	case classify.KindUpdate:
		// An update is a comparison to read, not a file to add. Installing it
		// would put a second document next to the skill it is about, which is
		// the clutter the classification exists to prevent.
		return "", fmt.Errorf("%q is a drift report for an existing artifact, not something to install — run `ritual build` and read it", b.Name)
	case classify.KindRule:
		if t.RulesFile == "" {
			return "", fmt.Errorf("%s has no rules file to append to", t.DisplayName)
		}
		return t.RulesFile, nil
	case classify.KindCommand:
		if t.CommandDir != "" {
			return filepath.Join(t.CommandDir, f.Path), nil
		}
		// A harness with no command concept still understands skills, and a
		// command installed as a skill behaves correctly — it is just invoked
		// by description rather than by name.
		return filepath.Join(t.SkillDir, b.Slug, "SKILL.md"), nil
	case classify.KindReference:
		return filepath.Join(t.SkillDir, b.Slug, f.Path), nil
	default:
		if t.SkillDir == "" {
			return "", fmt.Errorf("%s has no skills directory", t.DisplayName)
		}
		return filepath.Join(t.SkillDir, b.Slug, f.Path), nil
	}
}

// Apply performs a plan. It returns the paths it changed.
func Apply(plan Plan) ([]string, error) {
	changed := make([]string, 0, len(plan.Writes))
	for _, w := range plan.Writes {
		switch w.Action {
		case ActionSkip:
			continue
		case ActionAppend:
			if err := appendTo(w.Path, w.Content); err != nil {
				return changed, err
			}
		case ActionCreate, ActionOverwrite:
			if err := os.MkdirAll(filepath.Dir(w.Path), 0o755); err != nil {
				return changed, err
			}
			if err := os.WriteFile(w.Path, []byte(w.Content), 0o644); err != nil {
				return changed, err
			}
		default:
			return changed, fmt.Errorf("unknown action %q for %s", w.Action, w.Path)
		}
		changed = append(changed, w.Path)
	}
	return changed, nil
}

// appendTo adds content to the end of a file, creating it when absent and
// inserting a blank line so an appended rule never runs into the previous one.
func appendTo(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Appending the same rule twice is the most likely repeat mistake, since
	// running a scan again will propose it again.
	if strings.Contains(string(existing), strings.TrimSpace(content)) {
		return nil
	}
	var buf strings.Builder
	if len(existing) > 0 {
		buf.Write(existing)
		if !strings.HasSuffix(string(existing), "\n") {
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}
	buf.WriteString(content)
	return os.WriteFile(path, []byte(buf.String()), 0o644)
}

// Describe renders a plan for the terminal without performing it.
func (p Plan) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s → %s\n", p.Name, p.Target.DisplayName)
	for _, w := range p.Writes {
		marker := map[Action]string{
			ActionCreate: "+", ActionAppend: "»", ActionSkip: "·", ActionOverwrite: "!",
		}[w.Action]
		fmt.Fprintf(&b, "  %s %s (%d bytes)", marker, w.Path, w.Bytes)
		if w.Reason != "" {
			fmt.Fprintf(&b, " — %s", w.Reason)
		}
		b.WriteString("\n")
	}
	for _, n := range p.Notes {
		fmt.Fprintf(&b, "  note: %s\n", n)
	}
	return b.String()
}
