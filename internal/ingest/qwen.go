package ingest

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

// readQwen reads a Qwen Code chat: ~/.qwen/projects/<path-slug>/chats/*.jsonl.
//
// Qwen Code is a Gemini CLI fork, so the records carry the Gemini `parts` shape
// while the directory layout follows Claude Code's path slug. Both halves are
// already handled; this reader only joins them.
func readQwen(path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	s := session.Session{Source: path}
	b := newTurnBuilder(lim, red)

	fallback := fileTime(path)
	err := scanJSONL(path, func(raw map[string]any) error {
		if s.ID == "" {
			s.ID = str(raw, "sessionId", "session_id")
		}
		if s.Workspace == "" {
			s.Workspace = workspaceFrom(raw)
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
		s.Workspace = slugWorkspace(path, "chats", red)
	} else {
		s.Workspace = red.Path(s.Workspace)
	}
	s.Repo = repoName(s.Workspace)
	return []session.Session{s}, nil
}

// fileTime is the fallback timestamp for a store that dates records only at
// the file level. It is the modification time, which is the end of the
// session rather than its start; that is close enough for cadence and is
// labelled as approximate wherever it reaches a report.
func fileTime(path string) time.Time {
	if info, err := os.Stat(path); err == nil {
		return info.ModTime().UTC()
	}
	return time.Time{}
}

// slugWorkspace reconstructs a display path from the path-slug directory that
// sits immediately before marker.
func slugWorkspace(path, marker string, red *redact.Redactor) string {
	slug := segmentBefore(path, marker)
	if slug == "" || slug == "projects" {
		return ""
	}
	return red.Path("/" + strings.ReplaceAll(slug, "-", "/"))
}

// segmentBefore returns the path segment immediately preceding marker, or ""
// when marker is absent or first. Walking a fixed number of parents instead
// breaks the moment a vendor adds or removes one level of nesting, which every
// one of these stores has done at least once.
func segmentBefore(path, marker string) string {
	segments := strings.Split(filepath.ToSlash(path), "/")
	for i := len(segments) - 1; i > 0; i-- {
		if segments[i] == marker {
			return segments[i-1]
		}
	}
	return ""
}
