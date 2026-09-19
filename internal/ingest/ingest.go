// Package ingest turns every supported agent's on-disk session store into
// normalized session.Session values.
//
// Adapters are deliberately lenient. A transcript store is an implementation
// detail of somebody else's product: fields appear, records change shape, a
// half-written file sits at the end of a crashed run. An adapter that fails the
// whole scan because one line did not parse would be useless within a month, so
// every reader skips what it cannot understand, counts it, and continues.
package ingest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/agentspec"
	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

// Limits bound what a single scan will read. They exist because a session store
// is unbounded in practice: a year of daily use is tens of gigabytes, and a
// single pathological transcript can hold a base64 image per turn.
type Limits struct {
	// MaxFileBytes skips any single session file larger than this. 0 means no
	// limit.
	MaxFileBytes int64
	// MaxTextBytes truncates any one turn's text.
	MaxTextBytes int
	// MaxArgBytes truncates any one tool argument value.
	MaxArgBytes int
	// MaxTurns caps turns per session; the remainder is dropped and counted.
	MaxTurns int
}

// DefaultLimits are tuned so a scan of a year of history stays in the low
// hundreds of megabytes of peak memory.
func DefaultLimits() Limits {
	return Limits{
		MaxFileBytes: 256 << 20,
		MaxTextBytes: 8000,
		MaxArgBytes:  400,
		MaxTurns:     4000,
	}
}

// Options control a scan.
type Options struct {
	// Agents restricts the scan to these catalog keys. Empty means every
	// installed agent.
	Agents []string
	// Since drops sessions that ended before this time. Zero means no bound.
	Since time.Time
	// Workspace, when set, keeps only sessions whose workspace path contains
	// this substring.
	Workspace string
	// Limits bound resource use.
	Limits Limits
	// KeepEmails disables email redaction for a local-only run.
	KeepEmails bool
	// Progress, when set, is called as each agent's store is read.
	Progress func(agent string, files, done int)
}

// Result is one scan.
type Result struct {
	Sessions []session.Session       `json:"sessions"`
	Stats    []session.Stats         `json:"stats"`
	Warnings []string                `json:"warnings,omitempty"`
	Roots    map[string]string       `json:"roots,omitempty"`
	Skipped  map[string]int          `json:"skipped,omitempty"`
	Spans    map[string][2]time.Time `json:"-"`
}

type reader func(path string, lim Limits, red *redact.Redactor) ([]session.Session, error)

func readerFor(layout agentspec.Layout) reader {
	switch layout {
	case agentspec.LayoutClaudeJSONL:
		return readClaude
	case agentspec.LayoutCodexRollout:
		return readCodex
	case agentspec.LayoutCursorTranscript:
		return readCursor
	case agentspec.LayoutOpenCodeSQLite:
		return readOpenCode
	case agentspec.LayoutGeminiChat:
		return readGemini
	case agentspec.LayoutGrokSession:
		return readGrok
	case agentspec.LayoutQwenJSONL:
		return readQwen
	case agentspec.LayoutKimiState:
		return readKimi
	case agentspec.LayoutCopilotEvents:
		return readCopilot
	case agentspec.LayoutClineTask:
		return readCline
	case agentspec.LayoutPiJSONL:
		return readPi
	default:
		return nil
	}
}

// Scan discovers and reads every matching session on this machine.
func Scan(opts Options) (*Result, error) {
	if opts.Limits == (Limits{}) {
		opts.Limits = DefaultLimits()
	}
	red := redact.New().KeepEmails(opts.KeepEmails)

	discoveries, err := agentspec.DiscoverAll(opts.Agents)
	if err != nil {
		return nil, fmt.Errorf("discover agent stores: %w", err)
	}

	res := &Result{
		Roots:   make(map[string]string, len(discoveries)),
		Skipped: make(map[string]int, len(discoveries)),
	}

	for _, d := range discoveries {
		res.Roots[d.Spec.Key] = redact.Path(d.Root)
		if d.Skipped > 0 {
			res.Skipped[d.Spec.Key] = d.Skipped
		}
		read := readerFor(d.Spec.Layout)
		if read == nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s: no reader for layout %q", d.Spec.Key, d.Spec.Layout))
			continue
		}

		stats := session.Stats{Agent: d.Spec.Key}
		for i, path := range d.Files {
			if opts.Progress != nil {
				opts.Progress(d.Spec.Key, len(d.Files), i)
			}
			if opts.Limits.MaxFileBytes > 0 && !d.Spec.WholeStore {
				if info, statErr := os.Stat(path); statErr == nil && info.Size() > opts.Limits.MaxFileBytes {
					res.Warnings = append(res.Warnings, fmt.Sprintf("%s: skipped %s (%d bytes over limit)", d.Spec.Key, filepath.Base(path), info.Size()))
					continue
				}
			}
			sessions, readErr := read(path, opts.Limits, red)
			if readErr != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: %s: %v", d.Spec.Key, filepath.Base(path), readErr))
				continue
			}
			for _, s := range sessions {
				s.Agent = d.Spec.Key
				s.Source = redact.Path(s.Source)
				if s.Source == "" {
					s.Source = redact.Path(path)
				}
				s.Finalize()
				if len(s.Turns) == 0 {
					continue
				}
				if !opts.Since.IsZero() && !s.End.IsZero() && s.End.Before(opts.Since) {
					continue
				}
				if opts.Workspace != "" && !strings.Contains(strings.ToLower(s.Workspace), strings.ToLower(opts.Workspace)) {
					continue
				}
				stats.Sessions++
				stats.Turns += len(s.Turns)
				for _, t := range s.Turns {
					if t.Kind == session.KindToolCall {
						stats.ToolCalls++
					}
				}
				if !s.Start.IsZero() && (stats.Earliest.IsZero() || s.Start.Before(stats.Earliest)) {
					stats.Earliest = s.Start
				}
				if !s.End.IsZero() && s.End.After(stats.Latest) {
					stats.Latest = s.End
				}
				res.Sessions = append(res.Sessions, s)
			}
		}
		if opts.Progress != nil {
			opts.Progress(d.Spec.Key, len(d.Files), len(d.Files))
		}
		if stats.Sessions > 0 {
			res.Stats = append(res.Stats, stats)
		}
	}

	sort.SliceStable(res.Sessions, func(i, j int) bool {
		if !res.Sessions[i].Start.Equal(res.Sessions[j].Start) {
			return res.Sessions[i].Start.Before(res.Sessions[j].Start)
		}
		return res.Sessions[i].ID < res.Sessions[j].ID
	})
	sort.Slice(res.Stats, func(i, j int) bool { return res.Stats[i].Agent < res.Stats[j].Agent })
	return res, nil
}

// --- shared record helpers -------------------------------------------------

// scanJSONL streams a JSON Lines file, handing each decoded object to fn.
// Malformed lines are skipped: a crashed agent leaves a half-written last line
// in almost every store, and that must not cost the rest of the file.
func scanJSONL(path string, fn func(raw map[string]any) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	// Transcript lines routinely exceed the default 64 KB: a tool result can
	// carry a whole file.
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] != '{' {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		if err := fn(raw); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	return nil
}

func readJSONFile(path string, into any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, into)
}

func str(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if t != "" {
					return t
				}
			case float64:
				return strconv.FormatFloat(t, 'f', -1, 64)
			case bool:
				return strconv.FormatBool(t)
			}
		}
	}
	return ""
}

func obj(m map[string]any, key string) map[string]any {
	if v, ok := m[key]; ok {
		if o, ok := v.(map[string]any); ok {
			return o
		}
	}
	return nil
}

func arr(m map[string]any, key string) []any {
	if v, ok := m[key]; ok {
		if a, ok := v.([]any); ok {
			return a
		}
	}
	return nil
}

func boolean(m map[string]any, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// parseTime accepts every timestamp shape observed across the catalog: RFC3339
// with and without fractional seconds, epoch seconds, and epoch milliseconds.
func parseTime(v any) time.Time {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return time.Time{}
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.999999", "2006-01-02 15:04:05"} {
			if parsed, err := time.Parse(layout, s); err == nil {
				return parsed.UTC()
			}
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return epoch(float64(n))
		}
	case float64:
		return epoch(t)
	case int64:
		return epoch(float64(t))
	}
	return time.Time{}
}

func epoch(n float64) time.Time {
	if n <= 0 {
		return time.Time{}
	}
	// Anything past ~2001 in seconds is below 1e12; larger values are ms.
	if n > 1e12 {
		return time.UnixMilli(int64(n)).UTC()
	}
	return time.Unix(int64(n), 0).UTC()
}

func timeFrom(m map[string]any, keys ...string) time.Time {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if t := parseTime(v); !t.IsZero() {
				return t
			}
		}
	}
	return time.Time{}
}

// flattenArgs turns a decoded tool input into a flat string map, keeping only
// scalar leaves and a shallow view of nested structures. It is lossy on
// purpose: the miner needs the shape of a call, not its payload.
func flattenArgs(v any, lim Limits, red *redact.Redactor) map[string]string {
	out := make(map[string]string, 8)
	switch t := v.(type) {
	case map[string]any:
		for k, item := range t {
			switch val := item.(type) {
			case string:
				out[k] = val
			case float64:
				out[k] = strconv.FormatFloat(val, 'f', -1, 64)
			case bool:
				out[k] = strconv.FormatBool(val)
			case []any:
				parts := make([]string, 0, len(val))
				for _, e := range val {
					if s, ok := e.(string); ok {
						parts = append(parts, s)
					}
					if len(parts) >= 6 {
						break
					}
				}
				if len(parts) > 0 {
					out[k] = strings.Join(parts, ", ")
				}
			}
		}
	case string:
		// Codex carries a JSON document as a string; decode it when it looks
		// like one so `command` is reachable.
		trimmed := strings.TrimSpace(t)
		if strings.HasPrefix(trimmed, "{") {
			var decoded map[string]any
			if json.Unmarshal([]byte(trimmed), &decoded) == nil {
				return flattenArgs(decoded, lim, red)
			}
		}
		out["input"] = t
	}
	if len(out) == 0 {
		return nil
	}
	return red.Args(out, lim.MaxArgBytes)
}

// textFromContent renders the many content shapes (plain string, block list,
// nested parts) into prose, appending nothing that is not text.
func textFromContent(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var b strings.Builder
		for _, item := range t {
			block, ok := item.(map[string]any)
			if !ok {
				if s, ok := item.(string); ok {
					b.WriteString(s)
					b.WriteString("\n")
				}
				continue
			}
			switch str(block, "type") {
			case "text", "input_text", "output_text", "summary_text", "":
				if s := str(block, "text", "content", "value"); s != "" {
					b.WriteString(s)
					b.WriteString("\n")
				}
			}
		}
		return strings.TrimSpace(b.String())
	case map[string]any:
		return str(t, "text", "content", "value")
	}
	return ""
}

type turnBuilder struct {
	lim   Limits
	red   *redact.Redactor
	turns []session.Turn
	n     int
}

func newTurnBuilder(lim Limits, red *redact.Redactor) *turnBuilder {
	return &turnBuilder{lim: lim, red: red, turns: make([]session.Turn, 0, 64)}
}

func (b *turnBuilder) full() bool { return b.lim.MaxTurns > 0 && len(b.turns) >= b.lim.MaxTurns }

func (b *turnBuilder) message(actor session.Actor, at time.Time, text string) {
	text = strings.TrimSpace(text)
	if text == "" || b.full() {
		return
	}
	b.add(session.Turn{Actor: actor, Kind: session.KindMessage, At: at, Text: b.text(text)})
}

func (b *turnBuilder) toolCall(at time.Time, tool string, args map[string]string) {
	if b.full() {
		return
	}
	canonical := Canonical(tool)
	if canonical == "" {
		return
	}
	t := session.Turn{Actor: session.ActorAssistant, Kind: session.KindToolCall, At: at, Tool: canonical, Args: args}
	if canonical == "shell" {
		t.Command = NormalizeCommand(ExtractCommand(args))
	}
	b.add(t)
}

func (b *turnBuilder) toolResult(at time.Time, tool string, text string, isErr bool) {
	if b.full() {
		return
	}
	b.add(session.Turn{
		Actor: session.ActorTool, Kind: session.KindToolResult, At: at,
		Tool: Canonical(tool), Text: b.text(text), IsError: isErr,
	})
}

func (b *turnBuilder) add(t session.Turn) {
	t.Index = b.n
	b.n++
	b.turns = append(b.turns, t)
}

func (b *turnBuilder) text(s string) string {
	s = b.red.Text(s)
	if b.lim.MaxTextBytes > 0 && len(s) > b.lim.MaxTextBytes {
		s = s[:b.lim.MaxTextBytes] + "…"
	}
	return s
}

// errorFromResult reports whether a tool result looks like a failure. Vendors
// disagree on how to say so, and the miner's friction signal depends on it.
func errorFromResult(m map[string]any, text string) bool {
	if boolean(m, "is_error") || boolean(m, "isError") || boolean(m, "error") {
		return true
	}
	if status := strings.ToLower(str(m, "status", "state")); status == "error" || status == "failed" || status == "failure" {
		return true
	}
	head := strings.ToLower(text)
	if len(head) > 400 {
		head = head[:400]
	}
	for _, marker := range []string{"exit: 1", "exit code 1", "command failed", "traceback (most recent call last)", "error:", "fatal:", "no such file", "permission denied"} {
		if strings.Contains(head, marker) {
			return true
		}
	}
	return false
}
