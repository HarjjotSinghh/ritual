package redact

import (
	"strings"
	"testing"
)

func TestTextRedactsProviderKeys(t *testing.T) {
	r := New()
	cases := []struct {
		in   string
		gone string
	}{
		{"export ANTHROPIC_API_KEY=sk-ant-api03-abcdefghijklmnop1234", "sk-ant-api03"},
		{"token: ghp_abcdefghijklmnopqrstuvwxyz0123", "ghp_abcdef"},
		{"Authorization: Bearer abcdefghijklmnopqrst", "abcdefghijklmnopqrst"},
		{"DATABASE_PASSWORD=hunter2hunter2", "hunter2hunter2"},
		{"https://user:secretpass@example.com/db", "secretpass"},
		{"xoxb-123456789012-abcdefghijkl", "xoxb-123456789012"},
	}
	for _, c := range cases {
		got := r.Text(c.in)
		if strings.Contains(got, c.gone) {
			t.Errorf("Text(%q) = %q, still contains %q", c.in, got, c.gone)
		}
	}
}

func TestTextKeepsOrdinaryProse(t *testing.T) {
	r := New()
	in := "Run the theme push and check the cart drawer on mobile"
	if got := r.Text(in); got != in {
		t.Fatalf("Text rewrote ordinary prose: %q", got)
	}
}

func TestEmailsAreMaskedUnlessKept(t *testing.T) {
	r := New()
	if got := r.Text("ping alice@example.com about it"); strings.Contains(got, "alice@example.com") {
		t.Fatalf("email survived: %q", got)
	}
	r2 := New().KeepEmails(true)
	if got := r2.Text("ping alice@example.com"); !strings.Contains(got, "alice@example.com") {
		t.Fatalf("KeepEmails did not keep the address: %q", got)
	}
}

func TestArgsTruncatesLongValues(t *testing.T) {
	r := New()
	got := r.Args(map[string]string{"content": strings.Repeat("x", 500)}, 50)
	if len(got["content"]) > 60 {
		t.Fatalf("value not truncated: %d bytes", len(got["content"]))
	}
	if !strings.HasSuffix(got["content"], "…") {
		t.Fatal("truncation was not marked")
	}
}
