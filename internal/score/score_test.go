package score

import (
	"testing"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/mine"
)

func candidate(mutate func(*mine.Candidate)) mine.Candidate {
	c := mine.Candidate{
		Occurrences: 8, Sessions: 8,
		Agents: []string{"claude"}, Repos: []string{"storefront"},
		Steps: []mine.Step{
			{Action: "shell:shopify theme push", Support: 1, Count: 8},
			{Action: "browser", Support: 1, Count: 8},
			{Action: "shell:npm run test", Support: 0.8, Count: 6},
			{Action: "edit", Support: 0.7, Count: 5},
		},
		Cadence:               mine.Cadence{Days: 8, Regularity: 0.8, Label: "about daily", FirstSeen: time.Now().AddDate(0, 0, -10)},
		Cohesion:              0.7,
		MedianTurns:           18,
		MedianDurationMinutes: 12,
		Phrases:               []mine.ScoredPhrase{{Phrase: "theme push verification", Score: 3}},
	}
	if mutate != nil {
		mutate(&c)
	}
	return c
}

func TestRealWorkflowOutscoresTrivialOne(t *testing.T) {
	real := Score(candidate(nil), DefaultWeights())

	trivial := Score(candidate(func(c *mine.Candidate) {
		c.Steps = []mine.Step{{Action: "shell:git status", Support: 1, Count: 40}}
		c.Occurrences = 40
		c.MedianTurns = 2
		c.MedianDurationMinutes = 0.5
		c.Phrases = nil
	}), DefaultWeights())

	if trivial.Total >= real.Total {
		t.Fatalf("a `git status` loop scored %.1f against a real workflow's %.1f", trivial.Total, real.Total)
	}
}

func TestSingleDayBurstIsPenalized(t *testing.T) {
	burst := candidate(func(c *mine.Candidate) {
		c.Cadence = mine.Cadence{Days: 1, Label: "once"}
		c.Sessions = 1
		c.Occurrences = 6
	})
	res := Score(burst, DefaultWeights())

	found := false
	for _, p := range res.Penalties {
		if p.Name == "single-day" {
			found = true
		}
	}
	if !found {
		t.Fatalf("a one-day burst was not penalized: %+v", res.Penalties)
	}
	if res.Total >= res.Raw {
		t.Fatalf("penalty did not reduce the total: raw %.1f, total %.1f", res.Raw, res.Total)
	}
}

func TestLooseClusterIsPenalized(t *testing.T) {
	res := Score(candidate(func(c *mine.Candidate) { c.Cohesion = 0.2 }), DefaultWeights())
	for _, p := range res.Penalties {
		if p.Name == "loose-cluster" {
			return
		}
	}
	t.Fatalf("a loose cluster was not penalized: %+v", res.Penalties)
}

func TestScoreIsBounded(t *testing.T) {
	extreme := Score(candidate(func(c *mine.Candidate) {
		c.Occurrences = 10000
		c.Cadence.Days = 10000
		c.MedianDurationMinutes = 100000
		c.Repos = []string{"a", "b", "c", "d", "e"}
		c.Agents = []string{"a", "b", "c"}
	}), DefaultWeights())
	if extreme.Total > 100 || extreme.Total < 0 {
		t.Fatalf("total = %.1f, outside 0..100", extreme.Total)
	}
}

func TestReasonsAreAlwaysGiven(t *testing.T) {
	res := Score(mine.Candidate{}, DefaultWeights())
	if len(res.Reasons) == 0 {
		t.Fatal("a score with no reasons cannot be audited")
	}
}
