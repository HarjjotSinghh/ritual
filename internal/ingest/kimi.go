package ingest

import (
	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"

	"path/filepath"
)

// readKimi reads a Kimi Code session: ~/.kimi-code/sessions/<workspace-hash>/
// <session-id>/state.json — one document holding the whole conversation.
func readKimi(path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	var doc map[string]any
	if err := readJSONFile(path, &doc); err != nil {
		return nil, err
	}
	s := session.Session{Source: path}
	b := newTurnBuilder(lim, red)

	s.ID = str(doc, "sessionId", "session_id", "id")
	if s.ID == "" {
		s.ID = filepath.Base(filepath.Dir(path))
	}
	s.Title = red.Text(str(doc, "title", "summary", "name"))
	s.Workspace = red.Path(workspaceFrom(doc))

	fallback := timeFrom(doc, "createdAt", "created_at", "startTime")
	if fallback.IsZero() {
		fallback = fileTime(path)
	}
	for _, item := range walkMessageArrays(doc) {
		raw, ok := item.(map[string]any)
		if !ok {
			continue
		}
		appendGenericMessage(b, raw, lim, red, fallback)
	}

	s.Turns = b.turns
	s.Repo = repoName(s.Workspace)
	return []session.Session{s}, nil
}
