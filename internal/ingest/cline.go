package ingest

import (
	"path/filepath"
	"strings"
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

	// Timing and the working directory live in the sibling record, not in the
	// message file and not in a ui_messages.json, which this layout does not
	// have at all.
	start, workspace := clineMeta(path)
	if start.IsZero() {
		start = fileTime(path)
	}
	s.Workspace = workspace

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

// clineMeta reads the session record beside the message file: <id>.json next
// to <id>.messages.json. It carries ISO start and end times and the workspace
// root, none of which the message array records.
func clineMeta(messagesPath string) (time.Time, string) {
	base := strings.TrimSuffix(messagesPath, ".messages.json")
	var doc map[string]any
	if err := readJSONFile(base+".json", &doc); err != nil || doc == nil {
		return time.Time{}, ""
	}
	start := timeFrom(doc, "started_at", "startedAt", "time_created", "createdAt", "ts")
	workspace := str(doc, "workspace_root", "workspaceRoot", "cwd", "directory")
	if workspace == "" {
		workspace = workspaceFrom(doc)
	}
	return start, workspace
}
