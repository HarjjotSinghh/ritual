package ingest

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

// readGemini reads a Gemini CLI chat: ~/.gemini/tmp/<project-hash>/chats/
// session-*.json (older builds) or chats/*.json.
//
// The project directory is a hash of the workspace path, so the workspace is
// only recoverable when the document records it. When it does not, the hash is
// kept as an opaque project label: grouping by it is still correct, it just
// cannot be printed as a path.
func readGemini(path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	var doc map[string]any
	if err := readJSONFile(path, &doc); err != nil {
		// A session killed mid-write leaves an unterminated document. Losing
		// the whole file over its last record is the wrong trade, so the
		// records that did land are recovered line by line.
		recovered, lineErr := recoverRecords(path)
		if lineErr != nil || len(recovered) == 0 {
			return nil, err
		}
		doc = map[string]any{"messages": recovered}
	}
	s := session.Session{Source: path}
	b := newTurnBuilder(lim, red)

	s.ID = str(doc, "sessionId", "session_id", "id")
	s.Title = red.Text(str(doc, "title", "summary", "name"))
	s.Workspace = red.Path(workspaceFrom(doc))

	fallback := timeFrom(doc, "startTime", "start_time", "createdAt", "lastUpdated")
	if fallback.IsZero() {
		if info, err := os.Stat(path); err == nil {
			fallback = info.ModTime().UTC()
		}
	}

	for _, item := range walkMessageArrays(doc) {
		raw, ok := item.(map[string]any)
		if !ok {
			continue
		}
		appendGenericMessage(b, raw, lim, red, fallback)
	}

	s.Turns = b.turns
	if s.ID == "" {
		s.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if s.Workspace == "" {
		s.Repo = geminiProjectHash(path)
	} else {
		s.Repo = repoName(s.Workspace)
	}
	return []session.Session{s}, nil
}

// geminiProjectHash returns the tmp/<hash> segment, which identifies the
// project without naming it.
func geminiProjectHash(path string) string {
	dir := filepath.Dir(path)
	if filepath.Base(dir) == "chats" {
		dir = filepath.Dir(dir)
	}
	base := filepath.Base(dir)
	if base == "" || base == "." || base == "tmp" {
		return ""
	}
	if len(base) > 12 {
		base = base[:12]
	}
	return "gemini:" + base
}
