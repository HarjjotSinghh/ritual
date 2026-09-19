package ingest

import (
	"path/filepath"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

// readPi reads a Pi session: ~/.pi/agent/sessions/<cwd-slug>/*.jsonl.
//
// The layout mirrors Claude Code's — a path slug per workspace, one JSONL per
// session — while the records follow the shared message shape.
func readPi(path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	s := session.Session{Source: path}
	b := newTurnBuilder(lim, red)

	fallback := fileTime(path)
	err := scanJSONL(path, func(raw map[string]any) error {
		if s.ID == "" {
			s.ID = str(raw, "sessionId", "session_id", "id")
		}
		if s.Workspace == "" {
			s.Workspace = workspaceFrom(raw)
		}
		if s.Title == "" {
			s.Title = red.Text(str(raw, "title", "summary"))
		}
		appendGenericMessage(b, raw, lim, red, fallback)
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.Turns = b.turns
	if s.ID == "" {
		s.ID = strings.TrimSuffix(filepath.Base(path), ".jsonl")
	}
	if s.Workspace == "" {
		s.Workspace = red.Path(strings.ReplaceAll(filepath.Base(filepath.Dir(path)), "-", "/"))
	} else {
		s.Workspace = red.Path(s.Workspace)
	}
	s.Repo = repoName(s.Workspace)
	return []session.Session{s}, nil
}
