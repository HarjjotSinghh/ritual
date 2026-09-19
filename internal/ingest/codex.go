package ingest

import (
	"path/filepath"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

// readCodex reads a Codex rollout: ~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl.
//
// Every line is {timestamp, ordinal, type, payload}. The conversation lives in
// response_item payloads; event_msg payloads are the TUI's own event stream and
// duplicate it, so only session_meta and task_complete are taken from there.
//
// The developer role carries the whole system prompt and every AGENTS.md on the
// machine. It is skipped: it is identical across sessions and would swamp the
// intent vocabulary.
func readCodex(path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	s := session.Session{Source: path}
	b := newTurnBuilder(lim, red)

	err := scanJSONL(path, func(raw map[string]any) error {
		at := timeFrom(raw, "timestamp")
		payload := obj(raw, "payload")
		if payload == nil {
			return nil
		}

		switch str(raw, "type") {
		case "session_meta":
			if s.ID == "" {
				s.ID = str(payload, "session_id", "id")
			}
			if s.Workspace == "" {
				s.Workspace = str(payload, "cwd")
			}
			return nil

		case "turn_context":
			if s.Workspace == "" {
				s.Workspace = str(payload, "cwd")
			}
			return nil

		case "response_item":
			switch str(payload, "type") {
			case "message":
				role := str(payload, "role")
				if role == "developer" || role == "system" {
					return nil
				}
				text := textFromContent(payload["content"])
				if role == "user" {
					b.message(session.ActorUser, at, CleanPrompt(text))
				} else {
					b.message(session.ActorAssistant, at, text)
				}
			case "function_call", "custom_tool_call", "local_shell_call", "tool_call":
				name := str(payload, "name", "tool_name")
				args := flattenArgs(payload["input"], lim, red)
				if args == nil {
					args = flattenArgs(payload["arguments"], lim, red)
				}
				if args == nil {
					args = flattenArgs(payload["action"], lim, red)
				}
				b.toolCall(at, codexToolName(name, args), args)
			case "function_call_output", "custom_tool_call_output", "tool_call_output":
				text := textFromContent(payload["output"])
				if text == "" {
					text = str(payload, "output")
				}
				b.toolResult(at, "", text, errorFromResult(payload, text))
			}
			return nil

		case "event_msg":
			if str(payload, "type") == "task_complete" && s.Title == "" {
				s.Title = Gist(red.Text(str(payload, "last_agent_message")), 90)
			}
			return nil
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

// codexToolName resolves Codex's generic sandbox tool into the verb it actually
// performed. `exec` with a command is a shell call; `exec` running a JavaScript
// harness snippet is still a shell call from the workflow's point of view.
func codexToolName(name string, args map[string]string) string {
	if name != "" {
		return name
	}
	if _, ok := args["command"]; ok {
		return "shell"
	}
	if _, ok := args["file_path"]; ok {
		return "read"
	}
	return "tool"
}
