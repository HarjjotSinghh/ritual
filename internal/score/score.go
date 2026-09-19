// Package score ranks mined workflow candidates.
//
// The scoring model exists because raw frequency is actively misleading. `git
// status` runs a hundred times a week and deserves no skill; a nine-step
// release verification run every Friday deserves one. What separates them is
// not how often they happen but how much undocumented judgement they carry:
// how many steps, how much context has to be restated, how often the operator
// had to correct the agent, how consistently it recurs.
//
// Every component is reported alongside the total, and every penalty explains
// itself, because a ranked list whose ordering cannot be interrogated is a
// ranked list nobody should act on.
package score

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/mine"
)

// Weights control the blend. They are exported so the eval harness can sweep
// them and so an operator can disagree in config rather than in a fork.
type Weights struct {
	Recurrence  float64 `json:"recurrence" toml:"recurrence"`
	Regularity  float64 `json:"regularity" toml:"regularity"`
	Complexity  float64 `json:"complexity" toml:"complexity"`
	Repetition  float64 `json:"repetition" toml:"repetition"`
	Friction    float64 `json:"friction" toml:"friction"`
	Breadth     float64 `json:"breadth" toml:"breadth"`
	TimeSpent   float64 `json:"time_spent" toml:"time_spent"`
	Cohesion    float64 `json:"cohesion" toml:"cohesion"`
	Corrections float64 `json:"corrections" toml:"corrections"`
}

// DefaultWeights were set by running the eval harness against histories where
// the operator had already written skills by hand, and tuning until the mined
// ranking put those workflows near the top.
func DefaultWeights() Weights {
	return Weights{
		Recurrence:  0.20,
		Regularity:  0.10,
		Complexity:  0.18,
		Repetition:  0.12,
		Friction:    0.10,
		Breadth:     0.08,
		TimeSpent:   0.08,
		Cohesion:    0.09,
		Corrections: 0.05,
	}
}

// Components are the individual 0..1 signals behind a total.
type Components struct {
	Recurrence  float64 `json:"recurrence"`
	Regularity  float64 `json:"regularity"`
	Complexity  float64 `json:"complexity"`
	Repetition  float64 `json:"repetition"`
	Friction    float64 `json:"friction"`
	Breadth     float64 `json:"breadth"`
	TimeSpent   float64 `json:"time_spent"`
	Cohesion    float64 `json:"cohesion"`
	Corrections float64 `json:"corrections"`
}

// Penalty is a named deduction with its reason.
type Penalty struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Reason string  `json:"reason"`
}

// Result is a scored candidate.
type Result struct {
	Total      float64    `json:"total"`
	Raw        float64    `json:"raw"`
	Components Components `json:"components"`
	Penalties  []Penalty  `json:"penalties,omitempty"`
	Reasons    []string   `json:"reasons"`
}

// trivialActions are steps that carry no judgement on their own. A workflow
// made only of these is somebody looking around, not a procedure.
var trivialActions = map[string]struct{}{
	"read": {}, "list": {}, "search": {},
	"shell:ls": {}, "shell:cat": {}, "shell:pwd": {}, "shell:cd": {},
	"shell:git status": {}, "shell:git diff": {}, "shell:git log": {},
	"shell:echo": {}, "shell:head": {}, "shell:tail": {}, "shell:wc": {},
}

// Score evaluates one candidate.
func Score(c mine.Candidate, w Weights) Result {
	comp := Components{
		Recurrence:  recurrence(c),
		Regularity:  clamp(c.Cadence.Regularity),
		Complexity:  complexity(c),
		Repetition:  repetition(c),
		Friction:    friction(c),
		Breadth:     breadth(c),
		TimeSpent:   timeSpent(c),
		Cohesion:    clamp(c.Cohesion),
		Corrections: corrections(c),
	}

	raw := comp.Recurrence*w.Recurrence +
		comp.Regularity*w.Regularity +
		comp.Complexity*w.Complexity +
		comp.Repetition*w.Repetition +
		comp.Friction*w.Friction +
		comp.Breadth*w.Breadth +
		comp.TimeSpent*w.TimeSpent +
		comp.Cohesion*w.Cohesion +
		comp.Corrections*w.Corrections

	res := Result{Raw: round1(raw * 100), Components: comp}
	res.Penalties = penalties(c)
	total := raw
	for _, p := range res.Penalties {
		total *= 1 - p.Amount
	}
	res.Total = round1(clamp(total) * 100)
	res.Reasons = reasons(c, comp)
	return res
}

// recurrence counts distinct days, not runs. Twelve runs in one afternoon while
// fighting a flaky deploy is one day of evidence.
func recurrence(c mine.Candidate) float64 {
	days := float64(c.Cadence.Days)
	if days <= 1 {
		return 0.05
	}
	// Saturating growth: the difference between two days and six is large, the
	// difference between twenty and forty is not.
	return clamp(math.Log(days) / math.Log(12))
}

// complexity rewards a procedure with real structure. A one-step "workflow" is
// a command; a nine-step one is where the operator's undocumented judgement
// lives.
func complexity(c mine.Candidate) float64 {
	steps := float64(len(c.Steps))
	turns := float64(c.MedianTurns)
	stepScore := clamp(steps / 8)
	turnScore := clamp(turns / 25)
	distinct := 0
	for _, s := range c.Steps {
		if _, trivial := trivialActions[s.Action]; !trivial {
			distinct++
		}
	}
	substance := clamp(float64(distinct) / 5)
	return clamp(0.4*stepScore + 0.25*turnScore + 0.35*substance)
}

// repetition measures how much of the same context the operator restates each
// time. Shared phrasing across runs is the literal evidence that they are
// re-typing instructions a skill would hold for them.
func repetition(c mine.Candidate) float64 {
	if len(c.Phrases) == 0 {
		return 0.1
	}
	best := c.Phrases[0].Score
	return clamp(best / 5)
}

// friction counts what went wrong. Errors and retries mean the agent needed
// information the prompt did not carry.
func friction(c mine.Candidate) float64 {
	return clamp(c.ErrorRate*0.7 + math.Min(float64(len(c.Corrections))/4, 1)*0.3)
}

// breadth rewards a workflow that survives a change of repository or agent:
// that is the clearest evidence it is a practice rather than a project detail.
func breadth(c mine.Candidate) float64 {
	repos := clamp(float64(len(c.Repos)-1) / 3)
	agents := clamp(float64(len(c.Agents)-1) / 2)
	return clamp(0.6*repos + 0.4*agents)
}

// timeSpent is total minutes across every run, saturating at four hours. It is
// the crudest estimate of what automation would return.
func timeSpent(c mine.Candidate) float64 {
	total := c.MedianDurationMinutes * float64(c.Occurrences)
	if total <= 0 {
		return 0.15
	}
	return clamp(math.Log1p(total) / math.Log1p(240))
}

func corrections(c mine.Candidate) float64 {
	if len(c.Corrections) == 0 {
		return 0
	}
	standing := 0
	for _, corr := range c.Corrections {
		if corr.Kind == mine.CorrectionPreference {
			standing++
		}
	}
	return clamp(float64(standing)/3*0.7 + float64(len(c.Corrections))/6*0.3)
}

func penalties(c mine.Candidate) []Penalty {
	out := make([]Penalty, 0, 3)

	trivial := 0
	for _, s := range c.Steps {
		if _, ok := trivialActions[s.Action]; ok {
			trivial++
		}
	}
	if len(c.Steps) > 0 && float64(trivial)/float64(len(c.Steps)) > 0.8 {
		out = append(out, Penalty{
			Name: "generic", Amount: 0.5,
			Reason: "every step is a look-around action (read, list, git status) with nothing decided",
		})
	}
	if len(c.Steps) <= 1 {
		out = append(out, Penalty{
			Name: "single-step", Amount: 0.35,
			Reason: "one action is a command or an alias, not a procedure worth documenting",
		})
	}
	if c.Cohesion < 0.35 {
		out = append(out, Penalty{
			Name: "loose-cluster", Amount: 0.3,
			Reason: fmt.Sprintf("the grouped runs only agree %.0f%% with each other, so this may be several workflows", c.Cohesion*100),
		})
	}
	if c.Cadence.Days <= 1 {
		out = append(out, Penalty{
			Name: "single-day", Amount: 0.45,
			Reason: "every run happened on one day, which is a task, not a habit",
		})
	}
	if c.Sessions < 3 && c.Occurrences >= 5 {
		out = append(out, Penalty{
			Name: "in-session-loop", Amount: 0.25,
			Reason: "the repeats are mostly inside the same session, which usually means retrying rather than repeating",
		})
	}
	return out
}

func reasons(c mine.Candidate, comp Components) []string {
	type note struct {
		weight float64
		text   string
	}
	notes := []note{
		{comp.Recurrence, fmt.Sprintf("ran on %d separate days (%s)", c.Cadence.Days, c.Cadence.Label)},
		{comp.Complexity, fmt.Sprintf("%d recurring steps, typically %d turns per run", len(c.Steps), c.MedianTurns)},
		{comp.Breadth, fmt.Sprintf("spans %d repositories and %d agents", len(c.Repos), len(c.Agents))},
		{comp.Friction, fmt.Sprintf("%.0f%% of runs hit an error or needed correcting", c.ErrorRate*100)},
		{comp.Repetition, "the prompts repeat the same context each time"},
		{comp.TimeSpent, fmt.Sprintf("about %.0f minutes per run", c.MedianDurationMinutes)},
		{comp.Corrections, fmt.Sprintf("%d standing corrections were issued during these runs", len(c.Corrections))},
	}
	sort.SliceStable(notes, func(i, j int) bool { return notes[i].weight > notes[j].weight })

	out := make([]string, 0, 4)
	for _, n := range notes {
		if n.weight < 0.2 {
			continue
		}
		out = append(out, n.text)
		if len(out) == 4 {
			break
		}
	}
	if len(out) == 0 {
		out = append(out, "weak signal across every dimension; shown for completeness")
	}
	return out
}

// Describe renders a component breakdown as an aligned block for the terminal.
func (r Result) Describe() string {
	var b strings.Builder
	rows := []struct {
		name  string
		value float64
	}{
		{"recurrence", r.Components.Recurrence},
		{"regularity", r.Components.Regularity},
		{"complexity", r.Components.Complexity},
		{"repetition", r.Components.Repetition},
		{"friction", r.Components.Friction},
		{"breadth", r.Components.Breadth},
		{"time spent", r.Components.TimeSpent},
		{"cohesion", r.Components.Cohesion},
		{"corrections", r.Components.Corrections},
	}
	for _, row := range rows {
		filled := int(row.value*20 + 0.5)
		fmt.Fprintf(&b, "  %-12s %s %.2f\n", row.name, strings.Repeat("█", filled)+strings.Repeat("·", 20-filled), row.value)
	}
	return b.String()
}

func clamp(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
