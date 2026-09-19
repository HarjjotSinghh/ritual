package mine

import (
	"regexp"
	"sort"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/session"
	"github.com/HarjjotSinghh/ritual/internal/textutil"
)

// A correction is the highest-value signal in a transcript. When a human says
// "no, always run the mobile check too", they are not describing a task — they
// are stating a rule the agent did not know, and will not know next time
// either, because nothing in the session survives it. Every one of those is a
// line that belongs in a rules file.
//
// Detection is pattern-based and conservative. A false positive writes a rule
// nobody asked for into an agent's permanent instructions, which is a worse
// outcome than missing one, so the patterns require an explicit corrective or
// standing-instruction marker rather than inferring intent from tone.

var (
	// standingMarkers state a rule meant to outlive the task.
	standingMarkers = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\balways\b`),
		regexp.MustCompile(`(?i)\bnever\b`),
		regexp.MustCompile(`(?i)\bfrom now on\b`),
		regexp.MustCompile(`(?i)\bgoing forward\b`),
		regexp.MustCompile(`(?i)\bevery time\b`),
		regexp.MustCompile(`(?i)\beach time\b`),
		regexp.MustCompile(`(?i)\bby default\b`),
		regexp.MustCompile(`(?i)\bremember (?:to|that)\b`),
		regexp.MustCompile(`(?i)\bdon'?t forget\b`),
		regexp.MustCompile(`(?i)\bmake sure (?:you|to)\b`),
		regexp.MustCompile(`(?i)\bin future\b`),
		regexp.MustCompile(`(?i)\bstop (?:doing|adding|using)\b`),
	}

	// fixMarkers redirect the work in progress.
	fixMarkers = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^(?:no|nope|nah)\b`),
		regexp.MustCompile(`(?i)^(?:actually|instead|wait)\b`),
		regexp.MustCompile(`(?i)\bthat'?s (?:not|wrong|incorrect)\b`),
		regexp.MustCompile(`(?i)\bdon'?t (?:do|use|change|touch|edit|remove|add)\b`),
		regexp.MustCompile(`(?i)\byou (?:forgot|missed|broke|removed|changed)\b`),
		regexp.MustCompile(`(?i)\bwhy did you\b`),
		regexp.MustCompile(`(?i)\b(?:revert|undo) (?:that|this|it)\b`),
		regexp.MustCompile(`(?i)\bnot what i (?:asked|wanted|meant)\b`),
		regexp.MustCompile(`(?i)\buse .{2,40} instead\b`),
		regexp.MustCompile(`(?i)\bi (?:said|told you)\b`),
	}
)

// classifyCorrection decides whether a human turn is a correction, and of which
// kind. trigger records the action the agent had just taken, which is what the
// correction was aimed at.
func classifyCorrection(text, trigger string, t session.Turn, s session.Session) (Correction, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || len(trimmed) > 600 {
		return Correction{}, false
	}
	c := Correction{
		Text: textutil.Truncate(trimmed, 280), Trigger: trigger,
		At: t.At, SessionID: s.ID, Agent: s.Agent, Repo: s.Repo,
	}
	for _, re := range standingMarkers {
		if re.MatchString(trimmed) {
			c.Kind = CorrectionPreference
			return c, true
		}
	}
	for _, re := range fixMarkers {
		if re.MatchString(trimmed) {
			c.Kind = CorrectionFix
			return c, true
		}
	}
	return Correction{}, false
}

// RuleCandidate is a standing preference the human repeated across sessions.
type RuleCandidate struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Theme string `json:"theme"`

	Occurrences int          `json:"occurrences"`
	Sessions    int          `json:"sessions"`
	Agents      []string     `json:"agents"`
	Repos       []string     `json:"repos"`
	Keywords    []string     `json:"keywords"`
	Examples    []Correction `json:"examples"`
}

// GroupCorrections clusters standing preferences by shared vocabulary so
// "always test mobile too", "also check mobile", and "don't forget the mobile
// layout" become one rule with three citations instead of three rules.
//
// A candidate has to clear two bars. minOccurrences guards against a
// preference stated once, which may have been true only of that task. Two
// distinct sessions are then required on top of it, because restating
// something twice inside one conversation is the operator correcting a single
// misunderstanding, not describing how they always work.
func GroupCorrections(arcs []Arc, minOccurrences int) []RuleCandidate {
	type group struct {
		tokens   textutil.Set
		members  []Correction
		sessions map[string]struct{}
		agents   map[string]struct{}
		repos    map[string]struct{}
	}

	all := make([]Correction, 0, 64)
	for _, a := range arcs {
		for _, c := range a.Corrections {
			if c.Kind != CorrectionPreference {
				continue
			}
			all = append(all, c)
		}
	}
	// Stable input order keeps the greedy grouping reproducible.
	sort.SliceStable(all, func(i, j int) bool {
		if !all[i].At.Equal(all[j].At) {
			return all[i].At.Before(all[j].At)
		}
		return all[i].Text < all[j].Text
	})

	groups := make([]*group, 0, 16)
	for _, c := range all {
		tokens := textutil.NewSet(contentTokens(c.Text)...)
		if len(tokens) < 2 {
			continue
		}
		var best *group
		bestSim := 0.0
		for _, g := range groups {
			if sim := textutil.Jaccard(tokens, g.tokens); sim > bestSim {
				best, bestSim = g, sim
			}
		}
		if best == nil || bestSim < 0.34 {
			best = &group{
				tokens:   textutil.NewSet(tokens.Sorted()...),
				sessions: map[string]struct{}{},
				agents:   map[string]struct{}{},
				repos:    map[string]struct{}{},
			}
			groups = append(groups, best)
		} else {
			best.tokens.Add(tokens.Sorted()...)
		}
		best.members = append(best.members, c)
		best.sessions[c.SessionID] = struct{}{}
		best.agents[c.Agent] = struct{}{}
		if c.Repo != "" {
			best.repos[c.Repo] = struct{}{}
		}
	}

	out := make([]RuleCandidate, 0, len(groups))
	for _, g := range groups {
		if len(g.members) < minOccurrences || len(g.sessions) < 2 {
			continue
		}
		rc := RuleCandidate{
			Occurrences: len(g.members),
			Sessions:    len(g.sessions),
			Agents:      keysOf(g.agents),
			Repos:       keysOf(g.repos),
			Keywords:    topTokens(g.members),
			Examples:    limitCorrections(g.members, 5),
		}
		rc.Text = representative(g.members)
		rc.Theme = strings.Join(rc.Keywords[:min(3, len(rc.Keywords))], " ")
		rc.ID = textutil.Fingerprint("rule", strings.Join(rc.Keywords, " "))
		out = append(out, rc)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Occurrences != out[j].Occurrences {
			return out[i].Occurrences > out[j].Occurrences
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// contentTokens drops the corrective marker itself before comparing, so two
// preferences are grouped by what they ask for rather than by both starting
// with "always".
func contentTokens(text string) []string {
	tokens := textutil.Tokens(text)
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		switch t {
		case "always", "never", "forget", "remember", "sure", "future", "default", "time", "every", "each", "going", "forward", "stop":
			continue
		}
		out = append(out, t)
	}
	return out
}

// representative picks the clearest phrasing in a group: the shortest text that
// still carries most of the group's shared vocabulary. A long example usually
// mixes the rule with task detail that does not generalize.
func representative(members []Correction) string {
	if len(members) == 0 {
		return ""
	}
	shared := map[string]int{}
	for _, m := range members {
		for _, t := range textutil.Dedupe(contentTokens(m.Text)) {
			shared[t]++
		}
	}
	best := members[0]
	bestScore := -1.0
	for _, m := range members {
		tokens := textutil.Dedupe(contentTokens(m.Text))
		if len(tokens) == 0 {
			continue
		}
		covered := 0
		for _, t := range tokens {
			if shared[t] > 1 {
				covered++
			}
		}
		// Reward coverage, penalize length: the ideal is the short sentence
		// that says the shared thing.
		score := float64(covered) - float64(len(tokens))*0.15
		if score > bestScore {
			best, bestScore = m, score
		}
	}
	return best.Text
}

func topTokens(members []Correction) []string {
	counts := map[string]int{}
	for _, m := range members {
		for _, t := range textutil.Dedupe(contentTokens(m.Text)) {
			counts[t]++
		}
	}
	return textutil.TopN(counts, 8)
}

func limitCorrections(in []Correction, n int) []Correction {
	if len(in) <= n {
		out := make([]Correction, len(in))
		copy(out, in)
		return out
	}
	out := make([]Correction, 0, n)
	out = append(out, in[:n]...)
	return out
}

func keysOf(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		if k == "" {
			continue
		}
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
