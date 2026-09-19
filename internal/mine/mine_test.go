package mine

import (
	"testing"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/session"
)

func at(day, hour int) time.Time {
	return time.Date(2026, 9, day, hour, 0, 0, 0, time.UTC)
}

// buildSession creates a session with one prompt followed by a tool sequence.
func buildSession(id string, start time.Time, prompt string, steps ...string) session.Session {
	s := session.Session{ID: id, Agent: "claude", Repo: "storefront", Workspace: "/tmp/storefront"}
	s.Append(session.Turn{Actor: session.ActorUser, Kind: session.KindMessage, Text: prompt, At: start})
	for i, step := range steps {
		turn := session.Turn{
			Actor: session.ActorAssistant, Kind: session.KindToolCall,
			At: start.Add(time.Duration(i+1) * time.Minute),
		}
		if len(step) > 6 && step[:6] == "shell:" {
			turn.Tool, turn.Command = "shell", step[6:]
		} else {
			turn.Tool = step
		}
		s.Append(turn)
	}
	s.Finalize()
	return s
}

func TestSegmentSplitsOnNewPrompt(t *testing.T) {
	s := buildSession("s1", at(1, 9), "first task", "read", "edit")
	s.Append(
		session.Turn{Actor: session.ActorUser, Kind: session.KindMessage, Text: "now do a completely different second task", At: at(1, 10)},
		session.Turn{Actor: session.ActorAssistant, Kind: session.KindToolCall, Tool: "search", At: at(1, 10).Add(time.Minute)},
		session.Turn{Actor: session.ActorAssistant, Kind: session.KindToolCall, Tool: "write", At: at(1, 10).Add(2 * time.Minute)},
	)
	s.Finalize()

	arcs := Segment([]session.Session{s}, DefaultSegmentOptions())
	if len(arcs) != 2 {
		t.Fatalf("arcs = %d, want 2", len(arcs))
	}
	if arcs[0].Intent != "first task" {
		t.Fatalf("first intent = %q", arcs[0].Intent)
	}
}

func TestSegmentFoldsContinuations(t *testing.T) {
	s := buildSession("s1", at(1, 9), "verify the storefront", "read")
	s.Append(
		session.Turn{Actor: session.ActorUser, Kind: session.KindMessage, Text: "yes, go ahead", At: at(1, 9).Add(2 * time.Minute)},
		session.Turn{Actor: session.ActorAssistant, Kind: session.KindToolCall, Tool: "edit", At: at(1, 9).Add(3 * time.Minute)},
	)
	s.Finalize()

	arcs := Segment([]session.Session{s}, DefaultSegmentOptions())
	if len(arcs) != 1 {
		t.Fatalf("arcs = %d, want 1: an acknowledgement is not a new task", len(arcs))
	}
}

func TestSegmentSplitsOnIdleGap(t *testing.T) {
	s := buildSession("s1", at(1, 9), "verify the storefront", "read", "search")
	s.Append(
		session.Turn{Actor: session.ActorAssistant, Kind: session.KindToolCall, Tool: "edit", At: at(2, 9)},
		session.Turn{Actor: session.ActorAssistant, Kind: session.KindToolCall, Tool: "write", At: at(2, 9).Add(time.Minute)},
	)
	s.Finalize()

	arcs := Segment([]session.Session{s}, DefaultSegmentOptions())
	if len(arcs) != 2 {
		t.Fatalf("arcs = %d, want 2: a session resumed the next day is two pieces of work", len(arcs))
	}
}

func TestSegmentCountsRetriesAfterErrors(t *testing.T) {
	s := session.Session{ID: "s1", Agent: "claude"}
	s.Append(
		session.Turn{Actor: session.ActorUser, Kind: session.KindMessage, Text: "run the build", At: at(1, 9)},
		session.Turn{Actor: session.ActorAssistant, Kind: session.KindToolCall, Tool: "shell", Command: "npm run build", At: at(1, 9).Add(time.Minute)},
		session.Turn{Actor: session.ActorTool, Kind: session.KindToolResult, IsError: true, At: at(1, 9).Add(2 * time.Minute)},
		session.Turn{Actor: session.ActorAssistant, Kind: session.KindToolCall, Tool: "shell", Command: "npm run build", At: at(1, 9).Add(3 * time.Minute)},
	)
	s.Finalize()

	arcs := Segment([]session.Session{s}, SegmentOptions{IdleSplit: time.Hour, MinSteps: 1, MaxPrompts: 4})
	if len(arcs) != 1 {
		t.Fatalf("arcs = %d", len(arcs))
	}
	if arcs[0].Errors != 1 || arcs[0].Retries != 1 {
		t.Fatalf("errors = %d, retries = %d, want 1 and 1", arcs[0].Errors, arcs[0].Retries)
	}
}

func TestClusterGroupsRepeatedWork(t *testing.T) {
	var sessions []session.Session
	for i := 0; i < 4; i++ {
		sessions = append(sessions, buildSession(
			"verify"+string(rune('a'+i)), at(i+1, 9),
			"verify the storefront theme after pushing to production",
			"read", "browser", "shell:npm run test"))
	}
	for i := 0; i < 3; i++ {
		sessions = append(sessions, buildSession(
			"invoice"+string(rune('a'+i)), at(i+1, 15),
			"reconcile the monthly invoices from the billing export",
			"shell:python reconcile", "write"))
	}

	arcs := Segment(sessions, DefaultSegmentOptions())
	opts := DefaultClusterOptions()
	opts.MinOccurrences = 3
	clusters := ClusterArcs(arcs, opts)

	if len(clusters) != 2 {
		t.Fatalf("clusters = %d, want 2", len(clusters))
	}
	if len(clusters[0].Arcs) != 4 {
		t.Fatalf("largest cluster has %d arcs, want 4", len(clusters[0].Arcs))
	}
}

func TestClusteringIsDeterministic(t *testing.T) {
	var sessions []session.Session
	for i := 0; i < 6; i++ {
		sessions = append(sessions, buildSession(
			"s"+string(rune('a'+i)), at(i+1, 9),
			"verify the storefront theme after pushing to production",
			"read", "browser"))
	}
	arcs := Segment(sessions, DefaultSegmentOptions())

	first := Run(sessionsOf(arcs), DefaultOptions())
	second := Run(sessionsOf(arcs), DefaultOptions())
	if len(first.Candidates) != len(second.Candidates) {
		t.Fatalf("candidate counts differ: %d vs %d", len(first.Candidates), len(second.Candidates))
	}
	for i := range first.Candidates {
		if first.Candidates[i].ID != second.Candidates[i].ID {
			t.Fatalf("candidate %d differs between runs: %q vs %q", i, first.Candidates[i].ID, second.Candidates[i].ID)
		}
	}
}

// sessionsOf is a helper for the determinism test: it rebuilds sessions from
// arcs so Run can be called twice on identical input.
func sessionsOf(arcs []Arc) []session.Session {
	out := make([]session.Session, 0, len(arcs))
	for _, a := range arcs {
		s := session.Session{ID: a.SessionID, Agent: a.Agent, Repo: a.Repo}
		s.Append(session.Turn{Actor: session.ActorUser, Kind: session.KindMessage, Text: a.Intent, At: a.Start})
		for i, step := range a.Steps {
			turn := session.Turn{Actor: session.ActorAssistant, Kind: session.KindToolCall, At: a.Start.Add(time.Duration(i+1) * time.Minute)}
			if len(step) > 6 && step[:6] == "shell:" {
				turn.Tool, turn.Command = "shell", step[6:]
			} else {
				turn.Tool = step
			}
			s.Append(turn)
		}
		s.Finalize()
		out = append(out, s)
	}
	return out
}

func TestCanonicalStepsOrdersByPosition(t *testing.T) {
	arcs := []Arc{
		{Steps: []string{"read", "edit", "shell:npm run build"}},
		{Steps: []string{"read", "edit", "shell:npm run build"}},
		{Steps: []string{"read", "shell:npm run build"}},
	}
	steps := CanonicalSteps(arcs, 0.5)
	if len(steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(steps))
	}
	if steps[0].Action != "read" {
		t.Fatalf("first step = %q, want read", steps[0].Action)
	}
	if steps[0].Support != 1 {
		t.Fatalf("read support = %v, want 1", steps[0].Support)
	}
}

func TestCanonicalStepsDropsRareActions(t *testing.T) {
	arcs := []Arc{
		{Steps: []string{"read", "edit"}},
		{Steps: []string{"read", "edit"}},
		{Steps: []string{"read", "edit", "shell:git stash"}},
	}
	for _, s := range CanonicalSteps(arcs, 0.5) {
		if s.Action == "shell:git stash" {
			t.Fatal("a one-off action became part of the canonical sequence")
		}
	}
}

func TestMeasureCadenceCountsDaysNotRuns(t *testing.T) {
	arcs := []Arc{
		{Start: at(1, 9)}, {Start: at(1, 10)}, {Start: at(1, 11)},
		{Start: at(2, 9)},
	}
	c := MeasureCadence(arcs)
	if c.Days != 2 {
		t.Fatalf("days = %d, want 2: three runs in one afternoon is one day of evidence", c.Days)
	}
}

func TestMeasureCadenceLabelsRegularWork(t *testing.T) {
	var arcs []Arc
	for day := 1; day <= 6; day++ {
		arcs = append(arcs, Arc{Start: at(day, 9)})
	}
	c := MeasureCadence(arcs)
	if c.Label != "about daily" {
		t.Fatalf("label = %q, want about daily", c.Label)
	}
	if c.Regularity < 0.9 {
		t.Fatalf("regularity = %v, want near 1 for evenly spaced runs", c.Regularity)
	}
}

func TestGroupCorrectionsMergesRestatements(t *testing.T) {
	arcs := []Arc{
		{Corrections: []Correction{{Kind: CorrectionPreference, Text: "always test the mobile layout too", SessionID: "a", Agent: "claude"}}},
		{Corrections: []Correction{{Kind: CorrectionPreference, Text: "never forget the mobile layout check", SessionID: "b", Agent: "claude"}}},
		{Corrections: []Correction{{Kind: CorrectionFix, Text: "no, revert that", SessionID: "c", Agent: "claude"}}},
	}
	rules := GroupCorrections(arcs, 2)
	if len(rules) != 1 {
		t.Fatalf("rules = %d, want 1: restatements of one preference are one rule", len(rules))
	}
	if rules[0].Occurrences != 2 {
		t.Fatalf("occurrences = %d, want 2", rules[0].Occurrences)
	}
}

func TestDescribeStepRendersImperatives(t *testing.T) {
	cases := map[string]string{
		"shell:npm run build": "Run `npm run build`",
		"read":                "Read the relevant files",
		"mcp:slack:send":      "Call the slack integration (send)",
	}
	for action, want := range cases {
		if got := DescribeStep(action); got != want {
			t.Errorf("DescribeStep(%q) = %q, want %q", action, got, want)
		}
	}
}
