package inventory

import "testing"

func TestParseFrontmatter(t *testing.T) {
	name, description := parseFrontmatter("---\nname: storefront-verify\ndescription: \"Post-release verification\"\n---\n\n# body\n")
	if name != "storefront-verify" {
		t.Fatalf("name = %q", name)
	}
	if description != "Post-release verification" {
		t.Fatalf("description = %q", description)
	}
}

func TestParseFrontmatterIgnoresBodyWithout(t *testing.T) {
	name, _ := parseFrontmatter("# just a heading\nname: not-frontmatter\n")
	if name != "" {
		t.Fatalf("name = %q, want empty when there is no frontmatter block", name)
	}
}

func TestFirstProseSkipsHeadingsAndFrontmatter(t *testing.T) {
	got := firstProse("---\nname: x\n---\n\n# Title\n\nThe actual sentence.\n")
	if got != "The actual sentence." {
		t.Fatalf("firstProse = %q", got)
	}
}

func TestBestMatchUsesContainment(t *testing.T) {
	inv := &Inventory{Items: []Item{
		{Kind: KindSkill, Name: "storefront-verify", Tokens: []string{"storefront", "theme", "verify", "push", "mobile"}},
		{Kind: KindSkill, Name: "invoice-reconcile", Tokens: []string{"invoice", "billing"}},
	}}
	match, ok := inv.BestMatch([]string{"storefront", "theme", "verify"}, KindSkill)
	if !ok || match.Item.Name != "storefront-verify" {
		t.Fatalf("match = %+v, ok = %v", match, ok)
	}
}

func TestBestMatchRefusesWeakOverlap(t *testing.T) {
	inv := &Inventory{Items: []Item{
		{Kind: KindSkill, Name: "invoice-reconcile", Tokens: []string{"invoice", "billing"}},
	}}
	if _, ok := inv.BestMatch([]string{"storefront", "theme", "verify", "mobile"}, KindSkill); ok {
		t.Fatal("a weak overlap was reported as a match, which would hide a real suggestion")
	}
}

func TestBestMatchRespectsKindFilter(t *testing.T) {
	inv := &Inventory{Items: []Item{
		{Kind: KindRules, Name: "AGENTS.md", Tokens: []string{"storefront", "theme", "verify"}},
	}}
	if _, ok := inv.BestMatch([]string{"storefront", "theme", "verify"}, KindSkill); ok {
		t.Fatal("a rules file matched a skill lookup")
	}
	if _, ok := inv.BestMatch([]string{"storefront", "theme", "verify"}, KindRules); !ok {
		t.Fatal("the rules file did not match a rules lookup")
	}
}
