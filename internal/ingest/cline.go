package ingest

import (
	"path/filepath"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

// readCline reads a Cline task: ~/.cline/tasks/<task-id>/
// api_conversation_history.json — the raw provider conversation, an array of
// Anthropic-shaped {role, content:[blocks]} objects.
//
// The provider history carries no timestamps. Cline keeps those in
// ui_messages.json beside it, so that file supplies the session start and the
// file's modification time supplies the end.
func readCline(path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	var messages []map[string]any
	if err := readJSONFile(path, &messages); err != nil {
		return nil, err
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
