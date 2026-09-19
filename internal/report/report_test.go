package report

import (
	"testing"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/classify"
	"github.com/HarjjotSinghh/ritual/internal/ingest"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

// buildHistory fabricates a history where one workflow is repeated across days
// and one is done once, which is the minimum needed to check that the pipeline
// separates a habit from a task.
func buildHistory() []session.Session {
	var out []session.Session
	for day := 1; day <= 6; day++ {
		s := session.Session{ID: "verify-" + string(rune('a'+day)), Agent: "claude", Repo: "storefront"}
		start := time.Date(2026, 9, day, 9, 0, 0, 0, time.UTC)
		s.Append(session.Turn{Actor: session.ActorUser, Kind: session.KindMessage,
			Text: "verify the storefront theme after pushing it live and check the cart drawer", At: start})
		for i, step := range []struct{ tool, cmd string }{
			{"shell", "shopify theme push"}, {"browser", ""}, {"shell", "npm run test"}, {"edit", ""},
		} {
			s.Append(session.Turn{Actor: session.ActorAssistant, Kind: session.KindToolCall,
				Tool: step.tool, Command: step.cmd, At: start.Add(time.Duration(i+1) * time.Minute)})
		}
		s.Finalize()
		out = append(out, s)
	}

	once := session.Session{ID: "one-off", Agent: "claude", Repo: "storefront"}
	start := time.Date(2026, 9, 3, 15, 0, 0, 0, time.UTC)
	once.Append(session.Turn{Actor: session.ActorUser, Kind: session.KindMessage,
		Text: "rename the analytics event for the newsletter signup", At: start})
	once.Append(session.Turn{Actor: session.ActorAssistant, Kind: session.KindToolCall, Tool: "edit", At: start.Add(time.Minute)})
	once.Finalize()
	return append(out, once)
}

func TestPipelineFindsTheRepeatedWorkflow(t *testing.T) {
	opts := DefaultOptions()
	opts.SkipInventory = true

	rep, err := BuildFrom(&ingest.Result{Sessions: buildHistory()}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Findings) == 0 {
		t.Fatal("the pipeline found nothing in a history with six identical runs")
	}

	top := rep.Findings[0]
	if top.Occurrences < 6 {
		t.Fatalf("top finding has %d runs, want 6", top.Occurrences)
	}
	if top.Cadence.Days != 6 {
		t.Fatalf("cadence days = %d, want 6", top.Cadence.Days)
	}
	if top.Decision.Kind == classify.KindIgnore {
		t.Fatalf("a daily four-step workflow was ignored: %s", top.Decision.Rationale)
	}
	if len(top.Evidence) == 0 {
		t.Fatal("a finding with no evidence cannot be checked")
	}
}

func TestPipelineIsDeterministic(t *testing.T) {
	opts := DefaultOptions()
	opts.SkipInventory = true
	history := buildHistory()

	first, err := BuildFrom(&ingest.Result{Sessions: history}, opts)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildFrom(&ingest.Result{Sessions: history}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Findings) != len(second.Findings) {
		t.Fatalf("finding counts differ: %d vs %d", len(first.Findings), len(second.Findings))
	}
	for i := range first.Findings {
		a, b := first.Findings[i], second.Findings[i]
		if a.ID != b.ID || a.Score.Total != b.Score.Total {
			t.Fatalf("finding %d differs between runs: %q/%.1f vs %q/%.1f", i, a.ID, a.Score.Total, b.ID, b.Score.Total)
		}
	}
}

func TestFindAcceptsPrefixes(t *testing.T) {
	opts := DefaultOptions()
	opts.SkipInventory = true
	rep, err := BuildFrom(&ingest.Result{Sessions: buildHistory()}, opts)
	if err != nil {
		t.Fatal(err)
	}
	id := rep.Findings[0].ID
	if _, ok := rep.Find(id[:5]); !ok {
		t.Fatalf("Find did not accept the prefix %q", id[:5])
	}
	if _, ok := rep.Find("zz"); ok {
		t.Fatal("Find matched a prefix that is too short to be meaningful")
	}
}

func TestActionableExcludesIgnored(t *testing.T) {
	rep := &Report{Findings: []Finding{
		{Decision: classify.Decision{Kind: classify.KindSkill}},
		{Decision: classify.Decision{Kind: classify.KindIgnore}},
	}}
	if len(rep.Actionable()) != 1 {
		t.Fatalf("Actionable = %d, want 1", len(rep.Actionable()))
	}
}
