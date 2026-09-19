// Package report assembles a full run — ingest, mine, score, classify — into
// one serializable document.
//
// The report is the product. Everything else (the terminal output, the local
// dashboard, the generated artifacts, the eval harness) reads this structure,
// so a change in presentation can never change a finding.
package report

import (
	"sort"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/classify"
	"github.com/HarjjotSinghh/ritual/internal/ingest"
	"github.com/HarjjotSinghh/ritual/internal/inventory"
	"github.com/HarjjotSinghh/ritual/internal/mine"
	"github.com/HarjjotSinghh/ritual/internal/score"
	"github.com/HarjjotSinghh/ritual/internal/session"
	"github.com/HarjjotSinghh/ritual/internal/version"
)

// Finding is one mined workflow with its score and classification.
type Finding struct {
	mine.Candidate
	Score    score.Result        `json:"score"`
	Decision classify.Decision   `json:"decision"`
	Rule     *mine.RuleCandidate `json:"rule,omitempty"`
}

// RuleFinding is a standing preference with its classification.
type RuleFinding struct {
	mine.RuleCandidate
	Decision classify.Decision `json:"decision"`
}

// Report is one complete run.
type Report struct {
	Version     string    `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`
	// Window describes what was scanned, so a report read weeks later is not
	// mistaken for a current one.
	Window Window `json:"window"`

	Stats     []session.Stats   `json:"stats"`
	Roots     map[string]string `json:"roots,omitempty"`
	Warnings  []string          `json:"warnings,omitempty"`
	Inventory []inventory.Item  `json:"inventory,omitempty"`

	Arcs      int `json:"arcs"`
	Clustered int `json:"clustered"`

	Findings []Finding     `json:"findings"`
	Rules    []RuleFinding `json:"rules"`
}

// Window is the scanned range.
type Window struct {
	Since    time.Time `json:"since,omitempty"`
	Earliest time.Time `json:"earliest,omitempty"`
	Latest   time.Time `json:"latest,omitempty"`
	Sessions int       `json:"sessions"`
	Agents   int       `json:"agents"`
}

// Options control a full run.
type Options struct {
	Ingest  ingest.Options
	Mine    mine.Options
	Weights score.Weights
	// MinScore is the total below which a candidate is classified as ignore.
	MinScore float64
	// SkipInventory disables the installed-artifact scan. It exists for the
	// eval harness, which has to judge suggestions as if nothing were
	// installed yet.
	SkipInventory bool
}

// DefaultOptions are what the CLI uses.
func DefaultOptions() Options {
	return Options{
		Ingest:   ingest.Options{Limits: ingest.DefaultLimits()},
		Mine:     mine.DefaultOptions(),
		Weights:  score.DefaultWeights(),
		MinScore: 28,
	}
}

// Build runs the whole pipeline.
func Build(opts Options) (*Report, error) {
	scan, err := ingest.Scan(opts.Ingest)
	if err != nil {
		return nil, err
	}
	return BuildFrom(scan, opts)
}

// BuildFrom runs everything after ingestion. The eval harness calls it directly
// with a filtered session set so a scan is not repeated per fold.
func BuildFrom(scan *ingest.Result, opts Options) (*Report, error) {
	mined := mine.Run(scan.Sessions, opts.Mine)

	var inv *inventory.Inventory
	if !opts.SkipInventory {
		roots := make([]string, 0, 8)
		for _, s := range scan.Sessions {
			if s.Workspace != "" {
				roots = append(roots, expandHome(s.Workspace))
			}
		}
		if scanned, invErr := inventory.Scan(roots); invErr == nil {
			inv = scanned
		}
	}

	rep := &Report{
		Version:     version.Version,
		GeneratedAt: time.Now().UTC(),
		Stats:       scan.Stats,
		Roots:       scan.Roots,
		Warnings:    scan.Warnings,
		Arcs:        mined.Arcs,
		Clustered:   mined.Clustered,
	}
	rep.Window = windowOf(scan, opts.Ingest.Since)
	if inv != nil {
		rep.Inventory = inv.Items
	}

	for _, c := range mined.Candidates {
		s := score.Score(c, opts.Weights)
		rep.Findings = append(rep.Findings, Finding{
			Candidate: c,
			Score:     s,
			Decision:  classify.Classify(c, s, inv, opts.MinScore),
		})
	}
	sort.SliceStable(rep.Findings, func(i, j int) bool {
		if rep.Findings[i].Score.Total != rep.Findings[j].Score.Total {
			return rep.Findings[i].Score.Total > rep.Findings[j].Score.Total
		}
		return rep.Findings[i].ID < rep.Findings[j].ID
	})

	for _, r := range mined.Rules {
		rep.Rules = append(rep.Rules, RuleFinding{
			RuleCandidate: r,
			Decision:      classify.ClassifyRule(r, inv),
		})
	}
	return rep, nil
}

// Actionable returns the findings that propose making something, in rank order.
func (r *Report) Actionable() []Finding {
	out := make([]Finding, 0, len(r.Findings))
	for _, f := range r.Findings {
		if f.Decision.Kind == classify.KindIgnore {
			continue
		}
		out = append(out, f)
	}
	return out
}

// Find returns a finding by id or slug prefix. Operators type the short id from
// the terminal, so a prefix match on either is accepted.
func (r *Report) Find(ref string) (Finding, bool) {
	for _, f := range r.Findings {
		if f.ID == ref || f.Slug == ref {
			return f, true
		}
	}
	for _, f := range r.Findings {
		if len(ref) >= 3 && (hasPrefix(f.ID, ref) || hasPrefix(f.Slug, ref)) {
			return f, true
		}
	}
	return Finding{}, false
}

// FindRule returns a rule finding by id prefix.
func (r *Report) FindRule(ref string) (RuleFinding, bool) {
	for _, rule := range r.Rules {
		if rule.ID == ref {
			return rule, true
		}
	}
	for _, rule := range r.Rules {
		if len(ref) >= 3 && hasPrefix(rule.ID, ref) {
			return rule, true
		}
	}
	return RuleFinding{}, false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func windowOf(scan *ingest.Result, since time.Time) Window {
	w := Window{Since: since, Sessions: len(scan.Sessions), Agents: len(scan.Stats)}
	for _, s := range scan.Stats {
		if !s.Earliest.IsZero() && (w.Earliest.IsZero() || s.Earliest.Before(w.Earliest)) {
			w.Earliest = s.Earliest
		}
		if s.Latest.After(w.Latest) {
			w.Latest = s.Latest
		}
	}
	return w
}
