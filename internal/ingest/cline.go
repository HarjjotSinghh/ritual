package ingest

import (
	"path/filepath"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

// readCline reads a Cline session: ~/.cline/data/sessions/<id>/<id>.messages.json,
// an array of provider messages in the Anthropic {role, content:[blocks]} shape.
//
// Older builds wrote ~/.cline/tasks/<id>/api_conversation_history.json instead.
// Both are handled: the payload is the same array, only the path moved.
func readCline(path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	var messages []map[string]any
	if err := readJSONFile(path, &messages); err != nil {
		// Some builds wrap the array in an object rather than writing it bare.
		var doc map[string]any
		if wrapErr := readJSONFile(path, &doc); wrapErr != nil {
			return nil, err
		}
		for _, item := range walkMessageArrays(doc) {
			if raw, ok := item.(map[string]any); ok {
				messages = append(messages, raw)
			}
		}
	}
	dir := filepath.Dir(path)
	s := session.Session{Source: path, ID: filepath.Base(dir)}
	b := newTurnBuilder(lim, red)

	start := clineStart(filepath.Join(dir, "ui_messages.json"))
	if start.IsZero() {
		start = fileTime(path)
	}

	for _, raw := range messages {
		if s.Workspace == "" {
			s.Workspace = workspaceFrom(raw)
		}
		appendGenericMessage(b, raw, lim, red, start)
	}

	s.Turns = b.turns
	s.Workspace = red.Path(s.Workspace)
	s.Repo = repoName(s.Workspace)
	return []session.Session{s}, nil
}

// clineStart reads the first UI message's timestamp, which is when the human
// opened the task.
func clineStart(uiPath string) time.Time {
	var ui []map[string]any
	if err := readJSONFile(uiPath, &ui); err != nil || len(ui) == 0 {
		return time.Time{}
	}
	return timeFrom(ui[0], "ts", "timestamp", "time")
}
