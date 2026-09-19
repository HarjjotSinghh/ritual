package ingest

import (
	"path/filepath"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

// readCopilot reads a GitHub Copilot CLI session:
// ~/.copilot/session-state/<session-id>/events.jsonl.
//
// The file is an event log rather than a message list, so a record may be a
// state transition with no conversational content. Those are ignored; the
// message-shaped events carry the same fields the shared reader handles.
func readCopilot(path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	s := session.Session{Source: path, ID: filepath.Base(filepath.Dir(path))}
	b := newTurnBuilder(lim, red)

	fallback := fileTime(path)
	err := scanJSONL(path, func(raw map[string]any) error {
		if s.Workspace == "" {
			s.Workspace = workspaceFrom(raw)
		}
		// An event envelope wraps the payload; unwrap it when present so the
		// shared reader sees the message itself.
		if inner := obj(raw, "data"); inner != nil && roleOf(raw) == session.ActorUnknown {
			if roleOf(inner) != session.ActorUnknown || genericText(inner) != "" {
				appendGenericMessage(b, inner, lim, red, fallback)
				return nil
			}
		}
		appendGenericMessage(b, raw, lim, red, fallback)
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.Turns = b.turns
	s.Workspace = red.Path(s.Workspace)
	s.Repo = repoName(s.Workspace)
	return []session.Session{s}, nil
}
