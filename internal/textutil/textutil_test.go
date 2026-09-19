package textutil

import (
	"reflect"
	"testing"
)

func TestTokensDropsStopwordsAndNoise(t *testing.T) {
	got := Tokens("Please can you just review the Shopify checkout flow again, 2026")
	want := []string{"review", "shopify", "checkout", "flow"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Tokens = %v, want %v", got, want)
	}
}

func TestMeaningfulRejectsIdentifiers(t *testing.T) {
	cases := map[string]bool{
		"checkout":                             true,
		"storefront-verify":                       true,
		"2026-09-09":                           false,
		"83042602":                             false,
		"deadbeefdeadbeef":                     false,
		"file.tsx":                             false,
		"ab":                                   false,
		"aaaaaaaa":                             false,
		"supercalifragilisticexpialidocious12": false,
	}
	for token, want := range cases {
		if got := Meaningful(token); got != want {
			t.Errorf("Meaningful(%q) = %v, want %v", token, got, want)
		}
	}
}

func TestJaccardEdges(t *testing.T) {
	if got := Jaccard(NewSet(), NewSet()); got != 1 {
		t.Fatalf("two empty sets = %v, want 1", got)
	}
	if got := Jaccard(NewSet("a"), NewSet()); got != 0 {
		t.Fatalf("one empty set = %v, want 0", got)
	}
	if got := Jaccard(NewSet("a", "b"), NewSet("b", "c")); got != 1.0/3.0 {
		t.Fatalf("Jaccard = %v, want 1/3", got)
	}
}

func TestShinglesJoinsWithSeparator(t *testing.T) {
	got := Shingles([]string{"a", "b", "c"}, 2)
	want := []string{"a > b", "b > c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Shingles = %v, want %v", got, want)
	}
	if got := Shingles([]string{"a"}, 3); !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("short sequence = %v, want the whole sequence", got)
	}
}

func TestFingerprintIsStable(t *testing.T) {
	a := Fingerprint("candidate", "deploy-check", "read|edit")
	b := Fingerprint("candidate", "deploy-check", "read|edit")
	if a != b {
		t.Fatalf("Fingerprint is not stable: %q != %q", a, b)
	}
	if c := Fingerprint("candidate", "deploy-check", "read|write"); c == a {
		t.Fatal("different inputs produced the same fingerprint")
	}
}

func TestTopNBreaksTiesLexically(t *testing.T) {
	got := TopN(map[string]int{"b": 2, "a": 2, "c": 1}, 2)
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("TopN = %v, want [a b]", got)
	}
}

func TestSlugTrimsAndBounds(t *testing.T) {
	if got := Slug("  Verify Storefront: theme push!  "); got != "verify-storefront-theme-push" {
		t.Fatalf("Slug = %q", got)
	}
	if got := Slug("!!!"); got != "workflow" {
		t.Fatalf("Slug of punctuation = %q, want the fallback", got)
	}
}
