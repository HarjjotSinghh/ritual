// Package mine turns normalized sessions into evidence-backed workflow
// candidates.
//
// The pipeline is deterministic end to end and runs entirely on the local
// machine: segment sessions into task arcs, extract the features of each arc,
// cluster arcs that are the same work done again, canonicalize the steps each
// cluster shares, and measure how the cluster behaves over time. No model is
// involved. A language model may later be asked to write the prose for a
// generated artifact, but it never decides what a workflow is, because a
// suggestion a user cannot audit is a suggestion they cannot trust.
package mine

import (
	"time"
)

// Arc is one continuous attempt at one task: a human prompt, the work that
// followed, and any corrections issued before the human moved on to something
// else.
type Arc struct {
	ID        string    `json:"id"`
	Agent     string    `json:"agent"`
	SessionID string    `json:"session_id"`
	Source    string    `json:"source"`
	Repo      string    `json:"repo,omitempty"`
	Workspace string    `json:"workspace,omitempty"`
	Branch    string    `json:"branch,omitempty"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`

	// Intent is the cleaned opening prompt: what the human asked for.
	Intent string `json:"intent"`
	// Prompts are every human turn in the arc, including the opening one.
	Prompts []string `json:"prompts,omitempty"`
	// Steps are the canonical actions taken, in order, with consecutive
	// repeats collapsed. A shell step carries its normalized command, so
	// "shell:git status" and "shell:npm run build" are different steps.
	Steps []string `json:"steps,omitempty"`
	// Tools is the distinct set of canonical tool verbs used.
	Tools []string `json:"tools,omitempty"`
	// Commands is the distinct set of normalized shell commands.
	Commands []string `json:"commands,omitempty"`
	// Paths are the directory and extension shapes the arc touched, never
	// full file paths.
	Paths []string `json:"paths,omitempty"`
	// Corrections are the human's mid-arc redirections.
	Corrections []Correction `json:"corrections,omitempty"`

	Turns  int `json:"turns"`
	Errors int `json:"errors"`
	// Retries counts a step repeated after an error, which is the clearest
	// signal that the human's instructions were incomplete.
	Retries int `json:"retries"`
}

// Duration is the arc's wall time.
func (a Arc) Duration() time.Duration {
	if a.Start.IsZero() || a.End.IsZero() || a.End.Before(a.Start) {
		return 0
	}
	return a.End.Sub(a.Start)
}

// CorrectionKind separates a one-off fix from a standing preference. The
// distinction decides the artifact: a standing preference belongs in a rules
// file where it applies to every future turn, while a one-off fix is at most a
// step inside a skill.
type CorrectionKind string

const (
	// CorrectionFix is a redirection scoped to the task at hand.
	CorrectionFix CorrectionKind = "fix"
	// CorrectionPreference is a standing instruction: always, never, from now
	// on, every time.
	CorrectionPreference CorrectionKind = "preference"
)

// Correction is a human turn that redirected the agent.
type Correction struct {
	Kind      CorrectionKind `json:"kind"`
	Text      string         `json:"text"`
	Trigger   string         `json:"trigger"`
	At        time.Time      `json:"at"`
	SessionID string         `json:"session_id"`
	Agent     string         `json:"agent"`
	Repo      string         `json:"repo,omitempty"`
}

// Step is one action in a cluster's canonical sequence, with the share of the
// cluster's arcs that performed it.
type Step struct {
	Action  string  `json:"action"`
	Support float64 `json:"support"`
	Count   int     `json:"count"`
	// Position is the median index of the step across the arcs that ran it,
	// which is what orders the canonical sequence.
	Position float64 `json:"position"`
}

// Cadence describes how a workflow recurs.
type Cadence struct {
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	// Days is the number of distinct calendar days the workflow was performed
	// on. It is the honest recurrence count: five runs in one afternoon is one
	// day of evidence, not five.
	Days int `json:"days"`
	// MedianIntervalHours is the median gap between consecutive days the
	// workflow ran. Zero when it ran on fewer than two days.
	MedianIntervalHours float64 `json:"median_interval_hours"`
	// Regularity is 0..1, higher when the intervals are consistent. A workflow
	// run every weekday scores near 1; a burst followed by silence scores low.
	Regularity float64 `json:"regularity"`
	// SpanDays is the number of days between first and last occurrence.
	SpanDays int `json:"span_days"`
	// Label is a human phrase: "about daily", "roughly weekly", "bursty".
	Label string `json:"label"`
}

// Evidence is one cited occurrence of a candidate workflow. Every suggestion
// carries these so the user can open the transcript and check the claim.
type Evidence struct {
	ArcID     string    `json:"arc_id"`
	Agent     string    `json:"agent"`
	SessionID string    `json:"session_id"`
	Source    string    `json:"source"`
	Repo      string    `json:"repo,omitempty"`
	At        time.Time `json:"at"`
	Intent    string    `json:"intent"`
	Steps     int       `json:"steps"`
}

// Candidate is one mined workflow, before scoring and classification.
type Candidate struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Slug  string `json:"slug"`
	// Summary is one sentence describing the work, built from the cluster's
	// shared vocabulary and steps.
	Summary string `json:"summary"`

	Occurrences int      `json:"occurrences"`
	Sessions    int      `json:"sessions"`
	Agents      []string `json:"agents"`
	Repos       []string `json:"repos"`
	Keywords    []string `json:"keywords"`
	// Phrases are the multi-word expressions the cluster's prompts share. They
	// are what the candidate is named from, because that is how people name
	// their own work.
	Phrases []ScoredPhrase `json:"phrases,omitempty"`

	Steps       []Step       `json:"steps"`
	Commands    []string     `json:"commands"`
	Tools       []string     `json:"tools"`
	Paths       []string     `json:"paths,omitempty"`
	Corrections []Correction `json:"corrections,omitempty"`

	Cadence  Cadence    `json:"cadence"`
	Evidence []Evidence `json:"evidence"`

	// Cohesion is the mean similarity of the cluster's arcs to its centroid,
	// 0..1. A low value means the cluster grouped loosely related work and the
	// suggestion should be treated with suspicion.
	Cohesion float64 `json:"cohesion"`
	// MedianTurns and MedianDurationMinutes describe the size of one run, which
	// is what "worth automating" ultimately turns on.
	MedianTurns           int     `json:"median_turns"`
	MedianDurationMinutes float64 `json:"median_duration_minutes"`
	ErrorRate             float64 `json:"error_rate"`
}
