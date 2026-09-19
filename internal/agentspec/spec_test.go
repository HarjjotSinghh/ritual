package agentspec

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"projects/**/*.jsonl", "projects/slug/abc.jsonl", true},
		{"projects/**/*.jsonl", "projects/a/b/c/abc.jsonl", true},
		{"projects/**/*.jsonl", "projects/abc.jsonl", true},
		{"projects/**/*.jsonl", "other/abc.jsonl", false},
		{"projects/**/*.jsonl", "projects/slug/abc.json", false},
		{"sessions/**/chat_history.jsonl", "sessions/enc/id/chat_history.jsonl", true},
		{"sessions/**/chat_history.jsonl", "sessions/enc/id/summary.json", false},
		{"opencode.db", "opencode.db", true},
		{"opencode.db", "sub/opencode.db", false},
		{"projects/**/agent-transcripts/**/*.jsonl", "projects/s/agent-transcripts/id/id.jsonl", true},
		{"projects/**/agent-transcripts/**/*.jsonl", "projects/s/agent-transcripts/id.jsonl", true},
		{"tmp/**/chats/*.json", "tmp/hash/chats/session-1.json", true},
		{"", "anything", false},
	}
	for _, c := range cases {
		if got := matchGlob(c.pattern, c.path); got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestCatalogIsInternallyConsistent(t *testing.T) {
	seen := map[string]struct{}{}
	for _, s := range Catalog() {
		if s.Key == "" || s.DisplayName == "" {
			t.Errorf("spec %+v is missing an identity", s)
		}
		if _, dup := seen[s.Key]; dup {
			t.Errorf("duplicate catalog key %q", s.Key)
		}
		seen[s.Key] = struct{}{}
		if len(s.Roots) == 0 {
			t.Errorf("%s has no default root", s.Key)
		}
		if s.Glob == "" {
			t.Errorf("%s has no session glob", s.Key)
		}
		if readerMissing(s.Layout) {
			t.Errorf("%s declares layout %q with no reader", s.Key, s.Layout)
		}
	}
}

// readerMissing is a guard against adding a catalog entry without a reader,
// which would silently skip an agent rather than failing loudly.
func readerMissing(layout Layout) bool {
	switch layout {
	case LayoutClaudeJSONL, LayoutCodexRollout, LayoutCursorTranscript, LayoutOpenCodeSQLite,
		LayoutGeminiChat, LayoutGrokSession, LayoutQwenJSONL, LayoutKimiState,
		LayoutCopilotEvents, LayoutClineTask, LayoutPiJSONL, LayoutAiderMarkdown:
		return false
	}
	return true
}

func TestLookup(t *testing.T) {
	if _, ok := Lookup("claude"); !ok {
		t.Fatal("claude is missing from the catalog")
	}
	if _, ok := Lookup("nope"); ok {
		t.Fatal("Lookup invented an agent")
	}
}
