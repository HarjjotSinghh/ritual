// Package textutil holds the small deterministic text primitives the miner is
// built on: tokenization, stopword removal, similarity, and slugging.
//
// Every function here is pure and ordering-stable. That matters more than it
// sounds: ritual's core claim is that the same sessions produce the same
// report, so a map iteration that leaks into output would be a correctness bug,
// not a cosmetic one.
package textutil

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var wordRE = regexp.MustCompile(`[a-z0-9][a-z0-9_\-\.]*`)

// Stopwords are dropped before similarity. The list covers English filler plus
// the conversational scaffolding that shows up in every single coding-agent
// prompt ("please", "can you", "now", "ok") and would otherwise make unrelated
// prompts look alike.
var Stopwords = map[string]struct{}{}

func init() {
	for _, w := range strings.Fields(`a an the and or but if then than so because as of to in on at by for with from into over under again
	i me my we our you your it its this that these those is are was were be been being do does did doing have has had having
	can could should would will shall may might must not no nor just really very please thanks thank ok okay now then
	let lets make made get got go going want need like also too still yet here there what which who whom when where why how
	all any both each few more most other some such only own same`) {
		Stopwords[w] = struct{}{}
	}
}

// Tokens lowercases, splits on non-word runes, and drops stopwords and
// single-character noise.
func Tokens(s string) []string {
	s = strings.ToLower(s)
	raw := wordRE.FindAllString(s, -1)
	out := make([]string, 0, len(raw))
	for _, w := range raw {
		w = strings.Trim(w, "-._")
		if len(w) < 2 {
			continue
		}
		if _, stop := Stopwords[w]; stop {
			continue
		}
		if isAllDigits(w) {
			continue
		}
		out = append(out, w)
	}
	return out
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// Set is a deterministic string set.
type Set map[string]struct{}

// NewSet builds a set from values.
func NewSet(values ...string) Set {
	s := make(Set, len(values))
	for _, v := range values {
		s[v] = struct{}{}
	}
	return s
}

// Add inserts values.
func (s Set) Add(values ...string) {
	for _, v := range values {
		s[v] = struct{}{}
	}
}

// Sorted returns the members in lexical order.
func (s Set) Sorted() []string {
	out := make([]string, 0, len(s))
	for v := range s {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// Jaccard is the intersection over union of two sets. Two empty sets are
// treated as maximally similar, which keeps arcs with no tool calls from being
// scattered across every cluster.
func Jaccard(a, b Set) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	small, large := a, b
	if len(b) < len(a) {
		small, large = b, a
	}
	for v := range small {
		if _, ok := large[v]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// Cosine is the cosine similarity of two sparse weight vectors.
func Cosine(a, b map[string]float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	var dot, na, nb float64
	for k, v := range a {
		na += v * v
		if w, ok := b[k]; ok {
			dot += v * w
		}
	}
	for _, v := range b {
		nb += v * v
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// IDF computes inverse document frequency over a corpus of token slices.
func IDF(docs [][]string) map[string]float64 {
	df := make(map[string]int, 512)
	for _, d := range docs {
		seen := make(map[string]struct{}, len(d))
		for _, t := range d {
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			df[t]++
		}
	}
	n := float64(len(docs))
	idf := make(map[string]float64, len(df))
	for t, c := range df {
		idf[t] = math.Log((n+1)/(float64(c)+1)) + 1
	}
	return idf
}

// TFIDF builds a normalized weight vector for one document.
func TFIDF(tokens []string, idf map[string]float64) map[string]float64 {
	if len(tokens) == 0 {
		return nil
	}
	tf := make(map[string]float64, len(tokens))
	for _, t := range tokens {
		tf[t]++
	}
	out := make(map[string]float64, len(tf))
	for t, c := range tf {
		w := idf[t]
		if w == 0 {
			w = 1
		}
		out[t] = (1 + math.Log(c)) * w
	}
	return out
}

// Shingles returns the overlapping n-grams of a sequence, joined with " > ".
// The miner uses them to compare tool orderings without demanding an exact
// match on the whole sequence.
func Shingles(seq []string, n int) []string {
	if n <= 0 || len(seq) < n {
		if len(seq) == 0 {
			return nil
		}
		return []string{strings.Join(seq, " > ")}
	}
	out := make([]string, 0, len(seq)-n+1)
	for i := 0; i+n <= len(seq); i++ {
		out = append(out, strings.Join(seq[i:i+n], " > "))
	}
	return out
}

var slugRE = regexp.MustCompile(`[^a-z0-9]+`)

// Slug converts a phrase into a kebab-case identifier suitable for a skill
// directory name.
func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRE.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "workflow"
	}
	if len(s) > 60 {
		s = s[:60]
		s = strings.Trim(s, "-")
	}
	return s
}

// Fingerprint is a short stable hash, used for candidate ids so the same
// workflow keeps the same id across runs.
func Fingerprint(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:10]
}

// Truncate shortens a string on a rune boundary and marks it.
func Truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max])) + "…"
}

// FirstLine returns the first non-empty line, collapsed to single spaces.
func FirstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return strings.Join(strings.Fields(line), " ")
		}
	}
	return ""
}

// Dedupe returns values with duplicates removed, preserving first-seen order.
func Dedupe(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// TopN returns the n highest-count keys of a counter, ties broken lexically so
// the result is stable.
func TopN(counts map[string]int, n int) []string {
	type kv struct {
		k string
		v int
	}
	all := make([]kv, 0, len(counts))
	for k, v := range counts {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].v != all[j].v {
			return all[i].v > all[j].v
		}
		return all[i].k < all[j].k
	})
	if n > len(all) {
		n = len(all)
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, all[i].k)
	}
	return out
}
