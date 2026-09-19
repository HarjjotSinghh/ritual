package ingest

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

// readGrok reads a Grok CLI session directory:
// ~/.grok/sessions/<url-encoded-cwd>/<session-id>/chat_history.jsonl, with
// summary.json beside it holding the title, the cwd, and the git remote.
//
// Grok records tool calls on the assistant record (`tool_calls` with a JSON
// `arguments` string) and results as their own `tool_result` lines, so both
// halves of a call are present and orderable.
//
// Subagent sessions are recorded exactly like top-level ones. They are skipped
// for the same reason Claude Code sidechains are: the human ran one workflow,
// and counting the delegate's session again would inflate its recurrence.
func readGrok(path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	dir := filepath.Dir(path)
	s := session.Session{Source: path, ID: filepath.Base(dir)}
	b := newTurnBuilder(lim, red)

	var summary map[string]any
	if err := readJSONFile(filepath.Join(dir, "summary.json"), &summary); err == nil && summary != nil {
		if strings.EqualFold(str(summary, "session_kind"), "subagent") {
			return nil, nil
		}
		s.Title = red.Text(str(summary, "session_summary", "title"))
		if info := obj(summary, "info"); info != nil {
			s.Workspace = str(info, "cwd")
			if id := str(info, "id"); id != "" {
				s.ID = id
			}
		}
		if s.Workspace == "" {
			s.Workspace = str(summary, "git_root_dir")
		}
	}
	if s.Workspace == "" {
		s.Workspace = grokWorkspace(dir)
	}
	s.Workspace = red.Path(s.Workspace)

	fallback := fileTime(path)
	err := scanJSONL(path, func(raw map[string]any) error {
		at := timeFrom(raw, "timestamp", "created_at", "time")
		if at.IsZero() {
			at = fallback
		}
		switch str(raw, "type") {
		case "system", "reasoning":
			return nil
		case "user":
			b.message(session.ActorUser, at, CleanPrompt(textFromContent(raw["content"])))
		case "assistant":
			if text := textFromContent(raw["content"]); text != "" {
				b.message(session.ActorAssistant, at, text)
			}
			appendGenericTools(b, raw, lim, red, at)
		case "tool_result":
			text := textFromContent(raw["content"])
			b.toolResult(at, str(raw, "name", "tool"), text, errorFromResult(raw, text))
		default:
			appendGenericMessage(b, raw, lim, red, fallback)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.Turns = b.turns
	s.Repo = repoName(s.Workspace)
	return []session.Session{s}, nil
}

// grokWorkspace decodes the percent-encoded cwd Grok uses as a directory name.
// Unlike a hyphen slug this is lossless, so the result is a real path.
func grokWorkspace(sessionDir string) string {
	encoded := filepath.Base(filepath.Dir(sessionDir))
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		return ""
	}
	return decoded
}
