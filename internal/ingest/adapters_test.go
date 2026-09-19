package ingest

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

func read(t *testing.T, fn reader, name string) session.Session {
	t.Helper()
	sessions, err := fn(filepath.Join("testdata", name), DefaultLimits(), redact.New())
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if len(sessions) != 1 {
		t.Fatalf("read %s: got %d sessions, want 1", name, len(sessions))
	}
	s := sessions[0]
	s.Finalize()
	return s
}

func TestReadClaude(t *testing.T) {
	s := read(t, readClaude, "claude.jsonl")

	if s.ID != "sess-1" || s.Title != "Storefront verification" {
		t.Fatalf("identity = %q / %q", s.ID, s.Title)
	}
	if s.Branch != "main" {
		t.Fatalf("branch = %q", s.Branch)
	}
	prompts := s.UserTurns()
	if len(prompts) != 1 {
		t.Fatalf("user turns = %d, want 1 (sdk and sidechain records must be dropped)", len(prompts))
	}
	if prompts[0].Text != "Verify the storefront after the theme push" {
		t.Fatalf("prompt = %q", prompts[0].Text)
	}

	calls := s.ToolCalls()
	if len(calls) != 2 {
		t.Fatalf("tool calls = %d, want 2", len(calls))
	}
	if calls[0].Tool != "shell" || calls[0].Command != "shopify theme push" {
		t.Fatalf("first call = %q / %q", calls[0].Tool, calls[0].Command)
	}

	errors := 0
	for _, turn := range s.Turns {
		if turn.Kind == session.KindToolResult && turn.IsError {
			errors++
		}
	}
	if errors != 1 {
		t.Fatalf("error results = %d, want 1", errors)
	}
}

func TestReadClaudeSurvivesMalformedLines(t *testing.T) {
	// The fixture ends with a line that is not JSON, which is what a crashed
	// agent leaves behind. It must cost that line and nothing else.
	s := read(t, readClaude, "claude.jsonl")
	if len(s.Turns) == 0 {
		t.Fatal("a malformed trailing line discarded the whole file")
	}
}

func TestReadCodex(t *testing.T) {
	s := read(t, readCodex, "codex.jsonl")

	if s.ID != "codex-1" {
		t.Fatalf("id = %q", s.ID)
	}
	if s.Title != "Fixed the failing test." {
		t.Fatalf("title = %q", s.Title)
	}
	prompts := s.UserTurns()
	if len(prompts) != 1 {
		t.Fatalf("user turns = %d, want 1 (the developer role must be skipped)", len(prompts))
	}
	calls := s.ToolCalls()
	if len(calls) != 1 || calls[0].Command != "npm run test:integration" {
		t.Fatalf("tool calls = %+v", calls)
	}
}

func TestReadGrokSkipsSystemAndReadsToolCalls(t *testing.T) {
	s := read(t, readGrok, "grok_chat_history.jsonl")

	if len(s.UserTurns()) != 1 {
		t.Fatalf("user turns = %d", len(s.UserTurns()))
	}
	calls := s.ToolCalls()
	if len(calls) != 1 {
		t.Fatalf("tool calls = %d", len(calls))
	}
	if calls[0].Command != "git status && git add" {
		t.Fatalf("command = %q", calls[0].Command)
	}
}

func TestReadGemini(t *testing.T) {
	s := read(t, readGemini, "gemini_session.json")

	if s.ID != "gem-1" {
		t.Fatalf("id = %q", s.ID)
	}
	if len(s.UserTurns()) != 1 {
		t.Fatalf("user turns = %d", len(s.UserTurns()))
	}
	calls := s.ToolCalls()
	if len(calls) != 1 || calls[0].Tool != "edit" {
		t.Fatalf("tool calls = %+v", calls)
	}
}

func TestFinalizeBackfillsTimestampsAndOrders(t *testing.T) {
	s := session.Session{Turns: []session.Turn{
		{Index: 2, Kind: session.KindMessage, Actor: session.ActorAssistant, Text: "third"},
		{Index: 0, Kind: session.KindMessage, Actor: session.ActorUser, Text: "first", At: mustTime("2026-01-01T00:00:00Z")},
		{Index: 1, Kind: session.KindMessage, Actor: session.ActorAssistant, Text: "second"},
	}}
	s.Finalize()

	if s.Turns[0].Text != "first" || s.Turns[2].Text != "third" {
		t.Fatalf("turns were not ordered: %+v", s.Turns)
	}
	for i, turn := range s.Turns {
		if turn.At.IsZero() {
			t.Fatalf("turn %d kept a zero timestamp", i)
		}
	}
	if s.Start.IsZero() || s.End.IsZero() {
		t.Fatal("session bounds were not derived")
	}
}

func TestSessionDurationRefusesToGuess(t *testing.T) {
	var s session.Session
	if d := s.Duration(); d != 0 {
		t.Fatalf("Duration = %v for an undated session, want 0", d)
	}
}

func mustTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return t
}
