package mine

import (
	"math"
	"sort"
	"strings"

	"github.com/HarjjotSinghh/ritual/internal/textutil"
)

// Naming a workflow from single tokens produces titles like "Canvases
// commitments encouraged": each word is individually distinctive and the
// combination means nothing. What people actually use to name their own work is
// a phrase — "alert triage", "end of day update", "theme push verification" —
// so candidates are named from the phrases their prompts share.
//
// Phrases are scored by how much more often they appear inside the cluster than
// across the whole history, with the in-cluster share weighted so a phrase used
// in one prompt of twelve cannot win on rarity alone.

// PhraseIndex holds corpus-wide phrase frequencies so a cluster's phrases can
// be judged against the operator's whole history.
type PhraseIndex struct {
	df    map[string]int
	total int
}

// BuildPhraseIndex counts, for every phrase, how many arcs contain it.
func BuildPhraseIndex(arcs []Arc) *PhraseIndex {
	idx := &PhraseIndex{df: make(map[string]int, 4096), total: len(arcs)}
	for _, a := range arcs {
		for phrase := range arcPhrases(a) {
			idx.df[phrase]++
		}
	}
	return idx
}

// ScoredPhrase is a phrase with the weight that selected it, so a caller can
// tell a strong shared name from the best of a weak field.
type ScoredPhrase struct {
	Phrase string  `json:"phrase"`
	Score  float64 `json:"score"`
}

// StrongPhrase is the score above which a phrase is a good enough name to use
// on its own. Below it, the caller falls back to naming the workflow by what it
// touches rather than by what its prompts said.
const StrongPhrase = 0.45

// Top returns the highest-scoring phrases for a set of arcs.
func (idx *PhraseIndex) Top(arcs []Arc, n int) []ScoredPhrase {
	if len(arcs) == 0 {
		return nil
	}
	local := make(map[string]int, 256)
	position := make(map[string]float64, 256)
	for _, a := range arcs {
		for phrase, pos := range arcPhrases(a) {
			local[phrase]++
			position[phrase] += pos
		}
	}

	type scored struct {
		phrase string
		score  float64
	}
	corpus := float64(idx.total)
	if corpus <= 0 {
		corpus = 1
	}
	inCluster := float64(len(arcs))

	ranked := make([]scored, 0, len(local))
	for phrase, count := range local {
		share := float64(count) / inCluster
		// A phrase present in only one run describes that run, not the
		// workflow. Beyond that the share threshold stays low, because a
		// distinctive phrase used by a quarter of the runs is far more telling
		// than a filler phrase used by all of them.
		if count < 2 || share < 0.22 {
			continue
		}
		outside := float64(idx.df[phrase]-count) / maxFloat(corpus-inCluster, 1)
		// Log-odds of appearing inside versus outside, weighted by how much of
		// the cluster uses it. Both halves matter: distinctiveness alone
		// surfaces typos, frequency alone surfaces "the file".
		lift := (share + 0.01) / (outside + 0.01)
		// Where a phrase sits in the prompt matters as much as how often it
		// appears. When a prompt is a near-identical template — which is
		// exactly what a repeated workflow looks like — every phrase in it is
		// equally distinctive, and the only thing separating an incidental
		// pairing from the actual subject is that people state the task first
		// and elaborate afterwards.
		early := 1 - 0.65*(position[phrase]/float64(count))
		ranked = append(ranked, scored{phrase, share * logApprox(lift) * phraseWeight(phrase) * early})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		// Among equally distinctive phrases, prefer the one the operator uses
		// elsewhere too: an established term of theirs, not a one-off pairing.
		if idx.df[ranked[i].phrase] != idx.df[ranked[j].phrase] {
			return idx.df[ranked[i].phrase] > idx.df[ranked[j].phrase]
		}
		return ranked[i].phrase < ranked[j].phrase
	})

	out := make([]ScoredPhrase, 0, n)
	for _, r := range ranked {
		if containsOverlapScored(out, r.phrase) {
			continue
		}
		out = append(out, ScoredPhrase{Phrase: r.phrase, Score: round3(r.score)})
		if len(out) >= n {
			break
		}
	}
	return out
}

// arcPhrases returns the meaningful two- and three-word phrases in an arc's
// prompts, mapped to where the phrase first appears as a fraction of the
// prompt's length. A phrase repeated within one prompt counts once, at its
// earliest position.
func arcPhrases(a Arc) map[string]float64 {
	out := make(map[string]float64, 64)
	texts := make([]string, 0, 3)
	texts = append(texts, textutil.Truncate(a.Intent, 600))
	for i, p := range a.Prompts {
		if i == 0 || i > 2 {
			continue
		}
		texts = append(texts, textutil.Truncate(p, 200))
	}
	for _, text := range texts {
		tokens := textutil.FilterMeaningful(textutil.Tokens(text))
		for n := 2; n <= 3; n++ {
			shingles := textutil.Shingles(tokens, n)
			span := float64(len(shingles) - 1)
			for i, phrase := range shingles {
				phrase = strings.ReplaceAll(phrase, " > ", " ")
				if lowInformationPhrase(phrase) {
					continue
				}
				pos := 0.0
				if span > 0 {
					pos = float64(i) / span
				}
				if existing, ok := out[phrase]; !ok || pos < existing {
					out[phrase] = pos
				}
			}
		}
	}
	return out
}

// lowInformation words are common to the operator's whole way of talking rather
// than to any one workflow. A phrase made only of these is filler.
var lowInformation = map[string]struct{}{
	"give": {}, "cool": {}, "today": {}, "look": {}, "context": {}, "proceed": {},
	"stop": {}, "latest": {}, "once": {}, "thing": {}, "things": {}, "stuff": {},
	"way": {}, "part": {}, "bit": {}, "good": {}, "better": {}, "best": {}, "sure": {},
	"file": {}, "files": {}, "code": {}, "project": {}, "repo": {}, "work": {},
	"please": {}, "maybe": {}, "something": {}, "anything": {}, "everything": {},
	"basically": {}, "essentially": {}, "actually": {}, "currently": {},
}

func lowInformationPhrase(phrase string) bool {
	words := strings.Fields(phrase)
	informative := 0
	for _, w := range words {
		if _, ok := lowInformation[w]; !ok {
			informative++
		}
	}
	return informative < 2
}

// phraseWeight prefers longer phrases, which read as names, over bigrams that
// happen to be frequent.
func phraseWeight(phrase string) float64 {
	if len(strings.Fields(phrase)) >= 3 {
		return 1.15
	}
	return 1
}

// containsOverlapScored drops a phrase that repeats a word already used by a
// chosen phrase, so a title is not "alert triage, triage alerts".
func containsOverlapScored(chosen []ScoredPhrase, phrase string) bool {
	words := textutil.NewSet(strings.Fields(phrase)...)
	for _, c := range chosen {
		for _, w := range strings.Fields(c.Phrase) {
			if _, ok := words[w]; ok {
				return true
			}
		}
	}
	return false
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// logApprox is ln with a small floor, so a phrase no more common inside the
// cluster than outside it still carries a little weight rather than vanishing.
func logApprox(v float64) float64 {
	if v <= 1 {
		return 0.05
	}
	return math.Log(v)
}
