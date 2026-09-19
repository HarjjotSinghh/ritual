// Package eval answers the only question that matters about a tool like this:
// does it find the workflows a person would actually have written down?
//
// The benchmark uses the operator's own skills as ground truth. Every installed
// skill has a creation date; for each one, ritual is run over only the sessions
// that predate it and asked what it would have proposed. If the mined report
// surfaces that workflow before the skill existed, the tool would have saved
// the operator the work of noticing it themselves.
//
// This is a harder test than it sounds and a fairer one than a synthetic
// corpus. The skills were written by a human who had the whole context; ritual
// gets only the transcripts, and it has to name the same thing.
package eval

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/ingest"
	"github.com/HarjjotSinghh/ritual/internal/inventory"
	"github.com/HarjjotSinghh/ritual/internal/report"
	"github.com/HarjjotSinghh/ritual/internal/session"
	"github.com/HarjjotSinghh/ritual/internal/textutil"
)

// Truth is one hand-written artifact used as ground truth.
type Truth struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Path        string    `json:"path"`
	CreatedAt   time.Time `json:"created_at"`
	Tokens      []string  `json:"-"`
}

// Outcome is how ritual did on one ground-truth artifact.
type Outcome struct {
	Truth Truth `json:"truth"`
	// Rank is the 1-based position of the best matching finding, or 0 when
	// nothing matched.
	Rank int `json:"rank"`
	// Similarity is how much of the skill's vocabulary the matching finding
	// covered.
	Similarity float64 `json:"similarity"`
	MatchID    string  `json:"match_id,omitempty"`
	MatchTitle string  `json:"match_title,omitempty"`
	// Sessions is how many sessions predated the skill and were available to
	// mine. A miss with three sessions of history is not a miss worth
	// reporting, so this is shown alongside every result.
	Sessions int    `json:"sessions"`
	Note     string `json:"note,omitempty"`
}

// Result is the whole benchmark.
type Result struct {
	Outcomes []Outcome `json:"outcomes"`
	// RecallAt is recall at each cutoff: the share of ground-truth artifacts
	// that appeared in the top N findings.
	RecallAt map[int]float64 `json:"recall_at"`
	// MRR is the mean reciprocal rank over artifacts with enough history.
	MRR float64 `json:"mrr"`
	// Evaluated is how many artifacts had enough history to judge.
	Evaluated int `json:"evaluated"`
	Skipped   int `json:"skipped"`
}

// Options configure a run.
type Options struct {
	// MinSessions is how much history a ground-truth artifact needs before a
	// miss counts against the tool.
	MinSessions int
	// Cutoffs are the recall@N values to compute.
	Cutoffs []int
	// MatchThreshold is the vocabulary containment at which a finding is
	// judged to be the same workflow as the skill.
	MatchThreshold float64
	// Only restricts the benchmark to artifacts whose name contains this text.
	Only string
	// ReportOptions are passed through to each fold.
	ReportOptions report.Options
	// Progress is called before each fold.
	Progress func(name string, index, total int)
}

// DefaultOptions are what `ritual eval` uses.
func DefaultOptions() Options {
	opts := report.DefaultOptions()
	// The benchmark must not see the answer key: with the inventory enabled,
	// every ground-truth workflow would be classified as "you already have
	// this" and the test would measure nothing.
	opts.SkipInventory = true
	return Options{
		MinSessions:    8,
		Cutoffs:        []int{1, 3, 5, 10},
		MatchThreshold: 0.45,
		ReportOptions:  opts,
	}
}

// LoadTruth collects the installed skills and commands that can serve as ground
// truth, dated by when their file was created.
func LoadTruth(projectRoots []string) ([]Truth, error) {
	inv, err := inventory.Scan(projectRoots)
	if err != nil {
		return nil, err
	}
	out := make([]Truth, 0, len(inv.Items))
	for _, item := range inv.Items {
		if item.Kind == inventory.KindRules {
			continue
		}
		path := expandHome(item.Path)
		created, err := createdAt(path)
		if err != nil {
			continue
		}
		out = append(out, Truth{
			Name: item.Name, Description: item.Description, Path: item.Path,
			CreatedAt: created,
			Tokens:    textutil.Dedupe(textutil.FilterMeaningful(item.Tokens)),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

// Run executes the benchmark against a set of already-ingested sessions.
//
// Ingestion happens once and the folds re-mine subsets of it, because reading a
// year of transcripts per ground-truth skill would make the benchmark too slow
// to run often, and a benchmark nobody runs is a benchmark that rots.
func Run(scan *ingest.Result, truths []Truth, opts Options) (*Result, error) {
	if opts.MatchThreshold <= 0 {
		opts = DefaultOptions()
	}
	res := &Result{RecallAt: map[int]float64{}}
	hits := map[int]int{}
	reciprocal := 0.0

	for i, truth := range truths {
		if opts.Only != "" && !strings.Contains(strings.ToLower(truth.Name), strings.ToLower(opts.Only)) {
			continue
		}
		if opts.Progress != nil {
			opts.Progress(truth.Name, i, len(truths))
		}

		before := sessionsBefore(scan.Sessions, truth.CreatedAt)
		outcome := Outcome{Truth: truth, Sessions: len(before)}
		if len(before) < opts.MinSessions {
			outcome.Note = fmt.Sprintf("only %d sessions predate this skill; not enough history to judge", len(before))
			res.Skipped++
			res.Outcomes = append(res.Outcomes, outcome)
			continue
		}

		fold := &ingest.Result{Sessions: before}
		rep, err := report.BuildFrom(fold, opts.ReportOptions)
		if err != nil {
			return nil, err
		}

		rank, sim, id, title := bestMatch(rep, truth, opts.MatchThreshold)
		outcome.Rank, outcome.Similarity, outcome.MatchID, outcome.MatchTitle = rank, sim, id, title
		if rank == 0 {
			outcome.Note = "no finding covered this workflow"
		}
		res.Outcomes = append(res.Outcomes, outcome)
		res.Evaluated++

		if rank > 0 {
			reciprocal += 1 / float64(rank)
			for _, cutoff := range opts.Cutoffs {
				if rank <= cutoff {
					hits[cutoff]++
				}
			}
		}
	}

	if res.Evaluated > 0 {
		res.MRR = round3(reciprocal / float64(res.Evaluated))
		for _, cutoff := range opts.Cutoffs {
			res.RecallAt[cutoff] = round3(float64(hits[cutoff]) / float64(res.Evaluated))
		}
	}
	return res, nil
}

// bestMatch finds the highest-ranked finding whose vocabulary covers the
// ground-truth artifact.
func bestMatch(rep *report.Report, truth Truth, threshold float64) (rank int, similarity float64, id, title string) {
	want := textutil.NewSet(truth.Tokens...)
	if len(want) == 0 {
		return 0, 0, "", ""
	}
	for i, f := range rep.Findings {
		have := textutil.NewSet(findingVocabulary(f)...)
		if len(have) == 0 {
			continue
		}
		overlap := 0
		for t := range have {
			if _, ok := want[t]; ok {
				overlap++
			}
		}
		// Containment measured against the finding, not the skill: a skill's
		// description can be long, and a finding that covers its core terms is
		// the same workflow even if it says less.
		sim := float64(overlap) / float64(len(have))
		if sim >= threshold {
			return i + 1, round3(sim), f.ID, f.Title
		}
	}
	return 0, 0, "", ""
}

func findingVocabulary(f report.Finding) []string {
	tokens := textutil.Tokens(f.Title)
	for _, p := range f.Phrases {
		tokens = append(tokens, textutil.Tokens(p.Phrase)...)
	}
	tokens = append(tokens, f.Keywords...)
	return textutil.Dedupe(textutil.FilterMeaningful(tokens))
}

func sessionsBefore(sessions []session.Session, cutoff time.Time) []session.Session {
	out := make([]session.Session, 0, len(sessions))
	for _, s := range sessions {
		if s.Start.IsZero() || s.Start.Before(cutoff) {
			out = append(out, s)
		}
	}
	return out
}

// createdAt returns when a skill first appeared.
//
// Git is asked first, because modification time lies in exactly the case that
// matters: a restored backup, a synced dotfiles repo, or a new machine gives
// every skill today's date, which would make the benchmark claim there was no
// history to find the workflow in. When the file is tracked, the date of the
// commit that added it is the real answer.
func createdAt(path string) (time.Time, error) {
	if t, ok := gitAddedAt(path); ok {
		return t, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime().UTC(), nil
}

// gitAddedAt returns the author date of the commit that added a file.
func gitAddedAt(path string) (time.Time, bool) {
	dir := filepath.Dir(path)
	cmd := exec.Command("git", "-C", dir, "log", "--diff-filter=A", "--follow", "--format=%aI", "--", filepath.Base(path))
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}, false
	}
	lines := strings.Fields(strings.TrimSpace(string(out)))
	if len(lines) == 0 {
		return time.Time{}, false
	}
	// --follow prints newest first; the last line is the original addition.
	t, err := time.Parse(time.RFC3339, lines[len(lines)-1])
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

func expandHome(p string) string {
	if !strings.HasPrefix(p, "${HOME}") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "${HOME}"))
}

func round3(v float64) float64 { return float64(int(v*1000+0.5)) / 1000 }
