package ingest

import (
	"path/filepath"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

// readClaude reads one Claude Code transcript: ~/.claude/projects/<path-slug>/
// <session-uuid>.jsonl, one JSON object per line.
//
// The file interleaves real conversation with harness bookkeeping
// (queue-operation, ai-title, atis-latch, attachment). Only user and assistant
// records carry a message; everything else is metadata, except ai-title, which
// is the only place a human-readable session title exists.
func readClaude(path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	s := session.Session{Source: path}
	b := newTurnBuilder(lim, red)

	err := scanJSONL(path, func(raw map[string]any) error {
		switch str(raw, "type") {
		case "ai-title":
			if s.Title == "" {
				s.Title = red.Text(str(raw, "aiTitle"))
			}
			return nil
		case "user", "assistant":
		default:
			return nil
		}

		if s.ID == "" {
			s.ID = str(raw, "sessionId")
		}
		if s.Workspace == "" {
			s.Workspace = str(raw, "cwd")
		}
		if s.Branch == "" {
			s.Branch = str(raw, "gitBranch")
		}
		// Sidechain records are subagent conversations. They are real work, but
		// attributing them to the human's prompt would double-count a workflow
		// the operator ran once.
		if boolean(raw, "isSidechain") {
			return nil
		}

		// promptSource distinguishes what the human typed from what the
		// harness submitted on their behalf. An SDK prompt is a programmatic
		// review or automation run; a system prompt is a task notification.
		// Both are user-role records, neither is a human intent, and counting
		// them produces a "workflow" the operator never performed.
		if role := str(raw, "promptSource"); role == "sdk" || role == "system" {
			return nil
		}

		at := timeFrom(raw, "timestamp")
		msg := obj(raw, "message")
		if msg == nil {
			return nil
		}
		role := str(msg, "role")
		content, ok := msg["content"]
		if !ok {
			return nil
		}

		if text, isString := content.(string); isString {
			if role == "user" {
				b.message(session.ActorUser, at, CleanPrompt(text))
			} else {
				b.message(session.ActorAssistant, at, text)
			}
			return nil
		}

		blocks, ok := content.([]any)
		if !ok {
			return nil
		}
		for _, item := range blocks {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			switch str(block, "type") {
			case "text":
				text := str(block, "text")
				if role == "user" {
					b.message(session.ActorUser, at, CleanPrompt(text))
				} else {
					b.message(session.ActorAssistant, at, text)
				}
			case "tool_use":
				b.toolCall(at, str(block, "name"), flattenArgs(block["input"], lim, red))
			case "tool_result":
				text := textFromContent(block["content"])
				b.toolResult(at, "", text, errorFromResult(block, text))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.Turns = b.turns
	if s.ID == "" {
		s.ID = strings.TrimSuffix(filepath.Base(path), ".jsonl")
	}
	s.Workspace = red.Path(s.Workspace)
	s.Repo = repoName(s.Workspace)
	return []session.Session{s}, nil
}

// repoName is the last path segment of a workspace, which is what a human calls
// the project. A worktree path keeps its own name, which is correct: work in a
// worktree is work in a different checkout.
func repoName(workspace string) string {
	workspace = strings.TrimRight(workspace, "/\\")
	if workspace == "" {
		return ""
	}
	return filepath.Base(workspace)
}
