// Package artifact renders a mined finding into the files an agent can
// actually use.
//
// The output follows the open Agent Skills layout — a directory containing
// SKILL.md with YAML frontmatter — because every major harness now reads it and
// a portable artifact is worth more than a clever one. Rules, commands, and
// hooks each have their own shape, and the classifier decides which is
// produced.
//
// Every generated file carries its evidence. A skill whose provenance is
// invisible is a skill nobody can audit six months later when it starts firing
// at the wrong time.
package artifact

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/classify"
	"github.com/HarjjotSinghh/ritual/internal/mine"
	"github.com/HarjjotSinghh/ritual/internal/report"
	"github.com/HarjjotSinghh/ritual/internal/textutil"
)

// File is one generated file, with a path relative to the bundle root.
type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Bundle is everything one finding produces.
type Bundle struct {
	Kind classify.Kind `json:"kind"`
	Name string        `json:"name"`
	Slug string        `json:"slug"`
	// Description is the frontmatter description: when the agent should reach
	// for this.
	Description string `json:"description"`
	Files       []File `json:"files"`
	// Notes are things the operator should know before installing, such as a
	// step ritual could not name confidently.
	Notes []string `json:"notes,omitempty"`
	// Append, when set, means the content is meant to be added to an existing
	// file rather than written as a new one.
	Append bool `json:"append,omitempty"`
}

// Build renders a finding into its bundle, dispatching on the classification.
func Build(f report.Finding) Bundle {
	switch f.Decision.Kind {
	case classify.KindCommand:
		return buildCommand(f)
	case classify.KindHook:
		return buildHook(f)
	case classify.KindReference:
		return buildReference(f)
	default:
		return buildSkill(f)
	}
}

func buildSkill(f report.Finding) Bundle {
	slug := skillSlug(f.Slug)
	b := Bundle{
		Kind: classify.KindSkill, Name: f.Title, Slug: slug,
		Description: describe(f),
	}
	var sb strings.Builder
	writeFrontmatter(&sb, slug, b.Description)
	fmt.Fprintf(&sb, "# %s\n\n%s\n\n", f.Title, f.Summary)

	writeWhenToUse(&sb, f)
	writeSteps(&sb, f)
	writePreferences(&sb, f)
	writeGuardrails(&sb, f)
	writeEvidence(&sb, f)

	b.Files = append(b.Files, File{Path: "SKILL.md", Content: sb.String()})
	b.Notes = notesFor(f)
	return b
}

func buildCommand(f report.Finding) Bundle {
	slug := skillSlug(f.Slug)
	b := Bundle{
		Kind: classify.KindCommand, Name: f.Title, Slug: slug,
		Description: describe(f),
	}
	var sb strings.Builder
	writeFrontmatter(&sb, slug, b.Description)
	fmt.Fprintf(&sb, "%s\n\n", f.Summary)
	writeSteps(&sb, f)
	writeEvidence(&sb, f)

	b.Files = append(b.Files, File{Path: slug + ".md", Content: sb.String()})
	b.Notes = append(notesFor(f), "Commands are invoked explicitly. If this should happen without being asked, install it as a hook instead.")
	return b
}

func buildHook(f report.Finding) Bundle {
	slug := skillSlug(f.Slug)
	b := Bundle{
		Kind: classify.KindHook, Name: f.Title, Slug: slug,
		Description: describe(f),
	}
	var sb strings.Builder
	writeFrontmatter(&sb, slug, b.Description)
	fmt.Fprintf(&sb, "# %s\n\n%s\n\n", f.Title, f.Summary)
	fmt.Fprintf(&sb, "## Trigger\n\n%s\n\n", f.Decision.Rationale)
	writeSteps(&sb, f)
	writeGuardrails(&sb, f)
	writeEvidence(&sb, f)

	b.Files = append(b.Files, File{Path: "SKILL.md", Content: sb.String()})
	b.Notes = append(notesFor(f),
		"ritual writes the procedure, not the wiring. Hook configuration differs per harness, and a hook that fires on the wrong event is worse than no hook, so connect it deliberately.")
	return b
}

func buildReference(f report.Finding) Bundle {
	slug := skillSlug(f.Slug)
	b := Bundle{
		Kind: classify.KindReference, Name: f.Title, Slug: slug,
		Description: describe(f),
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n%s\n\n", f.Title, f.Summary)
	sb.WriteString("These runs were almost entirely reading and searching: the agent kept rediscovering the same ground. Writing down what it found turns that into a lookup.\n\n")

	if len(f.Paths) > 0 {
		sb.WriteString("## Where it looked\n\n")
		for _, p := range f.Paths {
			fmt.Fprintf(&sb, "- `%s`\n", p)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("## What to write here\n\n")
	sb.WriteString("Replace this section with the answer the agent kept deriving — the schema, the naming convention, the deployment order, whatever these runs were reconstructing. Keep it short enough to stay true.\n\n")
	writeEvidence(&sb, f)

	b.Files = append(b.Files, File{Path: slug + ".md", Content: sb.String()})
	b.Notes = append(notesFor(f), "This one is a stub on purpose: ritual can see what the agent looked for, not what it concluded.")
	return b
}

// BuildRule renders a standing preference as a block to append to a rules file.
func BuildRule(r report.RuleFinding) Bundle {
	b := Bundle{
		Kind:   classify.KindRule,
		Name:   textutil.Truncate(r.Text, 60),
		Slug:   textutil.Slug(strings.Join(r.Keywords[:min(3, len(r.Keywords))], "-")),
		Append: true,
	}
	var sb strings.Builder
	sb.WriteString("<!-- added by ritual -->\n")
	fmt.Fprintf(&sb, "- %s\n", ruleImperative(r))
	fmt.Fprintf(&sb, "  <!-- stated %d times across %d sessions", r.Occurrences, r.Sessions)
	if len(r.Repos) > 0 {
		fmt.Fprintf(&sb, " in %s", strings.Join(r.Repos, ", "))
	}
	sb.WriteString(" -->\n")

	b.Description = fmt.Sprintf("Standing preference repeated %d times", r.Occurrences)
	b.Files = append(b.Files, File{Path: "AGENTS.md", Content: sb.String()})
	b.Notes = []string{
		"This is appended to a rules file, not installed as a skill: a preference that has to be invoked is a preference that gets forgotten.",
		"Read it before appending. ritual copies the operator's own phrasing, which is sometimes scoped to the task they said it in.",
	}
	return b
}

// ruleImperative turns a mid-conversation correction into a standing
// instruction. The operator's own words are kept — they know their domain, and
// a paraphrase would quietly change the rule — but conversational lead-ins are
// trimmed so the line reads as an instruction.
func ruleImperative(r report.RuleFinding) string {
	text := strings.TrimSpace(r.Text)
	for _, prefix := range []string{
		"no, ", "no ", "actually, ", "actually ", "also, ", "also ", "and ", "but ",
		"btw ", "by the way, ", "remember, ", "just ", "please ",
	} {
		if strings.HasPrefix(strings.ToLower(text), prefix) {
			text = text[len(prefix):]
			break
		}
	}
	if text == "" {
		return r.Text
	}
	text = strings.ToUpper(text[:1]) + text[1:]
	if !strings.HasSuffix(text, ".") && !strings.HasSuffix(text, "!") {
		text += "."
	}
	return textutil.Truncate(text, 400)
}

func writeFrontmatter(sb *strings.Builder, slug, description string) {
	sb.WriteString("---\n")
	fmt.Fprintf(sb, "name: %s\n", slug)
	fmt.Fprintf(sb, "description: %s\n", yamlScalar(description))
	sb.WriteString("---\n\n")
}

// describe writes the frontmatter description, which is the only thing an agent
// reads when deciding whether a skill applies. It is phrased as a trigger
// condition rather than a summary for exactly that reason.
func describe(f report.Finding) string {
	var b strings.Builder
	b.WriteString("Use when ")
	switch {
	case len(f.Phrases) > 0 && f.Phrases[0].Score >= mine.StrongPhrase:
		b.WriteString("the task involves " + f.Phrases[0].Phrase)
	case len(f.Repos) == 1:
		b.WriteString("working in " + f.Repos[0] + " on " + strings.Join(topKeywords(f, 3), ", "))
	default:
		b.WriteString("the task involves " + strings.Join(topKeywords(f, 3), ", "))
	}
	if len(f.Commands) > 0 {
		b.WriteString(", or when running " + backtickList(f.Commands, 2))
	}
	b.WriteString(". ")
	fmt.Fprintf(&b, "Mined from %d runs across %d sessions.", f.Occurrences, f.Sessions)
	return textutil.Truncate(strings.Join(strings.Fields(b.String()), " "), 400)
}

func writeWhenToUse(sb *strings.Builder, f report.Finding) {
	sb.WriteString("## When to use this\n\n")
	if len(f.Repos) > 0 {
		fmt.Fprintf(sb, "- Working in %s.\n", strings.Join(f.Repos, ", "))
	}
	for _, p := range f.Phrases {
		if p.Score < mine.StrongPhrase {
			continue
		}
		fmt.Fprintf(sb, "- The request mentions %s.\n", p.Phrase)
	}
	if len(f.Commands) > 0 {
		fmt.Fprintf(sb, "- The work involves %s.\n", backtickList(f.Commands, 4))
	}
	fmt.Fprintf(sb, "- Historically run %s.\n\n", f.Cadence.Label)
}

func writeSteps(sb *strings.Builder, f report.Finding) {
	if len(f.Steps) == 0 {
		return
	}
	sb.WriteString("## Steps\n\n")
	for i, s := range f.Steps {
		line := mine.DescribeStep(s.Action)
		// Support below certainty is stated rather than smoothed over: a step
		// two runs in three performed is a real part of the workflow and also
		// a place where the operator exercised judgement.
		if s.Support < 0.9 {
			fmt.Fprintf(sb, "%d. %s. *(%d of %d runs)*\n", i+1, line, s.Count, f.Occurrences)
		} else {
			fmt.Fprintf(sb, "%d. %s.\n", i+1, line)
		}
	}
	sb.WriteString("\n")
}

func writePreferences(sb *strings.Builder, f report.Finding) {
	if len(f.Corrections) == 0 {
		return
	}
	sb.WriteString("## Things the operator had to correct\n\n")
	sb.WriteString("These came up during the runs this skill was mined from. They are the parts that were not obvious.\n\n")
	seen := map[string]struct{}{}
	for _, c := range f.Corrections {
		key := strings.ToLower(textutil.Truncate(c.Text, 80))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		fmt.Fprintf(sb, "- %s\n", textutil.Truncate(strings.Join(strings.Fields(c.Text), " "), 240))
	}
	sb.WriteString("\n")
}

func writeGuardrails(sb *strings.Builder, f report.Finding) {
	if f.ErrorRate < 0.3 {
		return
	}
	sb.WriteString("## Where this goes wrong\n\n")
	fmt.Fprintf(sb, "%.0f%% of the mined runs hit an error or a retry. Check the failing step before assuming the procedure is complete.\n\n", f.ErrorRate*100)
}

func writeEvidence(sb *strings.Builder, f report.Finding) {
	sb.WriteString("## Evidence\n\n")
	fmt.Fprintf(sb, "Mined by ritual from %d runs across %d sessions", f.Occurrences, f.Sessions)
	if len(f.Agents) > 0 {
		fmt.Fprintf(sb, " (%s)", strings.Join(f.Agents, ", "))
	}
	if !f.Cadence.FirstSeen.IsZero() {
		fmt.Fprintf(sb, ", %s to %s",
			f.Cadence.FirstSeen.Format("2006-01-02"), f.Cadence.LastSeen.Format("2006-01-02"))
	}
	fmt.Fprintf(sb, ". Score %.0f/100.\n\n", f.Score.Total)

	if len(f.Evidence) > 0 {
		sb.WriteString("<details>\n<summary>Sessions this came from</summary>\n\n")
		for _, e := range f.Evidence {
			when := "undated"
			if !e.At.IsZero() {
				when = e.At.Format("2006-01-02")
			}
			fmt.Fprintf(sb, "- `%s` %s — %s\n", e.Agent, when, textutil.Truncate(e.Intent, 110))
		}
		sb.WriteString("\n</details>\n\n")
	}
	fmt.Fprintf(sb, "<!-- ritual:candidate=%s generated=%s -->\n", f.ID, time.Now().UTC().Format("2006-01-02"))
}

func notesFor(f report.Finding) []string {
	notes := make([]string, 0, 3)
	if f.Cohesion < 0.5 {
		notes = append(notes, fmt.Sprintf(
			"The grouped runs agree only %.0f%% with each other. Read the evidence before installing; this may be two workflows wearing one name.", f.Cohesion*100))
	}
	if f.Decision.Kind == classify.KindUpdate && f.Decision.Existing != nil {
		notes = append(notes, "An existing artifact already covers most of this: "+f.Decision.Existing.Path)
	}
	notes = append(notes, "ritual wrote the structure from what happened. The judgement — why a step exists, when to skip it — is still the operator's to add.")
	return notes
}

func topKeywords(f report.Finding, n int) []string {
	if len(f.Keywords) <= n {
		return f.Keywords
	}
	return f.Keywords[:n]
}

func backtickList(values []string, n int) string {
	if len(values) > n {
		values = values[:n]
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, "`"+v+"`")
	}
	return strings.Join(out, ", ")
}

// skillSlug ensures the directory name is a legal skill identifier: lowercase,
// hyphenated, and short enough that harnesses which surface it in a menu do not
// truncate it into ambiguity.
func skillSlug(slug string) string {
	slug = textutil.Slug(slug)
	parts := strings.Split(slug, "-")
	if len(parts) > 5 {
		parts = parts[:5]
	}
	out := strings.Join(parts, "-")
	if out == "" {
		return "mined-workflow"
	}
	return out
}

// yamlScalar quotes a description safely for frontmatter without pulling in a
// YAML encoder for one field.
func yamlScalar(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if strings.ContainsAny(s, `:#"'{}[]&*!|>%@`+"`") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SortBundles orders bundles deterministically for display.
func SortBundles(bundles []Bundle) {
	sort.SliceStable(bundles, func(i, j int) bool {
		if bundles[i].Kind != bundles[j].Kind {
			return bundles[i].Kind < bundles[j].Kind
		}
		return bundles[i].Slug < bundles[j].Slug
	})
}
