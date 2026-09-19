package artifact

import (
	"strings"
	"testing"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/classify"
	"github.com/HarjjotSinghh/ritual/internal/mine"
	"github.com/HarjjotSinghh/ritual/internal/report"
	"github.com/HarjjotSinghh/ritual/internal/score"
)

func finding() report.Finding {
	return report.Finding{
		Candidate: mine.Candidate{
			ID: "abc123", Title: "Verify the storefront after a theme push",
			Slug:        "verify-the-storefront-after-a-theme-push",
			Summary:     "Ran 8 times across 8 sessions in storefront, about daily.",
			Occurrences: 8, Sessions: 8,
			Agents: []string{"claude"}, Repos: []string{"storefront"},
			Keywords: []string{"storefront", "theme", "verification"},
			Phrases:  []mine.ScoredPhrase{{Phrase: "theme push", Score: 3}},
			Commands: []string{"shopify theme push", "npm run test"},
			Steps: []mine.Step{
				{Action: "shell:shopify theme push", Support: 1, Count: 8},
				{Action: "browser", Support: 0.75, Count: 6},
			},
			Cadence:  mine.Cadence{Days: 8, Label: "about daily", FirstSeen: time.Now().AddDate(0, 0, -10), LastSeen: time.Now()},
			Evidence: []mine.Evidence{{Agent: "claude", Intent: "verify the storefront", At: time.Now()}},
			Cohesion: 0.7,
		},
		Score:    score.Result{Total: 68},
		Decision: classify.Decision{Kind: classify.KindSkill, Rationale: "a procedure"},
	}
}

func TestSkillHasValidFrontmatter(t *testing.T) {
	b := Build(finding())
	if len(b.Files) != 1 || b.Files[0].Path != "SKILL.md" {
		t.Fatalf("files = %+v", b.Files)
	}
	content := b.Files[0].Content
	if !strings.HasPrefix(content, "---\n") {
		t.Fatal("SKILL.md does not open with frontmatter")
	}
	if !strings.Contains(content, "name: verify-the-storefront-after-a") {
		t.Fatalf("frontmatter name is wrong or unbounded:\n%s", content[:200])
	}
	if !strings.Contains(content, "description:") {
		t.Fatal("frontmatter has no description, so no agent can decide when to use it")
	}
}

func TestSkillSlugIsBounded(t *testing.T) {
	f := finding()
	f.Slug = "one-two-three-four-five-six-seven-eight"
	b := Build(f)
	if strings.Count(b.Slug, "-") > 4 {
		t.Fatalf("slug = %q, want at most five segments", b.Slug)
	}
}

func TestStepSupportIsStatedWhenPartial(t *testing.T) {
	content := Build(finding()).Files[0].Content
	if !strings.Contains(content, "*(6 of 8 runs)*") {
		t.Fatalf("a step performed in three quarters of runs was presented as certain:\n%s", content)
	}
}

func TestEvidenceAndProvenanceSurvive(t *testing.T) {
	content := Build(finding()).Files[0].Content
	if !strings.Contains(content, "<!-- ritual:candidate=abc123") {
		t.Fatal("the provenance marker is missing")
	}
	if !strings.Contains(content, "## Evidence") {
		t.Fatal("the evidence section is missing")
	}
}

func TestDescriptionEscapesYAMLSpecials(t *testing.T) {
	f := finding()
	f.Phrases = []mine.ScoredPhrase{{Phrase: "deploy: staging", Score: 3}}
	content := Build(f).Files[0].Content
	line := ""
	for _, l := range strings.Split(content, "\n") {
		if strings.HasPrefix(l, "description:") {
			line = l
			break
		}
	}
	if !strings.Contains(line, `"`) {
		t.Fatalf("a description with a colon was not quoted: %q", line)
	}
}

func TestCommandProducesASingleMarkdownFile(t *testing.T) {
	f := finding()
	f.Decision.Kind = classify.KindCommand
	b := Build(f)
	if b.Kind != classify.KindCommand || !strings.HasSuffix(b.Files[0].Path, ".md") {
		t.Fatalf("bundle = %+v", b)
	}
}

func TestRuleIsAppendedNotWritten(t *testing.T) {
	b := BuildRule(report.RuleFinding{
		RuleCandidate: mine.RuleCandidate{
			Text:        "no, always run the mobile check before reporting done",
			Occurrences: 3, Sessions: 3, Keywords: []string{"mobile", "check"},
		},
		Decision: classify.Decision{Kind: classify.KindRule},
	})
	if !b.Append {
		t.Fatal("a rule must be appended, not written over a rules file")
	}
	content := b.Files[0].Content
	if strings.Contains(content, "no, always") {
		t.Fatalf("the conversational lead-in was not trimmed: %q", content)
	}
	if !strings.Contains(content, "Always run the mobile check") {
		t.Fatalf("the rule text was lost: %q", content)
	}
	if !strings.Contains(content, "stated 3 times") {
		t.Fatal("the rule lost its provenance comment")
	}
}

func TestNotesWarnAboutLooseClusters(t *testing.T) {
	f := finding()
	f.Cohesion = 0.3
	b := Build(f)
	joined := strings.Join(b.Notes, " ")
	if !strings.Contains(joined, "agree only") {
		t.Fatalf("a loose cluster produced no warning: %v", b.Notes)
	}
}
