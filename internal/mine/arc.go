package mine

import (
	"path"
	"sort"
	"strings"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/ingest"
	"github.com/HarjjotSinghh/ritual/internal/session"
	"github.com/HarjjotSinghh/ritual/internal/textutil"
)

// SegmentOptions tune how sessions are cut into arcs.
type SegmentOptions struct {
	// IdleSplit starts a new arc when this much time passes between turns,
	// even without a new prompt. A session left open overnight and resumed the
	// next morning is two pieces of work, not one.
	IdleSplit time.Duration
	// MinSteps drops arcs with fewer actions than this. A prompt answered with
	// prose is a conversation, not a workflow.
	MinSteps int
	// MaxPrompts caps how many prompts are retained per arc.
	MaxPrompts int
}

// DefaultSegmentOptions are the values the scan uses unless overridden.
func DefaultSegmentOptions() SegmentOptions {
	return SegmentOptions{IdleSplit: 90 * time.Minute, MinSteps: 2, MaxPrompts: 12}
}

// continuationMarkers are prompts that carry no intent of their own. They
// extend the arc in progress instead of starting a new one, because treating
// "yes, do that" as a task would create a cluster of empty workflows.
var continuationMarkers = []string{
	"yes", "yep", "yeah", "ok", "okay", "sure", "go ahead", "go on", "continue",
	"proceed", "do it", "sounds good", "perfect", "thanks", "thank you", "great",
	"nice", "lgtm", "ship it", "approved", "y", "n", "no", "next", "keep going",
	"carry on", "fix it", "try again", "retry", "again", "same", "done",
}

// Segment cuts sessions into task arcs.
func Segment(sessions []session.Session, opts SegmentOptions) []Arc {
	if opts.IdleSplit <= 0 {
		opts = DefaultSegmentOptions()
	}
	arcs := make([]Arc, 0, len(sessions)*2)
	for _, s := range sessions {
		arcs = append(arcs, segmentSession(s, opts)...)
	}
	sort.SliceStable(arcs, func(i, j int) bool {
		if !arcs[i].Start.Equal(arcs[j].Start) {
			return arcs[i].Start.Before(arcs[j].Start)
		}
		return arcs[i].ID < arcs[j].ID
	})
	return arcs
}

type arcAccumulator struct {
	arc        Arc
	lastAction string
	lastError  bool
	stepSeen   map[string]struct{}
	toolSeen   map[string]struct{}
	cmdSeen    map[string]struct{}
	pathSeen   map[string]struct{}
	open       bool
}

func segmentSession(s session.Session, opts SegmentOptions) []Arc {
	out := make([]Arc, 0, 4)
	acc := newAccumulator(s)
	var prevAt time.Time

	flush := func() {
		if !acc.open {
			return
		}
		a := acc.finish()
		if len(a.Steps) >= opts.MinSteps || len(a.Corrections) > 0 {
			out = append(out, a)
		}
		acc = newAccumulator(s)
	}

	for _, t := range s.Turns {
		if !t.At.IsZero() && !prevAt.IsZero() && t.At.Sub(prevAt) > opts.IdleSplit {
			flush()
		}
		if !t.At.IsZero() {
			prevAt = t.At
		}

		switch {
		case t.Actor == session.ActorUser && t.Kind == session.KindMessage:
			text := strings.TrimSpace(t.Text)
			if text == "" {
				continue
			}
			if !acc.open {
				acc.begin(t, s)
				continue
			}
			if isContinuation(text) {
				// A short redirection inside live work is a correction, not a
				// new task. It is the strongest signal ritual has, so it is
				// kept even though the text is too thin to cluster on.
				if c, ok := classifyCorrection(text, acc.lastAction, t, s); ok {
					acc.arc.Corrections = append(acc.arc.Corrections, c)
				}
				acc.addPrompt(text, opts.MaxPrompts)
				continue
			}
			if c, ok := classifyCorrection(text, acc.lastAction, t, s); ok {
				acc.arc.Corrections = append(acc.arc.Corrections, c)
				acc.addPrompt(text, opts.MaxPrompts)
				continue
			}
			// A fresh, substantive prompt ends the previous arc.
			flush()
			acc.begin(t, s)

		case t.Kind == session.KindToolCall:
			if !acc.open {
				// Work with no prompt in front of it is a resumed session or a
				// harness-initiated turn. It still counts as work, so an
				// anonymous arc is opened for it.
				acc.beginAnonymous(t, s)
			}
			acc.addStep(t)

		case t.Kind == session.KindToolResult:
			if !acc.open {
				continue
			}
			if t.IsError {
				acc.arc.Errors++
				acc.lastError = true
			} else {
				acc.lastError = false
			}

		case t.Actor == session.ActorAssistant && t.Kind == session.KindMessage:
			if acc.open {
				acc.arc.Turns++
				acc.touch(t.At)
			}
		}
	}
	flush()
	return out
}

func newAccumulator(s session.Session) *arcAccumulator {
	return &arcAccumulator{
		arc: Arc{
			Agent: s.Agent, SessionID: s.ID, Source: s.Source,
			Repo: s.Repo, Workspace: s.Workspace, Branch: s.Branch,
		},
		stepSeen: map[string]struct{}{},
		toolSeen: map[string]struct{}{},
		cmdSeen:  map[string]struct{}{},
		pathSeen: map[string]struct{}{},
	}
}

func (a *arcAccumulator) begin(t session.Turn, s session.Session) {
	a.open = true
	refs, cleaned := ingest.ExtractSkillRefs(t.Text)
	a.arc.Skills = refs
	a.arc.Intent = cleaned
	a.arc.Prompts = []string{cleaned}
	a.arc.Start = t.At
	a.arc.End = t.At
	a.arc.Turns = 1
	a.arc.ID = textutil.Fingerprint(s.Agent, s.ID, t.At.Format(time.RFC3339), textutil.Truncate(t.Text, 80))
}

func (a *arcAccumulator) beginAnonymous(t session.Turn, s session.Session) {
	a.open = true
	a.arc.Start = t.At
	a.arc.End = t.At
	a.arc.ID = textutil.Fingerprint(s.Agent, s.ID, t.At.Format(time.RFC3339), "anonymous")
}

func (a *arcAccumulator) addPrompt(text string, max int) {
	a.arc.Turns++
	refs, cleaned := ingest.ExtractSkillRefs(text)
	for _, r := range refs {
		if !contains(a.arc.Skills, r) {
			a.arc.Skills = append(a.arc.Skills, r)
		}
	}
	if max > 0 && len(a.arc.Prompts) >= max {
		return
	}
	a.arc.Prompts = append(a.arc.Prompts, cleaned)
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func (a *arcAccumulator) addStep(t session.Turn) {
	a.arc.Turns++
	a.touch(t.At)

	action := t.Tool
	if t.Command != "" {
		action = "shell:" + t.Command
	}
	if action == "" {
		return
	}
	// A step repeated immediately after a failure is a retry: the same action,
	// tried again because it did not work the first time.
	if action == a.lastAction && a.lastError {
		a.arc.Retries++
	}
	if action != a.lastAction {
		a.arc.Steps = append(a.arc.Steps, action)
	}
	a.lastAction = action

	if _, ok := a.toolSeen[t.Tool]; !ok && t.Tool != "" {
		a.toolSeen[t.Tool] = struct{}{}
		a.arc.Tools = append(a.arc.Tools, t.Tool)
	}
	if t.Command != "" {
		if _, ok := a.cmdSeen[t.Command]; !ok {
			a.cmdSeen[t.Command] = struct{}{}
			a.arc.Commands = append(a.arc.Commands, t.Command)
		}
	}
	for _, shape := range pathShapes(t.Args) {
		if _, ok := a.pathSeen[shape]; ok {
			continue
		}
		a.pathSeen[shape] = struct{}{}
		a.arc.Paths = append(a.arc.Paths, shape)
	}
}

func (a *arcAccumulator) touch(at time.Time) {
	if at.IsZero() {
		return
	}
	if a.arc.Start.IsZero() || at.Before(a.arc.Start) {
		a.arc.Start = at
	}
	if at.After(a.arc.End) {
		a.arc.End = at
	}
}

func (a *arcAccumulator) finish() Arc {
	a.open = false
	arc := a.arc
	if arc.Intent == "" && len(arc.Steps) > 0 {
		arc.Intent = "untitled work: " + strings.Join(arc.Steps[:min(3, len(arc.Steps))], ", ")
	}
	sort.Strings(arc.Tools)
	sort.Strings(arc.Commands)
	sort.Strings(arc.Paths)
	return arc
}

func isContinuation(text string) bool {
	normalized := strings.ToLower(strings.Trim(strings.TrimSpace(text), ".!?,"))
	if len(strings.Fields(normalized)) > 6 {
		return false
	}
	for _, m := range continuationMarkers {
		if normalized == m || strings.HasPrefix(normalized, m+" ") || strings.HasPrefix(normalized, m+",") {
			return true
		}
	}
	return false
}

// pathShapes reduces file arguments to a directory-and-extension shape. The
// shape is what recurs across repositories — "app/components/*.tsx" — while the
// full path is specific to one checkout and would split a workflow into one
// cluster per file.
func pathShapes(args map[string]string) []string {
	if len(args) == 0 {
		return nil
	}
	out := make([]string, 0, 2)
	for _, key := range []string{"file_path", "filePath", "path", "files", "notebook_path", "target_file"} {
		raw, ok := args[key]
		if !ok || raw == "" {
			continue
		}
		for _, item := range strings.Split(raw, ",") {
			item = strings.TrimSpace(item)
			if item == "" || !strings.Contains(item, "/") {
				continue
			}
			dir := path.Dir(item)
			ext := strings.ToLower(path.Ext(item))
			// Keep at most the last two directory segments: deeper prefixes are
			// checkout-specific, shallower ones say nothing.
			segments := strings.Split(strings.Trim(dir, "/"), "/")
			if len(segments) > 2 {
				segments = segments[len(segments)-2:]
			}
			shape := strings.Join(segments, "/")
			if ext != "" {
				shape += "/*" + ext
			}
			out = append(out, shape)
			if len(out) >= 6 {
				return out
			}
		}
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
