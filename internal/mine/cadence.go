package mine

import (
	"math"
	"sort"
	"time"
)

// MeasureCadence describes how a workflow recurs over calendar time.
//
// Counting occurrences alone is misleading: twelve runs in one afternoon while
// debugging a flaky deploy is not a daily ritual, and a tool that suggested
// automating it would be reacting to a bad day. So recurrence is measured in
// distinct days, and the spacing between those days is what decides whether a
// workflow is a habit or a burst.
func MeasureCadence(arcs []Arc) Cadence {
	days := make(map[string]time.Time, len(arcs))
	for _, a := range arcs {
		if a.Start.IsZero() {
			continue
		}
		d := a.Start.UTC().Truncate(24 * time.Hour)
		days[d.Format("2006-01-02")] = d
	}
	if len(days) == 0 {
		return Cadence{Label: "undated"}
	}

	ordered := make([]time.Time, 0, len(days))
	for _, d := range days {
		ordered = append(ordered, d)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Before(ordered[j]) })

	c := Cadence{
		FirstSeen: ordered[0],
		LastSeen:  ordered[len(ordered)-1],
		Days:      len(ordered),
	}
	c.SpanDays = int(c.LastSeen.Sub(c.FirstSeen).Hours()/24) + 1

	if len(ordered) < 2 {
		c.Label = "once"
		return c
	}

	gaps := make([]float64, 0, len(ordered)-1)
	for i := 1; i < len(ordered); i++ {
		gaps = append(gaps, ordered[i].Sub(ordered[i-1]).Hours())
	}
	c.MedianIntervalHours = round3(median(gaps))
	c.Regularity = round3(regularity(gaps))
	c.Label = cadenceLabel(c)
	return c
}

// regularity is 1 minus the coefficient of variation of the gaps, clamped to
// 0..1. Evenly spaced runs score near 1; a cluster of runs followed by a long
// silence scores near 0.
func regularity(gaps []float64) float64 {
	if len(gaps) < 2 {
		return 0
	}
	var sum float64
	for _, g := range gaps {
		sum += g
	}
	mean := sum / float64(len(gaps))
	if mean <= 0 {
		return 0
	}
	var variance float64
	for _, g := range gaps {
		variance += (g - mean) * (g - mean)
	}
	variance /= float64(len(gaps))
	cv := math.Sqrt(variance) / mean
	r := 1 - cv
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

func cadenceLabel(c Cadence) string {
	switch {
	case c.Days < 2:
		return "once"
	case c.MedianIntervalHours <= 0:
		return "same day"
	case c.Regularity < 0.25:
		return "bursty"
	case c.MedianIntervalHours <= 30:
		return "about daily"
	case c.MedianIntervalHours <= 96:
		return "every few days"
	case c.MedianIntervalHours <= 200:
		return "roughly weekly"
	case c.MedianIntervalHours <= 800:
		return "every few weeks"
	default:
		return "occasional"
	}
}
