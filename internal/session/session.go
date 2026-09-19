// Package session defines the normalized shape every agent transcript is read
// into. Adapters in internal/ingest differ wildly — JSONL rollouts, SQLite
// stores, per-session JSON directories — but everything downstream of this
// package sees only Session and Turn, so the miner never learns a vendor's
// schema.
package session

import (
	"sort"
	"strings"
	"time"
)

// Actor is who produced a turn.
type Actor string

const (
	ActorUser      Actor = "user"
	ActorAssistant Actor = "assistant"
	ActorTool      Actor = "tool"
	ActorSystem    Actor = "system"
	ActorUnknown   Actor = "unknown"
)

// Kind is what a turn carries.
type Kind string

const (
	KindMessage    Kind = "message"
	KindToolCall   Kind = "tool_call"
	KindToolResult Kind = "tool_result"
	KindReasoning  Kind = "reasoning"
	KindSummary    Kind = "summary"
	KindMetadata   Kind = "metadata"
)

// Turn is one normalized event inside a session.
//
// Text is always redacted prose. Tool holds the vendor tool name mapped onto a
// canonical verb (see Canonical). Args holds a small, already-redacted subset
// of the call's input: enough to mine a workflow, never enough to reconstruct
// the payload.
type Turn struct {
	Index   int               `json:"index"`
	Actor   Actor             `json:"actor"`
	Kind    Kind              `json:"kind"`
	At      time.Time         `json:"at,omitempty"`
	Text    string            `json:"text,omitempty"`
	Tool    string            `json:"tool,omitempty"`
	Args    map[string]string `json:"args,omitempty"`
	IsError bool              `json:"is_error,omitempty"`
	// Command is the normalized shell command shape when Tool is a shell
	// runner: "git status", "npm run build", "gh pr create". Arguments that
	// carry content (paths, messages, URLs) are already stripped.
	Command string `json:"command,omitempty"`
}

// Session is one conversation with one agent, normalized.
type Session struct {
	ID        string    `json:"id"`
	Agent     string    `json:"agent"`
	Title     string    `json:"title,omitempty"`
	Workspace string    `json:"workspace,omitempty"`
	Repo      string    `json:"repo,omitempty"`
	Branch    string    `json:"branch,omitempty"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Source    string    `json:"source"`
	Turns     []Turn    `json:"turns"`
}

// Duration is the wall time the session spans. A session whose records carry no
// usable timestamps reports zero rather than a fabricated span.
func (s Session) Duration() time.Duration {
	if s.Start.IsZero() || s.End.IsZero() || s.End.Before(s.Start) {
		return 0
	}
	return s.End.Sub(s.Start)
}

// UserTurns returns only the human prompts, in order.
func (s Session) UserTurns() []Turn {
	out := make([]Turn, 0, 8)
	for _, t := range s.Turns {
		if t.Actor == ActorUser && t.Kind == KindMessage && strings.TrimSpace(t.Text) != "" {
			out = append(out, t)
		}
	}
	return out
}

// ToolCalls returns only tool invocations, in order.
func (s Session) ToolCalls() []Turn {
	out := make([]Turn, 0, 32)
	for _, t := range s.Turns {
		if t.Kind == KindToolCall {
			out = append(out, t)
		}
	}
	return out
}

// Finalize sorts turns by their recorded order, backfills timestamps that the
// vendor left empty from the nearest neighbour, and derives Start/End. Adapters
// call it once before handing a session back so the miner can assume ordering.
func (s *Session) Finalize() {
	sort.SliceStable(s.Turns, func(i, j int) bool { return s.Turns[i].Index < s.Turns[j].Index })

	// Backfill forward, then backward, so a run of undated turns inherits the
	// closest real timestamp instead of the zero value. A session where no turn
	// is dated keeps every turn zero, and Duration reports zero.
	var last time.Time
	for i := range s.Turns {
		if s.Turns[i].At.IsZero() {
			s.Turns[i].At = last
		} else {
			last = s.Turns[i].At
		}
	}
	var next time.Time
	for i := len(s.Turns) - 1; i >= 0; i-- {
		if s.Turns[i].At.IsZero() {
			s.Turns[i].At = next
		} else {
			next = s.Turns[i].At
		}
	}

	for _, t := range s.Turns {
		if t.At.IsZero() {
			continue
		}
		if s.Start.IsZero() || t.At.Before(s.Start) {
			s.Start = t.At
		}
		if s.End.IsZero() || t.At.After(s.End) {
			s.End = t.At
		}
	}
	for i := range s.Turns {
		s.Turns[i].Index = i
	}
}

// Stats is the per-agent rollup the scan summary prints.
type Stats struct {
	Agent     string    `json:"agent"`
	Sessions  int       `json:"sessions"`
	Turns     int       `json:"turns"`
	ToolCalls int       `json:"tool_calls"`
	Earliest  time.Time `json:"earliest,omitempty"`
	Latest    time.Time `json:"latest,omitempty"`
}
