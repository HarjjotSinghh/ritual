package ingest

import (
	"path/filepath"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

// readCursor reads a Cursor CLI transcript:
// ~/.cursor/projects/<path-slug>/agent-transcripts/<id>/<id>.jsonl.
//
// Records are {role, message:{content:[…]}} with Anthropic-shaped blocks, and
// the human prompt is wrapped in <user_query> alongside a <timestamp> banner,
// which CleanPrompt unwraps.
//
// The workspace is not in the records; it is the project directory name, which
// Cursor slugs by replacing separators with hyphens. That is lossy — a hyphen
// in a real directory name is indistinguishable from a separator — so the slug
// is reported as the repo and the workspace is left as the reconstructed path.
func readCursor(path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	s := session.Session{Source: path}
	b := newTurnBuilder(lim, red)

	// Cursor does not timestamp transcript records. The file's modification
	// time is the only date available, so every turn inherits it; that is
	// enough to place a session on the calendar and not enough to order turns
	// within it, which the record order already does.
	fallback := fileTime(path)

	err := scanJSONL(path, func(raw map[string]any) error {
		role := str(raw, "role")
		at := timeFrom(raw, "timestamp", "createdAt", "time")
		if at.IsZero() {
			at = fallback
		}
		msg := obj(raw, "message")
		if msg == nil {
			if text := str(raw, "content", "text"); text != "" {
				if role == "user" {
					b.message(session.ActorUser, at, CleanPrompt(text))
				} else if role == "assistant" {
					b.message(session.ActorAssistant, at, text)
				}
			}
			return nil
		}
		if msgAt := timeFrom(msg, "timestamp", "createdAt"); !msgAt.IsZero() {
			at = msgAt
		}

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
			case "tool_use", "toolUse":
				b.toolCall(at, str(block, "name", "toolName"), flattenArgs(firstPresent(block, "input", "args", "parameters"), lim, red))
			case "tool_result", "toolResult":
				text := textFromContent(firstPresent(block, "content", "result", "output"))
				b.toolResult(at, "", text, errorFromResult(block, text))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.Turns = b.turns
	s.ID = strings.TrimSuffix(filepath.Base(path), ".jsonl")
	s.Workspace = cursorWorkspace(path, red)
	s.Repo = repoName(s.Workspace)
	return []session.Session{s}, nil
}

func firstPresent(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

// cursorWorkspace recovers a readable project label from the slug directory.
// Cursor slugs the absolute path by replacing separators with hyphens, and that
// is not invertible — a hyphen inside a real directory name looks exactly like
// a separator. The reconstruction is therefore a display path: good enough to
// group a workflow by project, never used to touch the filesystem.
func cursorWorkspace(path string, red *redact.Redactor) string {
	slug := segmentBefore(path, "agent-transcripts")
	if slug == "" || slug == "projects" || slug == "empty-window" {
		return ""
	}
	return red.Path("/" + strings.ReplaceAll(slug, "-", "/"))
}
