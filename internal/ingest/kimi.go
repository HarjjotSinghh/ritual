package ingest

import (
	"path/filepath"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

// readKimi reads a Kimi Code transcript: ~/.kimi-code/sessions/<workspace>/
// <session-id>/agents/<agent>/wire.jsonl.
//
// The obvious-looking file, state.json beside it, is a status record: a title,
// the last prompt, the working directory, and a map of agents. It holds no
// message array at any depth, so a reader pointed at it finds a session with
// zero turns and reports the agent as present and empty. The conversation is in
// the wire log, which is two orders of magnitude larger.
//
// The wire log is an event stream rather than a message list. Only three record
// types carry conversation: turn.prompt is what the human sent,
// context.append_message is a completed message, and context.append_loop_event
// wraps the streamed parts of one.
func readKimi(path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	sessionDir := kimiSessionDir(path)
	s := session.Session{Source: path, ID: filepath.Base(sessionDir)}
	b := newTurnBuilder(lim, red)

	// state.json is useless as a transcript and is the only place the title and
	// working directory are recorded, so it is read for those alone.
	var state map[string]any
	if err := readJSONFile(filepath.Join(sessionDir, "state.json"), &state); err == nil && state != nil {
		s.Title = red.Text(str(state, "title", "summary", "name"))
		s.Workspace = red.Path(str(state, "workDir", "workdir", "cwd", "workspaceRoot"))
	}

	fallback := fileTime(path)
	err := scanJSONL(path, func(raw map[string]any) error {
		at := timeFrom(raw, "timestamp", "time", "ts", "createdAt")
		if at.IsZero() {
			at = fallback
		}
		kind := str(raw, "type")
		event := obj(raw, "event")
		if event == nil {
			event = raw
		}

		switch {
		case kind == "turn.prompt":
			b.message(session.ActorUser, at, CleanPrompt(kimiText(raw, event)))
		case strings.HasPrefix(kind, "context.append"):
			if text := kimiText(raw, event); text != "" {
				b.message(session.ActorAssistant, at, text)
			}
			appendGenericTools(b, event, lim, red, at)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.Turns = b.turns
	if s.Workspace == "" {
		s.Workspace = red.Path(filepath.Base(filepath.Dir(sessionDir)))
	}
	s.Repo = repoName(s.Workspace)
	return []session.Session{s}, nil
}

// kimiText digs the prose out of a wire record. The streamed shape puts it at
// event.part.text; the completed shape uses the ordinary message fields.
func kimiText(raw, event map[string]any) string {
	if part := obj(event, "part"); part != nil {
		if t := str(part, "text", "content"); t != "" {
			return t
		}
	}
	if msg := obj(event, "message"); msg != nil {
		if t := genericText(msg); t != "" {
			return t
		}
	}
	if t := genericText(event); t != "" {
		return t
	}
	return genericText(raw)
}

// kimiSessionDir walks up from agents/<agent>/wire.jsonl to the session
// directory that holds state.json.
func kimiSessionDir(path string) string {
	dir := filepath.Dir(path)
	for i := 0; i < 4; i++ {
		if filepath.Base(dir) == "agents" {
			return filepath.Dir(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Dir(filepath.Dir(path))
}
