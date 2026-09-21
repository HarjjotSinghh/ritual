package ingest

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"

	_ "modernc.org/sqlite"
)

// readOpenCode reads every session out of OpenCode's embedded store,
// ~/.local/share/opencode/opencode.db.
//
// Unlike every other agent in the catalog, one file holds the whole history, so
// this reader returns many sessions. The store is opened read-only: OpenCode
// may be running, and its write-ahead log must be visible or a scan would
// silently miss the current session. The database is copied to a temporary file
// first when it cannot be opened in place, because a WAL-mode database on a
// read-only filesystem refuses to open otherwise.
//
// OpenCode migrated its session store and left the old tables in place. A
// machine that has been through the migration keeps a few dozen rows in the
// legacy tables and everything else in the new ones, so a reader that knows
// only the old names finds a token sample of the history and reports it without
// complaint — which is exactly what this one did until a 1525-session store
// came back showing 31. The schema is therefore detected rather than assumed.
func readOpenCode(path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	db, cleanup, err := openReadOnly(path)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	tables, err := tableSet(db)
	if err != nil {
		return nil, err
	}
	if _, ok := tables["session_v2"]; ok {
		return readOpenCodeV2(db, path, lim, red)
	}
	return readOpenCodeLegacy(db, path, lim, red)
}

// tableSet lists the tables present, so the reader can pick a schema.
func tableSet(db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	out := make(map[string]struct{}, 32)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		out[name] = struct{}{}
	}
	return out, rows.Err()
}

// openCodeSession is the shared shape of a row from either session table.
type openCodeSession struct {
	id        string
	directory string
	title     string
	created   int64
	updated   int64
	parent    string
}

// loadOpenCodeSessions reads whichever session table was named, skipping child
// sessions. A child is a subagent run spawned by its parent, and counting it
// separately would inflate the recurrence of whatever the parent did.
func loadOpenCodeSessions(db *sql.DB, table string) ([]openCodeSession, error) {
	query := fmt.Sprintf(`SELECT id, COALESCE(directory,''), COALESCE(title,''),
		COALESCE(time_created,0), COALESCE(time_updated,0), COALESCE(parent_id,'')
		FROM %q`, table)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", table, err)
	}
	defer rows.Close()

	out := make([]openCodeSession, 0, 512)
	for rows.Next() {
		var s openCodeSession
		if err := rows.Scan(&s.id, &s.directory, &s.title, &s.created, &s.updated, &s.parent); err != nil {
			continue
		}
		if s.parent != "" {
			continue
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// readOpenCodeV2 reads the current schema: session_v2 plus session_message,
// where the message's role is a real column and its parts are inline in the
// payload rather than in a separate table.
func readOpenCodeV2(db *sql.DB, path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	sessions, err := loadOpenCodeSessions(db, "session_v2")
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, nil
	}

	index := make(map[string]*session.Session, len(sessions))
	builders := make(map[string]*turnBuilder, len(sessions))
	order := make([]string, 0, len(sessions))
	for _, row := range sessions {
		s := &session.Session{
			ID:        row.id,
			Title:     red.Text(row.title),
			Workspace: red.Path(row.directory),
			Source:    path,
			Start:     epoch(float64(row.created)),
			End:       epoch(float64(row.updated)),
		}
		s.Repo = repoName(s.Workspace)
		index[row.id] = s
		builders[row.id] = newTurnBuilder(lim, red)
		order = append(order, row.id)
	}

	type messageRow struct {
		role    string
		seq     int64
		created int64
		data    string
	}
	bySession := make(map[string][]messageRow, len(index))

	rows, err := db.Query(`SELECT session_id, COALESCE(type,''), COALESCE(seq,0),
		COALESCE(time_created,0), COALESCE(data,'{}') FROM session_message`)
	if err != nil {
		return nil, fmt.Errorf("query session_message: %w", err)
	}
	for rows.Next() {
		var sessionID string
		var m messageRow
		if err := rows.Scan(&sessionID, &m.role, &m.seq, &m.created, &m.data); err != nil {
			continue
		}
		if _, ok := index[sessionID]; !ok {
			continue
		}
		bySession[sessionID] = append(bySession[sessionID], m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan session_message: %w", err)
	}

	for sessionID, messages := range bySession {
		sort.SliceStable(messages, func(i, j int) bool { return messages[i].seq < messages[j].seq })
		b := builders[sessionID]
		for _, m := range messages {
			// agent-switched, synthetic, and system rows are bookkeeping: they
			// record a mode change or a harness insertion, not a turn anyone
			// took.
			role := strings.ToLower(m.role)
			if role != "user" && role != "assistant" {
				continue
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(m.data), &payload); err != nil {
				continue
			}
			at := epoch(float64(m.created))
			if t := timeFrom(payload, "time"); !t.IsZero() {
				at = t
			}
			sawText := false
			for _, item := range arr(payload, "content") {
				part, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if appendOpenCodePart(b, part, role, at, lim, red) {
					sawText = true
				}
			}
			// Most messages carry their prose in content[], but a large
			// minority put it directly on the payload instead. Reading only the
			// array found 594 sessions in a store holding 1520 and called the
			// rest empty, which is the same silent-success failure as reading
			// the wrong table.
			if !sawText {
				if text := str(payload, "text"); text != "" {
					if strings.EqualFold(role, "user") {
						b.message(session.ActorUser, at, CleanPrompt(text))
					} else {
						b.message(session.ActorAssistant, at, text)
					}
				}
			}
		}
	}

	return collectOpenCode(index, builders, order), nil
}

// readOpenCodeLegacy reads the pre-migration schema: session, message, and a
// separate part table.
func readOpenCodeLegacy(db *sql.DB, path string, lim Limits, red *redact.Redactor) ([]session.Session, error) {
	sessions, err := loadOpenCodeSessions(db, "session")
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, nil
	}

	index := make(map[string]*session.Session, len(sessions))
	builders := make(map[string]*turnBuilder, len(sessions))
	order := make([]string, 0, len(sessions))
	for _, row := range sessions {
		s := &session.Session{
			ID:        row.id,
			Title:     red.Text(row.title),
			Workspace: red.Path(row.directory),
			Source:    path,
			Start:     epoch(float64(row.created)),
			End:       epoch(float64(row.updated)),
		}
		s.Repo = repoName(s.Workspace)
		index[row.id] = s
		builders[row.id] = newTurnBuilder(lim, red)
		order = append(order, row.id)
	}

	type msgRow struct {
		id        string
		sessionID string
		created   int64
		role      string
	}
	messages := make(map[string]msgRow, 256)
	msgOrder := make(map[string][]msgRow, len(index))

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
		if _, ok := index[sessionID]; !ok {
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
				_ = appendOpenCodePart(b, payload, m.role, partAt, lim, red)
			}
		}
	}

	return collectOpenCode(index, builders, order), nil
}

// appendOpenCodePart turns one part into turns and reports whether it yielded
// prose, which is what decides if the payload-level fallback is needed. Both
// schemas use the same part shape; only where the part is stored changed.
func appendOpenCodePart(b *turnBuilder, part map[string]any, role string, at time.Time, lim Limits, red *redact.Redactor) (sawText bool) {
	switch str(part, "type") {
	case "text":
		text := str(part, "text")
		if strings.TrimSpace(text) == "" {
			return false
		}
		if strings.EqualFold(role, "user") {
			b.message(session.ActorUser, at, CleanPrompt(text))
		} else {
			b.message(session.ActorAssistant, at, text)
		}
		return true
	case "tool", "tool-call", "tool_use":
		state := obj(part, "state")
		name := str(part, "tool", "name")
		var args map[string]string
		if state != nil {
			args = flattenArgs(state["input"], lim, red)
		}
		if args == nil {
			args = flattenArgs(part["input"], lim, red)
		}
		b.toolCall(at, name, args)
		if state != nil {
			output := str(state, "output", "error")
			status := strings.ToLower(str(state, "status"))
			if output != "" || status == "error" {
				b.toolResult(at, name, output, status == "error" || errorFromResult(state, output))
			}
		}
	case "patch":
		// A patch part names the files a turn changed, which is the clearest
		// signal of what a workflow touches.
		if files := arr(part, "files"); len(files) > 0 {
			names := make([]string, 0, len(files))
			for _, f := range files {
				if s, ok := f.(string); ok {
					names = append(names, red.Path(s))
				}
			}
			b.toolCall(at, "edit", map[string]string{"files": strings.Join(names, ", ")})
		}
	}
	return false
}

// collectOpenCode assembles the finished sessions in a stable order.
func collectOpenCode(index map[string]*session.Session, builders map[string]*turnBuilder, order []string) []session.Session {
	sort.Strings(order)
	out := make([]session.Session, 0, len(order))
	for _, id := range order {
		s := index[id]
		s.Turns = builders[id].turns
		if len(s.Turns) == 0 {
			continue
		}
		out = append(out, *s)
	}
	return out
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
