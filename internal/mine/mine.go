package mine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/session"
	"github.com/HarjjotSinghh/ritual/internal/textutil"
)

// Options configure the whole mining pipeline.
type Options struct {
	Segment SegmentOptions
	Cluster ClusterOptions
	// MinStepSupport is the share of a cluster's arcs that must perform an
	// action for it to enter the canonical sequence.
	MinStepSupport float64
	// MinRuleOccurrences is how many times a standing preference must be
	// repeated before it is proposed as a rule.
	MinRuleOccurrences int
	// MaxEvidence caps the citations kept per candidate.
	MaxEvidence int
	// MinSessions is how many distinct sessions a workflow must span. Four
	// runs inside one session is a loop, not a habit, and suggesting a skill
	// for it would mistake one bad afternoon for a practice.
	MinSessions int
}

// DefaultOptions are what `ritual scan` uses.
func DefaultOptions() Options {
	return Options{
		Segment:            DefaultSegmentOptions(),
		Cluster:            DefaultClusterOptions(),
		MinStepSupport:     0.45,
		MinRuleOccurrences: 2,
		MaxEvidence:        8,
		MinSessions:        2,
	}
}

// Output is everything the miner found.
type Output struct {
	Candidates []Candidate     `json:"candidates"`
	Rules      []RuleCandidate `json:"rules"`
	Arcs       int             `json:"arcs"`
	Clustered  int             `json:"clustered"`
}

// Run executes the full pipeline: segment, cluster, canonicalize, measure.
func Run(sessions []session.Session, opts Options) Output {
	if opts.MinStepSupport <= 0 {
		opts = DefaultOptions()
	}
	arcs := Segment(sessions, opts.Segment)
	clusters := ClusterArcs(arcs, opts.Cluster)

	// Distinctiveness is measured against every arc, not only the clustered
	// ones, so a term that is common across the whole history ("fix", "file")
	// cannot become a candidate's headline.
	globalDF := documentFrequency(arcs)
	phrases := BuildPhraseIndex(arcs)

	out := Output{Arcs: len(arcs), Rules: GroupCorrections(arcs, opts.MinRuleOccurrences)}
	for _, c := range clusters {
		cand := buildCandidate(c, globalDF, phrases, len(arcs), opts)
		if cand.Sessions < opts.MinSessions {
			continue
		}
		out.Clustered += len(c.Arcs)
		out.Candidates = append(out.Candidates, cand)
	}
	sort.SliceStable(out.Candidates, func(i, j int) bool {
		if out.Candidates[i].Occurrences != out.Candidates[j].Occurrences {
			return out.Candidates[i].Occurrences > out.Candidates[j].Occurrences
		}
		return out.Candidates[i].ID < out.Candidates[j].ID
	})
	return out
}

func buildCandidate(c Cluster, globalDF map[string]int, phrases *PhraseIndex, corpus int, opts Options) Candidate {
	cand := Candidate{
		Occurrences: len(c.Arcs),
		Steps:       CanonicalSteps(c.Arcs, opts.MinStepSupport),
		Cadence:     MeasureCadence(c.Arcs),
		Cohesion:    round3(c.Cohesion(opts.Cluster.IntentWeight)),
	}

	sessions := map[string]struct{}{}
	agents := map[string]struct{}{}
	repos := map[string]struct{}{}
	commands := map[string]int{}
	tools := map[string]int{}
	paths := map[string]int{}
	turns := make([]float64, 0, len(c.Arcs))
	durations := make([]float64, 0, len(c.Arcs))
	errored := 0

	for _, a := range c.Arcs {
		sessions[a.Agent+"/"+a.SessionID] = struct{}{}
		agents[a.Agent] = struct{}{}
		if a.Repo != "" {
			repos[a.Repo] = struct{}{}
		}
		for _, cmd := range a.Commands {
			commands[cmd]++
		}
		for _, t := range a.Tools {
			tools[t]++
		}
		for _, p := range a.Paths {
			paths[p]++
		}
		turns = append(turns, float64(a.Turns))
		if d := a.Duration(); d > 0 {
			durations = append(durations, d.Minutes())
		}
		if a.Errors > 0 {
			errored++
		}
		cand.Corrections = append(cand.Corrections, a.Corrections...)
	}

	cand.Sessions = len(sessions)
	cand.Agents = keysOf(agents)
	cand.Repos = keysOf(repos)
	cand.Commands = textutil.TopN(commands, 10)
	cand.Tools = textutil.TopN(tools, 8)
	cand.Paths = textutil.TopN(paths, 6)
	cand.MedianTurns = int(median(turns) + 0.5)
	cand.MedianDurationMinutes = round3(median(durations))
	if len(c.Arcs) > 0 {
		cand.ErrorRate = round3(float64(errored) / float64(len(c.Arcs)))
	}
	if len(cand.Corrections) > 6 {
		cand.Corrections = cand.Corrections[:6]
	}

	cand.Keywords = distinctiveKeywords(c.Arcs, globalDF, corpus, 8)
	cand.Phrases = phrases.Top(c.Arcs, 3)
	cand.Title = titleFor(cand, c.Arcs)
	cand.Slug = textutil.Slug(cand.Title)
	cand.Summary = summarize(cand)
	cand.ID = textutil.Fingerprint("candidate", cand.Slug, strings.Join(stepActions(cand.Steps), "|"))
	cand.Evidence = evidenceFor(c.Arcs, opts.MaxEvidence)
	return cand
}

// documentFrequency counts how many arcs each token appears in, which is what
// makes a term distinctive to a cluster rather than common to the operator.
func documentFrequency(arcs []Arc) map[string]int {
	df := make(map[string]int, 2048)
	for _, a := range arcs {
		for _, t := range textutil.Dedupe(intentTokens(a)) {
			df[t]++
		}
	}
	return df
}

// distinctiveKeywords ranks a cluster's vocabulary by how much more often a
// term appears inside the cluster than across the whole history.
func distinctiveKeywords(arcs []Arc, globalDF map[string]int, corpus, n int) []string {
	local := make(map[string]int, 256)
	for _, a := range arcs {
		for _, t := range textutil.Dedupe(intentTokens(a)) {
			local[t]++
		}
	}
	if corpus <= 0 {
		corpus = 1
	}
	type scored struct {
		token string
		score float64
	}
	ranked := make([]scored, 0, len(local))
	for token, count := range local {
		if !textutil.Meaningful(token) {
			continue
		}
		// A term that appears in one arc of a ten-arc cluster is a detail of
		// that run, not a property of the workflow.
		if float64(count)/float64(len(arcs)) < 0.34 {
			continue
		}
		global := globalDF[token]
		if global == 0 {
			global = 1
		}
		lift := (float64(count) / float64(len(arcs))) / (float64(global) / float64(corpus))
		ranked = append(ranked, scored{token, lift})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].token < ranked[j].token
	})
	out := make([]string, 0, n)
	for _, r := range ranked {
		out = append(out, r.token)
		if len(out) >= n {
			break
		}
	}
	return out
}

// actionVerbs map a dominant tool verb onto the phrase a human would use for
// the workflow it anchors.
var actionVerbs = map[string]string{
	"browser":    "verification",
	"edit":       "changes",
	"write":      "authoring",
	"search":     "investigation",
	"read":       "review",
	"web_search": "research",
	"web_fetch":  "research",
	"subagent":   "parallel work",
	"plan":       "planning",
}

// workVerbs are the imperatives people actually open a prompt with. When most
// of a cluster's prompts start with the same one, it is the truest one-word
// description of the workflow available, and far better than whatever noun the
// keyword ranking happened to surface.
var workVerbs = map[string]string{
	"review": "Review", "fix": "Fix", "update": "Update", "add": "Add", "remove": "Remove",
	"refactor": "Refactor", "deploy": "Deploy", "verify": "Verify", "test": "Test",
	"check": "Check", "draft": "Draft", "write": "Write", "create": "Create",
	"build": "Build", "debug": "Debug", "investigate": "Investigate", "migrate": "Migrate",
	"rename": "Rename", "optimize": "Optimize", "audit": "Audit", "triage": "Triage",
	"summarize": "Summarize", "reply": "Reply to", "send": "Send", "publish": "Publish",
	"release": "Release", "merge": "Merge", "revert": "Revert", "install": "Install",
	"configure": "Configure", "document": "Document", "design": "Design",
	"implement": "Implement", "analyze": "Analyze", "generate": "Generate",
	"clean": "Clean up", "sync": "Sync", "ship": "Ship", "polish": "Polish",
	"rewrite": "Rewrite", "convert": "Convert", "port": "Port", "wire": "Wire up",
	"set": "Set up", "make": "Make", "run": "Run", "finish": "Finish",
}

// dominantVerb returns the imperative most of the cluster's prompts open with,
// or "" when the prompts do not agree. Requiring agreement matters: a verb
// taken from one prompt would name the workflow after a single day's phrasing.
func dominantVerb(arcs []Arc) string {
	counts := make(map[string]int, 8)
	considered := 0
	for _, a := range arcs {
		fields := strings.Fields(strings.ToLower(textutil.FirstLine(a.Intent)))
		if len(fields) == 0 {
			continue
		}
		considered++
		// Look past a leading politeness or subject ("can you review…",
		// "please fix…", "i need to deploy…").
		for i := 0; i < len(fields) && i < 4; i++ {
			word := strings.Trim(fields[i], ",.:;!?`\"'")
			if _, ok := workVerbs[word]; ok {
				counts[word]++
				break
			}
		}
	}
	if considered == 0 {
		return ""
	}
	top := textutil.TopN(counts, 1)
	if len(top) == 0 {
		return ""
	}
	if float64(counts[top[0]])/float64(considered) < 0.4 {
		return ""
	}
	return top[0]
}

// domainHints name the system a workflow operates on, recovered from the tools
// and commands it uses. When the prompts share no usable phrase — which is
// normal for someone who writes a different paragraph every time — this is what
// makes a title recognizable.
var domainHints = []struct {
	match  string
	domain string
}{
	{"mcp:slack", "Slack"},
	{"mcp:notion", "Notion"},
	{"mcp:linear", "Linear"},
	{"mcp:github", "GitHub"},
	{"mcp:figma", "Figma"},
	{"mcp:posthog", "PostHog"},
	{"mcp:sentry", "Sentry"},
	{"mcp:stripe", "Stripe"},
	{"mcp:vercel", "Vercel"},
	{"mcp:shopify", "Shopify"},
	{"browser", "browser"},
	{"shell:gh pr", "pull request"},
	{"shell:git", "git"},
	{"shell:npm run test", "test"},
	{"shell:npm run build", "build"},
	{"shell:vercel", "Vercel"},
	{"shell:shopify", "Shopify"},
	{"shell:docker", "Docker"},
	{"shell:kubectl", "Kubernetes"},
	{"shell:terraform", "Terraform"},
}

// domainHint returns the systems a candidate touches, most prominent first.
func domainHint(c Candidate) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 2)
	consider := func(values []string) {
		for _, v := range values {
			for _, h := range domainHints {
				if !strings.HasPrefix(v, h.match) && !strings.HasPrefix("shell:"+v, h.match) {
					continue
				}
				if _, ok := seen[h.domain]; ok {
					continue
				}
				seen[h.domain] = struct{}{}
				out = append(out, h.domain)
			}
		}
	}
	consider(c.Tools)
	consider(stepActions(c.Steps))
	consider(c.Commands)
	return out
}

// titleFor builds a short, human-recognizable name for the workflow.
//
// The order of preference is deliberate. A phrase the prompts share is the
// operator's own name for the work and beats anything inferred. Failing that,
// the systems the workflow touches plus its dominant verb describe it honestly
// — "Triage Slack alerts" is a title someone can recognize even if no two of
// their prompts were worded alike. Only when both fail does it fall back to
// ranked keywords, which is where the unreadable titles come from and why it is
// last.
func titleFor(c Candidate, arcs []Arc) string {
	verb := dominantVerb(arcs)
	domains := domainHint(c)

	subject := ""
	if len(c.Phrases) > 0 && c.Phrases[0].Score >= StrongPhrase {
		subject = c.Phrases[0].Phrase
		if len(c.Phrases) > 1 && len(strings.Fields(subject)) < 3 && c.Phrases[1].Score >= StrongPhrase {
			subject += " " + c.Phrases[1].Phrase
		}
	}
	if subject == "" && len(domains) > 0 {
		subject = strings.Join(domains[:min(2, len(domains))], " and ")
		// A domain alone is a noun; pair it with whatever the workflow does to
		// it, so two different Slack workflows do not both become "Slack work".
		if object := integrationObject(c, domains); object != "" {
			subject += " " + object
		} else if kw := firstMeaningfulKeyword(c, domains); kw != "" {
			subject += " " + kw
		}
	}
	if subject == "" {
		words := make([]string, 0, 3)
		for _, k := range c.Keywords {
			words = append(words, k)
			if len(words) == 3 {
				break
			}
		}
		subject = strings.Join(words, " ")
	}
	if subject == "" {
		subject = primarySubject(c)
	}

	switch {
	case verb != "" && subject != "":
		return workVerbs[verb] + " " + subject
	case subject != "":
		title := capitalize(subject)
		if len(c.Tools) > 0 {
			if suffix, ok := actionVerbs[c.Tools[0]]; ok && !strings.Contains(title, suffix) {
				title += " " + suffix
			}
		}
		return title
	case len(c.Commands) > 0:
		return "Run " + c.Commands[0]
	case len(arcs) > 0:
		if line := textutil.FirstLine(arcs[0].Intent); line != "" {
			return textutil.Truncate(line, 60)
		}
	}
	return "Recurring workflow"
}

// integrationObject reads the nouns out of the integration calls a workflow
// makes. `mcp:slack-search-channels` and `mcp:slack-read-channel` agree on
// "channels", which is a far better name for the work than whichever adjective
// the prompts happened to share.
func integrationObject(c Candidate, domains []string) string {
	skip := map[string]struct{}{
		"get": {}, "list": {}, "read": {}, "search": {}, "send": {}, "create": {},
		"update": {}, "fetch": {}, "query": {}, "add": {}, "delete": {}, "mcp": {},
		"public": {}, "private": {}, "data": {}, "sources": {}, "record": {},
	}
	for _, d := range domains {
		skip[strings.ToLower(d)] = struct{}{}
	}
	counts := make(map[string]int, 8)
	for _, action := range append(append([]string{}, c.Tools...), stepActions(c.Steps)...) {
		rest, ok := strings.CutPrefix(action, "mcp:")
		if !ok {
			continue
		}
		for _, word := range strings.Split(strings.ReplaceAll(rest, ":", "-"), "-") {
			word = strings.TrimSuffix(strings.ToLower(word), "s")
			if word == "" || len(word) < 3 {
				continue
			}
			if _, bad := skip[word]; bad {
				continue
			}
			counts[word]++
		}
	}
	top := textutil.TopN(counts, 1)
	if len(top) == 0 {
		return ""
	}
	return top[0] + "s"
}

// firstMeaningfulKeyword returns the top keyword that is not already implied by
// the domain names.
func firstMeaningfulKeyword(c Candidate, domains []string) string {
	lowered := make(map[string]struct{}, len(domains))
	for _, d := range domains {
		lowered[strings.ToLower(d)] = struct{}{}
	}
	for _, k := range c.Keywords {
		if _, ok := lowered[k]; ok {
			continue
		}
		return k
	}
	return ""
}

// primarySubject names what a workflow acts on when nothing else is available:
// the repository, or the most-used command.
func primarySubject(c Candidate) string {
	if len(c.Repos) == 1 {
		return c.Repos[0]
	}
	if len(c.Commands) > 0 {
		return "`" + c.Commands[0] + "`"
	}
	return ""
}

func summarize(c Candidate) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Ran %d times across %d sessions", c.Occurrences, c.Sessions)
	if len(c.Repos) == 1 {
		fmt.Fprintf(&b, " in %s", c.Repos[0])
	} else if len(c.Repos) > 1 {
		fmt.Fprintf(&b, " in %d repositories", len(c.Repos))
	}
	if len(c.Agents) > 1 {
		fmt.Fprintf(&b, " and %d agents", len(c.Agents))
	}
	if c.Cadence.Label != "" && c.Cadence.Label != "once" {
		fmt.Fprintf(&b, ", %s", c.Cadence.Label)
	}
	b.WriteString(".")
	if len(c.Steps) > 0 {
		shown := c.Steps
		if len(shown) > 4 {
			shown = shown[:4]
		}
		parts := make([]string, 0, len(shown))
		for _, s := range shown {
			parts = append(parts, shortStep(s.Action))
		}
		fmt.Fprintf(&b, " Usually: %s.", strings.Join(parts, " → "))
	}
	return b.String()
}

func shortStep(action string) string {
	if cmd, ok := strings.CutPrefix(action, "shell:"); ok {
		return cmd
	}
	if rest, ok := strings.CutPrefix(action, "mcp:"); ok {
		return strings.SplitN(rest, ":", 2)[0]
	}
	return strings.TrimPrefix(action, "tool:")
}

func stepActions(steps []Step) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.Action)
	}
	return out
}

func evidenceFor(arcs []Arc, max int) []Evidence {
	if max <= 0 {
		max = 8
	}
	// Citations are spread across the cluster's lifetime rather than taken from
	// the front, so the evidence shows recurrence instead of one busy week.
	picked := arcs
	if len(arcs) > max {
		picked = make([]Arc, 0, max)
		stride := float64(len(arcs)-1) / float64(max-1)
		for i := 0; i < max; i++ {
			picked = append(picked, arcs[int(float64(i)*stride+0.5)])
		}
	}
	out := make([]Evidence, 0, len(picked))
	for _, a := range picked {
		out = append(out, Evidence{
			ArcID: a.ID, Agent: a.Agent, SessionID: a.SessionID, Source: a.Source,
			Repo: a.Repo, At: a.Start, Steps: len(a.Steps),
			Intent: textutil.Truncate(textutil.FirstLine(a.Intent), 140),
		})
	}
	return out
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
