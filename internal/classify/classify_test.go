package classify

import (
	"testing"

	"github.com/HarjjotSinghh/ritual/internal/inventory"
	"github.com/HarjjotSinghh/ritual/internal/mine"
	"github.com/HarjjotSinghh/ritual/internal/score"
)

func procedure() mine.Candidate {
	return mine.Candidate{
		Title:    "Verify the storefront after a theme push",
		Keywords: []string{"storefront", "theme", "verification"},
		Steps: []mine.Step{
			{Action: "read", Support: 1}, {Action: "browser", Support: 1},
			{Action: "shell:npm run test", Support: 0.9}, {Action: "edit", Support: 0.8},
		},
		Cadence: mine.Cadence{Days: 7, Regularity: 0.7, Label: "about daily"},
	}
}

func TestMultiStepWorkBecomesASkill(t *testing.T) {
	d := Classify(procedure(), score.Result{Total: 70}, nil, 28)
	if d.Kind != KindSkill {
		t.Fatalf("kind = %q, want skill: %s", d.Kind, d.Rationale)
	}
}

func TestShortRegularWorkBecomesACommand(t *testing.T) {
	c := procedure()
	c.Steps = c.Steps[:2]
	d := Classify(c, score.Result{Total: 60}, nil, 28)
	if d.Kind != KindCommand {
		t.Fatalf("kind = %q, want command", d.Kind)
	}
}

func TestWorkThatFollowsAnEventBecomesAHook(t *testing.T) {
	c := procedure()
	c.Steps = append([]mine.Step{{Action: "shell:git push", Support: 1}}, c.Steps...)
	d := Classify(c, score.Result{Total: 60}, nil, 28)
	if d.Kind != KindHook {
		t.Fatalf("kind = %q, want hook: %s", d.Kind, d.Rationale)
	}
}

func TestRediscoveryBecomesReference(t *testing.T) {
	c := procedure()
	c.Steps = []mine.Step{
		{Action: "read", Support: 1}, {Action: "search", Support: 1},
		{Action: "list", Support: 0.9}, {Action: "read", Support: 0.9},
	}
	d := Classify(c, score.Result{Total: 50}, nil, 28)
	if d.Kind != KindReference {
		t.Fatalf("kind = %q, want reference", d.Kind)
	}
}

func TestLowScoreIsIgnored(t *testing.T) {
	d := Classify(procedure(), score.Result{Total: 10, Reasons: []string{"thin"}}, nil, 28)
	if d.Kind != KindIgnore {
		t.Fatalf("kind = %q, want ignore", d.Kind)
	}
}

func TestInvokedSkillBecomesAnUpdate(t *testing.T) {
	c := procedure()
	c.SkillRef = "storefront-verify"
	c.SkillRefShare = 0.9
	d := Classify(c, score.Result{Total: 70}, nil, 28)
	if d.Kind != KindUpdate {
		t.Fatalf("kind = %q, want update: proposing a skill the operator already invoked is the worst outcome", d.Kind)
	}
}

func TestExistingSkillPreemptsANewOne(t *testing.T) {
	inv := &inventory.Inventory{Items: []inventory.Item{{
		Kind: inventory.KindSkill, Name: "storefront-verify",
		Description: "verify the storefront theme after a push",
		Tokens:      []string{"storefront", "theme", "verification", "verify", "push"},
	}}}
	d := Classify(procedure(), score.Result{Total: 70}, inv, 28)
	if d.Kind != KindUpdate || d.Existing == nil {
		t.Fatalf("kind = %q, existing = %v", d.Kind, d.Existing)
	}
}

func TestRepeatedPreferenceBecomesARule(t *testing.T) {
	r := mine.RuleCandidate{Text: "always run the mobile check", Occurrences: 3, Sessions: 3, Keywords: []string{"mobile", "check"}}
	d := ClassifyRule(r, nil)
	if d.Kind != KindRule {
		t.Fatalf("kind = %q, want rule", d.Kind)
	}
}
