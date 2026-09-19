package ingest

import "testing"

func TestCanonicalMapsVendorNames(t *testing.T) {
	cases := map[string]string{
		"Read":                             "read",
		"str_replace_editor":               "edit",
		"run_terminal_command":             "shell",
		"Bash":                             "shell",
		"codebase_search":                  "search",
		"WebSearch":                        "web_search",
		"mcp__slack__send_message":         "mcp:slack:send-message",
		"mcp__ccd_session_mgmt__get_usage": "",
		"ToolSearch":                       "",
		"StructuredOutput":                 "",
		"SomeVendorThing":                  "tool:somevendorthing",
	}
	for in, want := range cases {
		if got := Canonical(in); got != want {
			t.Errorf("Canonical(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCanonicalDropsOpaqueMCPServerIDs(t *testing.T) {
	// Hosts assign opaque connection ids that mean nothing on another machine;
	// the function name is the part worth keeping.
	got := Canonical("mcp__2af2c00c-cf83-4f02-901c-67b30d090f14__slack_read_channel")
	if got != "mcp:slack-read-channel" {
		t.Fatalf("Canonical = %q, want mcp:slack-read-channel", got)
	}
}

func TestNormalizeCommandKeepsShapeNotContent(t *testing.T) {
	cases := map[string]string{
		`git commit -m "fix the cart drawer on mobile"`: "git commit",
		"npm run build":                      "npm run build",
		"npm run test:e2e -- --headed":       "npm run test:e2e",
		"gh pr create --title x --body y":    "gh pr create",
		"cd /Users/me/app && npm install":    "cd && npm install",
		"NODE_ENV=production node server.js": "node",
		"sudo systemctl restart nginx":       "systemctl restart",
		"/usr/bin/env go test ./...":         "go test",
		"shopify theme push --unpublished":   "shopify theme push",
		"":                                   "",
	}
	for in, want := range cases {
		if got := NormalizeCommand(in); got != want {
			t.Errorf("NormalizeCommand(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeCommandCapsCompoundLines(t *testing.T) {
	got := NormalizeCommand("a && b && c && d && e && f")
	if got != "a && b && c && d" {
		t.Fatalf("NormalizeCommand = %q, want the first four segments", got)
	}
}
