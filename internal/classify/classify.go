// Package classify decides what a mined workflow should become.
//
// This is where ritual differs from a tool that turns every pattern into a
// skill. A recurring behaviour can belong in six different places, and putting
// it in the wrong one is worse than leaving it alone: a standing preference
// written as a skill never fires, and a nine-step release procedure written as
// a rules line is ignored as noise.
//
// The decision is rule-based and states its reasoning, because the operator is
// the one who has to live with the artifact.
package classify

import (
	"fmt"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/inventory"
	"github.com/HarjjotSinghh/ritual/internal/mine"
	"github.com/HarjjotSinghh/ritual/internal/score"
	"github.com/HarjjotSinghh/ritual/internal/textutil"
)

// Kind is the artifact a candidate should become.
type Kind string

const (
	// KindSkill is a multi-step procedure with judgement in it.
	KindSkill Kind = "skill"
	// KindCommand is a short, invoked-on-demand action.
	KindCommand Kind = "command"
	// KindRule is a standing instruction that belongs in AGENTS.md or
	// CLAUDE.md, where it applies without being invoked.
	KindRule Kind = "rule"
	// KindHook is work that should happen automatically after an event rather
	// than being remembered.
	KindHook Kind = "hook"
	// KindReference is stable project knowledge the agent keeps rediscovering.
	KindReference Kind = "reference"
	// KindUpdate means an installed artifact already covers this and has
	// drifted from what the operator actually does.
	KindUpdate Kind = "update"
	// KindIgnore means the pattern is real but not worth an artifact.
	KindIgnore Kind = "ignore"
)

// Decision is the classification plus why.
type Decision struct {
	Kind       Kind            `json:"kind"`
	Confidence float64         `json:"confidence"`
	Rationale  string          `json:"rationale"`
	Existing   *inventory.Item `json:"existing,omitempty"`
	// Similarity is how much of the candidate the existing artifact covers,
	// present only when Existing is set.
	Similarity float64 `json:"similarity,omitempty"`
}

// hookTriggers are the events that should fire work automatically rather than
// be remembered. A workflow that always begins right after one of these is a
// hook the operator is running by hand.
var hookTriggers = map[string]string{
	"shell:git commit":     "after a commit",
	"shell:git push":       "after a push",
	"shell:gh pr create":   "when a pull request is opened",
	"shell:gh pr merge":    "after a pull request merges",
	"shell:npm run build":  "after a build",
	"shell:shopify theme":  "after a theme push",
	"shell:vercel deploy":  "after a deploy",
	"shell:vercel":         "after a deploy",
	"shell:terraform appl": "after an apply",
}

// Classify decides what a scored candidate should become.
func Classify(c mine.Candidate, s score.Result, inv *inventory.Inventory, minScore float64) Decision {
	vocab := candidateVocabulary(c)

	// A run that invoked a skill is evidence about that skill. Proposing to
	// write it again would be the worst suggestion this tool could make, so
	// the reference is trusted over any vocabulary heuristic.
	if c.SkillRef != "" {
		d := Decision{
			Kind: KindUpdate, Confidence: c.SkillRefShare, Similarity: round2(c.SkillRefShare),
			Rationale: fmt.Sprintf(
				"%.0f%% of these runs invoked the %q skill you already have. What is useful here is the difference: the steps below are what actually happened, which is worth comparing against what the skill says to do.",
				c.SkillRefShare*100, c.SkillRef),
		}
		if inv != nil {
			for _, item := range inv.Items {
				if strings.EqualFold(item.Name, c.SkillRef) {
					d.Existing = &item
					break
				}
			}
		}
		return d
	}

	if inv != nil {
		if match, ok := inv.BestMatch(vocab, inventory.KindSkill, inventory.KindCommand); ok {
			return Decision{
				Kind:       KindUpdate,
				Confidence: match.Similarity,
				Existing:   &match.Item,
				Similarity: round2(match.Similarity),
				Rationale: fmt.Sprintf(
					"%s %q already covers %.0f%% of what these runs do. The runs still differ from it, so the better move is to update that artifact rather than add a second one.",
					match.Item.Kind, match.Item.Name, match.Similarity*100),
			}
		}
	}

	if s.Total < minScore {
		return Decision{
			Kind: KindIgnore, Confidence: 1 - s.Total/100,
			Rationale: "the pattern is real but too thin to be worth an artifact: " + strings.Join(s.Reasons, "; ") + ".",
		}
	}

	if trigger, ok := startsWithTrigger(c); ok {
		return Decision{
			Kind: KindHook, Confidence: 0.7,
			Rationale: fmt.Sprintf(
				"every run starts %s and then does the same %d things. That is an automation the operator is performing manually; a hook removes the remembering.",
				trigger, len(c.Steps)-1),
		}
	}

	if isReference(c) {
		return Decision{
			Kind: KindReference, Confidence: 0.6,
			Rationale: "the runs are almost entirely reading and searching the same places, which means the agent keeps rediscovering knowledge that should be written down once.",
		}
	}

	if len(c.Steps) <= 3 && c.Cadence.Regularity > 0.4 {
		return Decision{
			Kind: KindCommand, Confidence: 0.65,
			Rationale: fmt.Sprintf(
				"only %d steps, run %s with little variation. A slash command is the right weight; a skill would be ceremony around two actions.",
				len(c.Steps), c.Cadence.Label),
		}
	}

	return Decision{
		Kind: KindSkill, Confidence: clamp01(s.Total / 100),
		Rationale: fmt.Sprintf(
			"%d recurring steps over %d days with context restated each time. That is a procedure, and a skill is where a procedure belongs.",
			len(c.Steps), c.Cadence.Days),
	}
}

// ClassifyRule decides where a repeated standing preference belongs. Unlike
// workflows, these are never skills: a preference that has to be invoked is a
// preference that will be forgotten.
func ClassifyRule(r mine.RuleCandidate, inv *inventory.Inventory) Decision {
	if inv != nil {
		if match, ok := inv.BestMatch(r.Keywords, inventory.KindRules); ok {
			return Decision{
				Kind: KindIgnore, Confidence: match.Similarity, Existing: &match.Item,
				Similarity: round2(match.Similarity),
				Rationale: fmt.Sprintf(
					"%s already states this; the operator repeating it suggests the agent is not following the existing rule, which is a different problem from a missing one.",
					match.Item.Name),
			}
		}
	}
	return Decision{
		Kind: KindRule, Confidence: clamp01(float64(r.Occurrences) / 5),
		Rationale: fmt.Sprintf(
			"stated %d times across %d sessions and never written down, so it is re-taught on every run.",
			r.Occurrences, r.Sessions),
	}
}

// startsWithTrigger reports whether a workflow reliably follows a known event.
func startsWithTrigger(c mine.Candidate) (string, bool) {
	if len(c.Steps) < 3 {
		return "", false
	}
	first := c.Steps[0]
	// The trigger has to be near-universal in the cluster. A workflow that
	// sometimes follows a push is not a hook; it is a choice.
	if first.Support < 0.8 {
		return "", false
	}
	for prefix, phrase := range hookTriggers {
		if strings.HasPrefix(first.Action, prefix) {
			return phrase, true
		}
	}
	return "", false
}

// isReference reports whether a workflow is mostly rediscovery: reading and
// searching, with almost nothing changed.
func isReference(c mine.Candidate) bool {
	if len(c.Steps) < 2 {
		return false
	}
	readish, mutating := 0, 0
	for _, s := range c.Steps {
		switch s.Action {
		case "read", "search", "list", "web_fetch":
			readish++
		case "edit", "write", "browser":
			mutating++
		default:
			if strings.HasPrefix(s.Action, "shell:") {
				mutating++
			}
		}
	}
	return mutating == 0 && float64(readish)/float64(len(c.Steps)) >= 0.8
}

// candidateVocabulary is the text a candidate would be matched against an
// installed artifact by: its name, its shared phrases, and its commands.
func candidateVocabulary(c mine.Candidate) []string {
	tokens := textutil.Tokens(c.Title)
	for _, p := range c.Phrases {
		tokens = append(tokens, textutil.Tokens(p.Phrase)...)
	}
	tokens = append(tokens, c.Keywords...)
	for _, cmd := range c.Commands {
		tokens = append(tokens, textutil.Tokens(cmd)...)
	}
	return textutil.Dedupe(textutil.FilterMeaningful(tokens))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
