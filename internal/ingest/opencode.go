package ingest

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"

	_ "modernc.org/sqlite"
)

// readOpenCode reads every session out of OpenCode's embedded store,
// ~/.local/share/opencode/opencode.db.
//
// Unlike every other agent in the catalog, one file holds the whole history, so
// this reader returns many sessions. The store is opened read-only over a URI
// with immutable=0: OpenCode may be running, and its write-ahead log must be
// visible or a scan would silently miss the current session. The database is
// copied to a temporary file first when it cannot be opened in place, because a
// WAL-mode database on a read-only filesystem refuses to open otherwise.
//
// Rows are shaped as {id, session_id, data} with the payload as JSON, so the
// schema is stable across OpenCode releases even as the payload evolves.
func readOpenCode(path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	db, cleanup, err := openReadOnly(path)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	type msgRow struct {
		id        string
		sessionID string
		created   int64
		role      string
	}

	sessions := make(map[string]*session.Session)
	order := make([]string, 0, 32)

	rows, err := db.Query(`SELECT id, COALESCE(directory,''), COALESCE(title,''), COALESCE(time_created,0), COALESCE(time_updated,0), COALESCE(parent_id,'') FROM session`)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	for rows.Next() {
		var id, dir, title, parent string
		var created, updated int64
		if err := rows.Scan(&id, &dir, &title, &created, &updated, &parent); err != nil {
			continue
		}
		// A child session is a subagent run spawned by its parent. Counting it
		// separately would inflate the recurrence of whatever the parent did.
		if parent != "" {
			continue
		}
		s := &session.Session{
			ID:        id,
			Title:     red.Text(title),
			Workspace: red.Path(dir),
			Source:    path,
			Start:     epoch(float64(created)),
			End:       epoch(float64(updated)),
		}
		s.Repo = repoName(s.Workspace)
		sessions[id] = s
		order = append(order, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan sessions: %w", err)
	}
	if len(sessions) == 0 {
		return nil, nil
	}

	messages := make(map[string]msgRow, 256)
	msgOrder := make(map[string][]msgRow, len(sessions))
	mrows, err := db.Query(`SELECT id, session_id, COALESCE(time_created,0), COALESCE(data,'{}') FROM message`)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	for mrows.Next() {
		var id, sessionID, data string
		var created int64
		if err := mrows.Scan(&id, &sessionID, &created, &data); err != nil {
			continue
		}
		if _, ok := sessions[sessionID]; !ok {
			continue
		}
		var payload map[string]any
		_ = json.Unmarshal([]byte(data), &payload)
		row := msgRow{id: id, sessionID: sessionID, created: created, role: str(payload, "role")}
		messages[id] = row
		msgOrder[sessionID] = append(msgOrder[sessionID], row)
	}
	mrows.Close()
	if err := mrows.Err(); err != nil {
		return nil, fmt.Errorf("scan messages: %w", err)
	}

	type partRow struct {
		created int64
		data    string
	}
	parts := make(map[string][]partRow, len(messages))
	prows, err := db.Query(`SELECT message_id, COALESCE(time_created,0), COALESCE(data,'{}') FROM part`)
	if err != nil {
		return nil, fmt.Errorf("query parts: %w", err)
	}
	for prows.Next() {
		var messageID, data string
		var created int64
		if err := prows.Scan(&messageID, &created, &data); err != nil {
			continue
		}
		if _, ok := messages[messageID]; !ok {
			continue
		}
		parts[messageID] = append(parts[messageID], partRow{created: created, data: data})
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return nil, fmt.Errorf("scan parts: %w", err)
	}

	builders := make(map[string]*turnBuilder, len(sessions))
	for id := range sessions {
		builders[id] = newTurnBuilder(lim, red)
	}

	for sessionID, msgs := range msgOrder {
		sort.SliceStable(msgs, func(i, j int) bool { return msgs[i].created < msgs[j].created })
		b := builders[sessionID]
		for _, m := range msgs {
			list := parts[m.id]
			sort.SliceStable(list, func(i, j int) bool { return list[i].created < list[j].created })
			at := epoch(float64(m.created))
			for _, p := range list {
				var payload map[string]any
				if err := json.Unmarshal([]byte(p.data), &payload); err != nil {
					continue
				}
				partAt := at
				if t := timeFrom(payload, "time"); !t.IsZero() {
					partAt = t
				}
				switch str(payload, "type") {
				case "text":
					text := str(payload, "text")
					if m.role == "user" {
						b.message(session.ActorUser, partAt, CleanPrompt(text))
					} else {
						b.message(session.ActorAssistant, partAt, text)
					}
				case "tool":
					state := obj(payload, "state")
					name := str(payload, "tool")
					var args map[string]string
					if state != nil {
						args = flattenArgs(state["input"], lim, red)
					}
					b.toolCall(partAt, name, args)
					if state != nil {
						output := str(state, "output", "error")
						status := strings.ToLower(str(state, "status"))
						if output != "" || status == "error" {
							b.toolResult(partAt, name, output, status == "error" || errorFromResult(state, output))
						}
					}
				case "patch":
					// A patch part names the files a turn changed, which is the
					// clearest signal of what a workflow touches.
					if files := arr(payload, "files"); len(files) > 0 {
						names := make([]string, 0, len(files))
						for _, f := range files {
							if s, ok := f.(string); ok {
								names = append(names, red.Path(s))
							}
						}
						b.toolCall(partAt, "edit", map[string]string{"files": strings.Join(names, ", ")})
					}
				}
			}
		}
	}

	out := make([]session.Session, 0, len(order))
	sort.Strings(order)
	for _, id := range order {
		s := sessions[id]
		s.Turns = builders[id].turns
		if len(s.Turns) == 0 {
			continue
		}
		out = append(out, *s)
	}
	return out, nil
}

// openReadOnly opens a SQLite database without taking a write lock or creating
// sidecar files. When the database is in WAL mode and its directory is not
// writable, it is copied to a scratch file first along with its -wal and -shm
// companions, so the copy is consistent rather than a torn read.
func openReadOnly(path string) (*sql.DB, func(), error) {
	noop := func() {}
	dsn := "file:" + path + "?mode=ro&_pragma=busy_timeout(3000)&_pragma=journal_mode(wal)"
	db, err := sql.Open("sqlite", dsn)
	if err == nil {
		if pingErr := db.Ping(); pingErr == nil {
			return db, func() { db.Close() }, nil
		}
		db.Close()
	}

	tmp, err := os.MkdirTemp("", "ritual-opencode-")
	if err != nil {
		return nil, noop, fmt.Errorf("stage database copy: %w", err)
	}
	cleanup := func() { os.RemoveAll(tmp) }
	target := filepath.Join(tmp, filepath.Base(path))
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := copyFile(path+suffix, target+suffix); err != nil && suffix == "" {
			cleanup()
			return nil, noop, err
		}
	}
	db, err = sql.Open("sqlite", "file:"+target+"?mode=ro")
	if err != nil {
		cleanup()
		return nil, noop, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		cleanup()
		return nil, noop, fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	return db, func() { db.Close(); cleanup() }, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
