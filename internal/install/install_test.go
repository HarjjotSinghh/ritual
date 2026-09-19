package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HarjjotSinghh/ritual/internal/artifact"
	"github.com/HarjjotSinghh/ritual/internal/classify"
)

func target(t *testing.T) Target {
	root := t.TempDir()
	return Target{
		Key: "test", DisplayName: "Test",
		SkillDir:   filepath.Join(root, "skills"),
		CommandDir: filepath.Join(root, "commands"),
		RulesFile:  filepath.Join(root, "AGENTS.md"),
	}
}

func skillBundle() artifact.Bundle {
	return artifact.Bundle{
		Kind: classify.KindSkill, Name: "Verify", Slug: "verify",
		Files: []artifact.File{{Path: "SKILL.md", Content: "---\nname: verify\n---\n\nbody\n"}},
	}
}

func TestPlanCreatesThenSkips(t *testing.T) {
	tgt := target(t)
	plan, err := PlanInstall(skillBundle(), tgt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Writes[0].Action != ActionCreate {
		t.Fatalf("action = %q, want create", plan.Writes[0].Action)
	}
	if _, err := Apply(plan); err != nil {
		t.Fatal(err)
	}

	again, err := PlanInstall(skillBundle(), tgt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if again.Writes[0].Action != ActionSkip {
		t.Fatalf("action = %q, want skip: an existing file must not be replaced silently", again.Writes[0].Action)
	}
	if again.Writes[0].Reason == "" {
		t.Fatal("a skip with no reason is a skip nobody can act on")
	}
}

func TestForceOverwrites(t *testing.T) {
	tgt := target(t)
	plan, _ := PlanInstall(skillBundle(), tgt, Options{})
	if _, err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	forced, _ := PlanInstall(skillBundle(), tgt, Options{Force: true})
	if forced.Writes[0].Action != ActionOverwrite {
		t.Fatalf("action = %q, want overwrite", forced.Writes[0].Action)
	}
}

func TestApplyWritesToTheSkillDirectory(t *testing.T) {
	tgt := target(t)
	plan, _ := PlanInstall(skillBundle(), tgt, Options{})
	changed, err := Apply(plan)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(tgt.SkillDir, "verify", "SKILL.md")
	if len(changed) != 1 || changed[0] != want {
		t.Fatalf("changed = %v, want %v", changed, want)
	}
	data, err := os.ReadFile(want)
	if err != nil || !strings.Contains(string(data), "name: verify") {
		t.Fatalf("file content = %q, err = %v", data, err)
	}
}

func TestRuleAppendsAndDoesNotDuplicate(t *testing.T) {
	tgt := target(t)
	bundle := artifact.Bundle{
		Kind: classify.KindRule, Name: "rule", Slug: "rule", Append: true,
		Files: []artifact.File{{Path: "AGENTS.md", Content: "- Always run the mobile check.\n"}},
	}
	if err := os.WriteFile(tgt.RulesFile, []byte("# Rules\n\n- Existing rule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		plan, err := PlanInstall(bundle, tgt, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if plan.Writes[0].Action != ActionAppend {
			t.Fatalf("action = %q, want append", plan.Writes[0].Action)
		}
		if _, err := Apply(plan); err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(tgt.RulesFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Existing rule") {
		t.Fatal("appending destroyed the existing rules file")
	}
	if strings.Count(string(data), "Always run the mobile check") != 1 {
		t.Fatalf("the rule was appended twice:\n%s", data)
	}
}

func TestCommandFallsBackToSkillsWhenUnsupported(t *testing.T) {
	tgt := target(t)
	tgt.CommandDir = ""
	bundle := artifact.Bundle{
		Kind: classify.KindCommand, Name: "eod", Slug: "eod",
		Files: []artifact.File{{Path: "eod.md", Content: "body"}},
	}
	plan, err := PlanInstall(bundle, tgt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.Writes[0].Path, filepath.Join("skills", "eod", "SKILL.md")) {
		t.Fatalf("path = %q, want a skills fallback", plan.Writes[0].Path)
	}
}

func TestRuleWithoutARulesFileIsAnError(t *testing.T) {
	tgt := target(t)
	tgt.RulesFile = ""
	bundle := artifact.Bundle{Kind: classify.KindRule, Append: true, Files: []artifact.File{{Path: "AGENTS.md", Content: "x"}}}
	if _, err := PlanInstall(bundle, tgt, Options{}); err == nil {
		t.Fatal("planning a rule against a target with no rules file should fail loudly")
	}
}
