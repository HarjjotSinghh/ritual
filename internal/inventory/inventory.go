// Package inventory finds the skills, commands, and rules files already
// installed on this machine.
//
// Without it, ritual's most likely failure mode is proposing a skill for a
// workflow the operator automated months ago — which is both useless and
// slightly insulting. Knowing what exists turns that case into the more useful
// suggestion: this workflow already has a skill, and here is where it drifted
// from what you actually do.
package inventory

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/textutil"
)

// Kind separates the artifact types an operator can already have.
type Kind string

const (
	KindSkill   Kind = "skill"
	KindCommand Kind = "command"
	KindRules   Kind = "rules"
)

// Item is one installed artifact.
type Item struct {
	Kind        Kind     `json:"kind"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Path        string   `json:"path"`
	Harness     string   `json:"harness"`
	Scope       string   `json:"scope"`
	Tokens      []string `json:"-"`
}

// Inventory is everything found.
type Inventory struct {
	Items []Item `json:"items"`
	Roots []string
}

// skillRoots are the per-harness locations that hold Agent Skills. The format
// is shared — a directory with a SKILL.md — even though every harness put it
// somewhere different.
func skillRoots(home string) []struct {
	path    string
	harness string
} {
	j := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	return []struct {
		path    string
		harness string
	}{
		{j(".claude", "skills"), "claude"},
		{j(".config", "claude", "skills"), "claude"},
		{j(".agents", "skills"), "shared"},
		{j(".codex", "skills"), "codex"},
		{j(".cursor", "skills"), "cursor"},
		{j(".cursor", "skills-cursor"), "cursor"},
		{j(".gemini", "skills"), "gemini"},
		{j(".qwen", "skills"), "qwen"},
		{j(".grok", "skills"), "grok"},
		{j(".pi", "agent", "skills"), "pi"},
		{j(".config", "opencode", "skills"), "opencode"},
	}
}

func commandRoots(home string) []struct {
	path    string
	harness string
} {
	j := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	return []struct {
		path    string
		harness string
	}{
		{j(".claude", "commands"), "claude"},
		{j(".codex", "prompts"), "codex"},
		{j(".config", "opencode", "command"), "opencode"},
		{j(".gemini", "commands"), "gemini"},
	}
}

func rulesFiles(home string) []string {
	j := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	return []string{
		j(".claude", "CLAUDE.md"),
		j("CLAUDE.md"),
		j(".codex", "AGENTS.md"),
		j("AGENTS.md"),
		j(".agents", "AGENTS.md"),
		j(".gemini", "GEMINI.md"),
		j(".config", "opencode", "AGENTS.md"),
	}
}

// Scan walks the user-level locations plus any project roots supplied by the
// caller. Project roots come from the mined sessions, so a repository the
// operator no longer works in is never touched.
func Scan(projectRoots []string) (*Inventory, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	inv := &Inventory{}

	for _, root := range skillRoots(home) {
		inv.addSkills(root.path, root.harness, "user")
	}
	for _, root := range commandRoots(home) {
		inv.addCommands(root.path, root.harness, "user")
	}
	for _, path := range rulesFiles(home) {
		inv.addRules(path, "user")
	}

	for _, project := range textutil.Dedupe(projectRoots) {
		if project == "" {
			continue
		}
		for _, sub := range []struct{ rel, harness string }{
			{filepath.Join(".claude", "skills"), "claude"},
			{filepath.Join(".agents", "skills"), "shared"},
			{filepath.Join(".cursor", "skills"), "cursor"},
			{"skills", "shared"},
		} {
			inv.addSkills(filepath.Join(project, sub.rel), sub.harness, "project")
		}
		inv.addCommands(filepath.Join(project, ".claude", "commands"), "claude", "project")
		for _, name := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md", ".cursorrules"} {
			inv.addRules(filepath.Join(project, name), "project")
		}
	}

	sort.SliceStable(inv.Items, func(i, j int) bool {
		if inv.Items[i].Kind != inv.Items[j].Kind {
			return inv.Items[i].Kind < inv.Items[j].Kind
		}
		return inv.Items[i].Name < inv.Items[j].Name
	})
	return inv, nil
}

func (inv *Inventory) addSkills(root, harness, scope string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	inv.Roots = append(inv.Roots, redact.Path(root))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(root, e.Name(), "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			// A plugin bundle nests one more level: <root>/<plugin>/skills/<name>.
			inv.addSkills(filepath.Join(root, e.Name(), "skills"), harness, scope)
			continue
		}
		name, description := parseFrontmatter(string(data))
		if name == "" {
			name = e.Name()
		}
		inv.Items = append(inv.Items, Item{
			Kind: KindSkill, Name: name, Description: description,
			Path: redact.Path(path), Harness: harness, Scope: scope,
			Tokens: textutil.Dedupe(append(textutil.Tokens(name), textutil.Tokens(description)...)),
		})
	}
}

func (inv *Inventory) addCommands(root, harness, scope string) {
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		name, description := parseFrontmatter(string(data))
		if name == "" {
			name = strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		}
		if description == "" {
			description = firstProse(string(data))
		}
		inv.Items = append(inv.Items, Item{
			Kind: KindCommand, Name: name, Description: description,
			Path: redact.Path(path), Harness: harness, Scope: scope,
			Tokens: textutil.Dedupe(append(textutil.Tokens(name), textutil.Tokens(description)...)),
		})
		return nil
	})
	if err == nil {
		inv.Roots = append(inv.Roots, redact.Path(root))
	}
}

func (inv *Inventory) addRules(path, scope string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	inv.Items = append(inv.Items, Item{
		Kind: KindRules, Name: filepath.Base(path),
		Description: firstProse(string(data)),
		Path:        redact.Path(path), Harness: harnessForRules(path), Scope: scope,
		Tokens: textutil.Dedupe(textutil.Tokens(string(data))),
	})
}

func harnessForRules(path string) string {
	switch strings.ToLower(filepath.Base(path)) {
	case "claude.md":
		return "claude"
	case "gemini.md":
		return "gemini"
	case ".cursorrules":
		return "cursor"
	default:
		return "shared"
	}
}

// parseFrontmatter reads the name and description out of a YAML frontmatter
// block. It is intentionally a line reader rather than a YAML parser: skill
// frontmatter is two or three scalar fields, and pulling in a YAML dependency
// to read them would be the larger mistake.
func parseFrontmatter(content string) (name, description string) {
	sc := bufio.NewScanner(strings.NewReader(content))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return "", ""
	}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(strings.Trim(strings.TrimSpace(value), `"'`))
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "name":
			name = value
		case "description":
			description = value
		}
	}
	return name, description
}

// firstProse returns the first non-heading, non-frontmatter paragraph, which is
// what a rules file or command uses in place of a description.
func firstProse(content string) string {
	sc := bufio.NewScanner(strings.NewReader(content))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	inFrontmatter := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if inFrontmatter || line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "<!--") {
			continue
		}
		return textutil.Truncate(line, 200)
	}
	return ""
}

// Match is an overlap between a mined candidate and something installed.
type Match struct {
	Item       Item    `json:"item"`
	Similarity float64 `json:"similarity"`
}

// BestMatch returns the installed artifact that best covers the given
// vocabulary, or ok=false when nothing is close enough.
//
// The threshold is a judgement call with asymmetric costs. Too low and ritual
// tells an operator they already have a skill they do not, hiding a real
// suggestion. Too high and it proposes a duplicate. Between those, proposing a
// duplicate is the recoverable mistake, so the bar is set high.
func (inv *Inventory) BestMatch(tokens []string, kinds ...Kind) (Match, bool) {
	want := textutil.NewSet(tokens...)
	if len(want) == 0 {
		return Match{}, false
	}
	allowed := make(map[Kind]struct{}, len(kinds))
	for _, k := range kinds {
		allowed[k] = struct{}{}
	}

	best := Match{}
	for _, item := range inv.Items {
		if len(allowed) > 0 {
			if _, ok := allowed[item.Kind]; !ok {
				continue
			}
		}
		have := textutil.NewSet(item.Tokens...)
		if len(have) == 0 {
			continue
		}
		// Containment rather than Jaccard: a rules file with ten thousand
		// tokens would score near zero against any candidate under Jaccard,
		// while still covering it completely.
		overlap := 0
		for t := range want {
			if _, ok := have[t]; ok {
				overlap++
			}
		}
		sim := float64(overlap) / float64(len(want))
		if sim > best.Similarity {
			best = Match{Item: item, Similarity: sim}
		}
	}
	if best.Similarity < 0.55 {
		return Match{}, false
	}
	return best, true
}
