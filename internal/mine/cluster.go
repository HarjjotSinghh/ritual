package mine

import (
	"math"
	"sort"

	"github.com/HarjjotSinghh/ritual/internal/textutil"
)

// Clustering groups arcs that are the same work done again.
//
// The method is deliberately the simplest one that is reproducible: a greedy
// leader pass over arcs in time order, followed by a merge pass over the
// resulting centroids. Hierarchical clustering would be marginally better and
// would also make the output depend on floating-point tie-breaking across
// platforms, which would undermine the one property that makes these
// suggestions auditable — that the same history always produces the same
// report.
//
// Similarity blends two views that fail in opposite directions. Intent text
// alone groups "fix the build" with "fix the tests". Step sequences alone group
// every workflow that happens to run `git status` first. Requiring both to
// agree is what keeps a cluster meaningful.

// ClusterOptions tune grouping.
type ClusterOptions struct {
	// Threshold is the minimum blended similarity for an arc to join a
	// cluster. Higher means more, tighter clusters.
	Threshold float64
	// IntentWeight is how much the prompt text counts relative to the steps.
	IntentWeight float64
	// MergeThreshold is the minimum centroid similarity for two clusters to be
	// merged in the second pass.
	MergeThreshold float64
	// MinOccurrences drops clusters with fewer arcs. Work done once is not a
	// workflow.
	MinOccurrences int
}

// DefaultClusterOptions are tuned on real multi-agent histories: high enough
// that unrelated tasks stay apart, low enough that the same task phrased two
// different ways still lands together.
func DefaultClusterOptions() ClusterOptions {
	return ClusterOptions{Threshold: 0.42, IntentWeight: 0.55, MergeThreshold: 0.55, MinOccurrences: 3}
}

// Cluster is a group of arcs judged to be the same workflow.
type Cluster struct {
	Arcs     []Arc
	centroid map[string]float64
	steps    textutil.Set
}

type arcVector struct {
	arc    Arc
	intent map[string]float64
	steps  textutil.Set
}

// ClusterArcs groups arcs and returns clusters in descending size order.
func ClusterArcs(arcs []Arc, opts ClusterOptions) []Cluster {
	if opts.Threshold <= 0 {
		opts = DefaultClusterOptions()
	}
	if len(arcs) == 0 {
		return nil
	}

	docs := make([][]string, 0, len(arcs))
	for _, a := range arcs {
		docs = append(docs, intentTokens(a))
	}
	idf := textutil.IDF(docs)

	vectors := make([]arcVector, 0, len(arcs))
	for i, a := range arcs {
		v := arcVector{arc: a, intent: textutil.TFIDF(docs[i], idf), steps: stepSet(a)}
		vectors = append(vectors, v)
	}

	clusters := make([]*Cluster, 0, len(vectors)/4+1)
	for _, v := range vectors {
		best := -1
		bestSim := 0.0
		for i, c := range clusters {
			sim := blended(v.intent, v.steps, c.centroid, c.steps, opts.IntentWeight)
			if sim > bestSim {
				best, bestSim = i, sim
			}
		}
		if best < 0 || bestSim < opts.Threshold {
			c := &Cluster{
				centroid: cloneVector(v.intent),
				steps:    textutil.NewSet(v.steps.Sorted()...),
			}
			c.Arcs = append(c.Arcs, v.arc)
			clusters = append(clusters, c)
			continue
		}
		c := clusters[best]
		c.Arcs = append(c.Arcs, v.arc)
		addInto(c.centroid, v.intent, float64(len(c.Arcs)))
		c.steps.Add(v.steps.Sorted()...)
	}

	clusters = mergePass(clusters, opts)

	out := make([]Cluster, 0, len(clusters))
	for _, c := range clusters {
		if len(c.Arcs) < opts.MinOccurrences {
			continue
		}
		sort.SliceStable(c.Arcs, func(i, j int) bool {
			if !c.Arcs[i].Start.Equal(c.Arcs[j].Start) {
				return c.Arcs[i].Start.Before(c.Arcs[j].Start)
			}
			return c.Arcs[i].ID < c.Arcs[j].ID
		})
		out = append(out, *c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].Arcs) != len(out[j].Arcs) {
			return len(out[i].Arcs) > len(out[j].Arcs)
		}
		return out[i].Arcs[0].ID < out[j].Arcs[0].ID
	})
	return out
}

// mergePass joins clusters whose centroids agree. The leader pass is
// order-dependent by construction: an arc that arrives early can found a
// cluster that a later, better-matching cluster would have absorbed. One merge
// pass removes most of that artifact without reintroducing order sensitivity,
// because merging is evaluated over every pair.
func mergePass(clusters []*Cluster, opts ClusterOptions) []*Cluster {
	if len(clusters) < 2 {
		return clusters
	}
	merged := true
	for round := 0; merged && round < 4; round++ {
		merged = false
		for i := 0; i < len(clusters); i++ {
			if clusters[i] == nil {
				continue
			}
			for j := i + 1; j < len(clusters); j++ {
				if clusters[j] == nil {
					continue
				}
				sim := blended(clusters[j].centroid, clusters[j].steps, clusters[i].centroid, clusters[i].steps, opts.IntentWeight)
				if sim < opts.MergeThreshold {
					continue
				}
				clusters[i].Arcs = append(clusters[i].Arcs, clusters[j].Arcs...)
				addInto(clusters[i].centroid, clusters[j].centroid, 2)
				clusters[i].steps.Add(clusters[j].steps.Sorted()...)
				clusters[j] = nil
				merged = true
			}
		}
		compact := clusters[:0]
		for _, c := range clusters {
			if c != nil {
				compact = append(compact, c)
			}
		}
		clusters = compact
	}
	return clusters
}

func blended(intentA map[string]float64, stepsA textutil.Set, intentB map[string]float64, stepsB textutil.Set, intentWeight float64) float64 {
	text := textutil.Cosine(intentA, intentB)
	steps := textutil.Jaccard(stepsA, stepsB)
	return intentWeight*text + (1-intentWeight)*steps
}

// intentTokens builds the text view of an arc: the opening prompt plus a
// bounded amount of follow-up, with the step vocabulary appended so that two
// prompts phrased differently but performed identically still share terms.
func intentTokens(a Arc) []string {
	tokens := textutil.Tokens(textutil.Truncate(a.Intent, 900))
	for i, p := range a.Prompts {
		if i == 0 {
			continue
		}
		if i > 4 {
			break
		}
		tokens = append(tokens, textutil.Tokens(textutil.Truncate(p, 240))...)
	}
	for _, cmd := range a.Commands {
		tokens = append(tokens, textutil.Tokens(cmd)...)
	}
	return tokens
}

// stepSet is the structural view: distinct actions plus adjacent pairs, so
// order carries weight without demanding an exact sequence match.
func stepSet(a Arc) textutil.Set {
	set := textutil.NewSet(a.Steps...)
	for _, sh := range textutil.Shingles(a.Steps, 2) {
		set.Add(sh)
	}
	return set
}

func cloneVector(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// addInto folds a new vector into a running mean of n members.
func addInto(centroid, add map[string]float64, n float64) {
	if n <= 0 {
		n = 1
	}
	w := 1 / n
	for k, v := range centroid {
		centroid[k] = v * (1 - w)
	}
	for k, v := range add {
		centroid[k] += v * w
	}
}

// Cohesion is the mean similarity of a cluster's members to its centroid. It is
// reported alongside every candidate so a loose grouping is visible rather than
// implied.
func (c Cluster) Cohesion(intentWeight float64) float64 {
	if len(c.Arcs) == 0 {
		return 0
	}
	docs := make([][]string, 0, len(c.Arcs))
	for _, a := range c.Arcs {
		docs = append(docs, intentTokens(a))
	}
	idf := textutil.IDF(docs)
	total := 0.0
	for i, a := range c.Arcs {
		total += blended(textutil.TFIDF(docs[i], idf), stepSet(a), c.centroid, c.steps, intentWeight)
	}
	mean := total / float64(len(c.Arcs))
	if math.IsNaN(mean) || math.IsInf(mean, 0) {
		return 0
	}
	if mean > 1 {
		return 1
	}
	return mean
}
