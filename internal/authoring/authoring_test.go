package authoring

import (
	"strings"
	"testing"

	"github.com/HarjjotSinghh/ritual/internal/artifact"
)

func original() artifact.Bundle {
	return artifact.Bundle{Files: []artifact.File{{Path: "SKILL.md", Content: strings.Join([]string{
		"---", "name: verify-storefront", "description: Use when pushing a theme", "---", "",
		"# Verify the storefront", "", "## Steps", "", "1. Push the theme.", "2. Check the cart.", "",
		"## Evidence", "", "Mined by ritual from 8 runs.", "", "<!-- ritual:candidate=abc123 generated=2026-09-01 -->", "",
	}, "\n")}}}
}

func TestValidateAcceptsAFaithfulRewrite(t *testing.T) {
	b := original()
	rewritten := strings.Replace(b.Files[0].Content, "Check the cart.", "Check the cart drawer opens and the totals are right.", 1)
	got, err := validate(rewritten, b)
	if err != nil {
		t.Fatalf("a faithful rewrite was rejected: %v", err)
	}
	if !strings.Contains(got, "cart drawer") {
		t.Fatal("the improvement was lost")
	}
}

func TestValidateRejectsADroppedFrontmatter(t *testing.T) {
	b := original()
	if _, err := validate("# Verify the storefront\n\nbody that lost its header\n", b); err == nil {
		t.Fatal("a response without frontmatter was accepted")
	}
}

func TestValidateRejectsARenamedSkill(t *testing.T) {
	b := original()
	renamed := strings.Replace(b.Files[0].Content, "name: verify-storefront", "name: something-else", 1)
	if _, err := validate(renamed, b); err == nil {
		t.Fatal("a renamed skill was accepted; the install path would silently change")
	}
}

func TestValidateRejectsADroppedProvenanceMarker(t *testing.T) {
	b := original()
	stripped := strings.Replace(b.Files[0].Content, "<!-- ritual:candidate=abc123 generated=2026-09-01 -->", "", 1)
	if _, err := validate(stripped, b); err == nil {
		t.Fatal("a response that erased its provenance was accepted")
	}
}

func TestValidateRejectsMassiveTruncation(t *testing.T) {
	b := original()
	if _, err := validate("---\nname: verify-storefront\n---\n<!-- ritual:candidate=abc123 -->\n", b); err == nil {
		t.Fatal("a response that lost most of the document was accepted")
	}
}

func TestValidateUnwrapsAFencedResponse(t *testing.T) {
	b := original()
	fenced := "```markdown\n" + b.Files[0].Content + "\n```"
	got, err := validate(fenced, b)
	if err != nil {
		t.Fatalf("a fenced response was rejected: %v", err)
	}
	if strings.HasPrefix(got, "```") {
		t.Fatal("the fence survived into the artifact")
	}
}

func TestResolveHonoursNone(t *testing.T) {
	if _, ok := Resolve("none"); ok {
		t.Fatal(`Resolve("none") enabled an author`)
	}
}
