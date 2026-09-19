package ingest

import (
	"path/filepath"
	"regexp"
	"strings"
)

// harnessInternal are tools that belong to the harness rather than to the
// work: schema loaders, structured-output shims, background-task plumbing,
// session management. They appear in thousands of transcripts, they correlate
// with nothing a human decided, and left in they dominate every mined
// sequence. Returning an empty verb drops the call.
var harnessInternal = map[string]struct{}{
	"toolsearch": {}, "structuredoutput": {}, "getdynamictools": {}, "calldynamictool": {},
	"taskoutput": {}, "taskstop": {}, "killshell": {}, "bashoutput": {}, "monitor": {},
	"listmcpresourcestool": {}, "readmcpresourcetool": {}, "readmcpresourcedirtool": {},
	"listplugins": {}, "searchplugins": {}, "listskills": {}, "searchskills": {},
	"exitplanmode": {}, "enterplanmode": {}, "exitworktree": {}, "enterworktree": {},
	"sendmessage": {}, "listagents": {}, "reportfindings": {}, "schedulewakeup": {},
	"todoread": {}, "attempt_completion": {}, "new_task": {},
}

// uuidish matches the opaque server identifiers some hosts assign to MCP
// connections. They are stable within one machine and meaningless everywhere
// else, so the function name is kept and the server identifier is dropped.
var uuidish = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$|^[0-9a-f]{16,}$`)

// Canonical maps a vendor tool name onto a shared verb. Without this, the same
// workflow mined from Claude Code and from Codex looks like two different
// workflows, because one calls the file reader "Read" and the other "exec" with
// a cat. The verb set is deliberately small: it exists to make sequences
// comparable, not to describe a tool faithfully.
func Canonical(tool string) string {
	t := strings.ToLower(strings.TrimSpace(tool))
	if t == "" {
		return ""
	}
	// MCP tools keep their server so a workflow that depends on Slack or Notion
	// is not flattened into a generic "mcp" step.
	if strings.HasPrefix(t, "mcp__ccd_") || strings.HasPrefix(t, "mcp__claude_code") {
		return ""
	}
	if strings.HasPrefix(t, "mcp__") {
		parts := strings.SplitN(strings.TrimPrefix(t, "mcp__"), "__", 2)
		server := parts[0]
		if len(parts) == 2 {
			if uuidish.MatchString(server) {
				return "mcp:" + sanitizeSegment(parts[1])
			}
			return "mcp:" + sanitizeSegment(server) + ":" + sanitizeSegment(parts[1])
		}
		if uuidish.MatchString(server) {
			return ""
		}
		return "mcp:" + sanitizeSegment(server)
	}
	if _, internal := harnessInternal[t]; internal {
		return ""
	}
	switch t {
	case "read", "read_file", "readfile", "view", "open_file", "cat", "str_replace_editor_view":
		return "read"
	case "write", "write_file", "create_file", "create", "new_file":
		return "write"
	case "edit", "apply_patch", "str_replace", "strreplace", "str_replace_editor", "multiedit", "multi_edit", "edit_file", "patch", "update_file", "replace", "searchreplace", "search_replace", "write_to_file", "replace_in_file":
		return "edit"
	case "bash", "shell", "exec", "exec_command", "run_terminal_command", "run_command", "terminal", "run", "local_shell", "container.exec", "execute_command":
		return "shell"
	case "grep", "search", "ripgrep", "rg", "search_files", "codebase_search", "file_search", "grep_search":
		return "search"
	case "glob", "list_files", "ls", "list_dir", "list_directory", "find":
		return "list"
	case "webfetch", "web_fetch", "fetch", "fetch_url", "read_url", "browse":
		return "web_fetch"
	case "websearch", "web_search", "google_web_search", "search_web":
		return "web_search"
	case "task", "agent", "dispatch_agent", "spawn_agent", "subagent":
		return "subagent"
	case "todowrite", "todo_write", "todo", "update_plan", "plan", "updatecurrentstep", "update_current_step", "updatetodos", "update_todos", "setplan":
		return "plan"
	case "notebookedit", "notebook_edit":
		return "edit"
	case "askuserquestion", "ask_user", "elicit":
		return "ask"
	case "skill", "use_skill", "invoke_skill":
		return "skill"
	}
	switch {
	case strings.Contains(t, "browser") || strings.Contains(t, "playwright") || strings.Contains(t, "puppeteer") || strings.Contains(t, "chrome"):
		return "browser"
	case strings.Contains(t, "screenshot"):
		return "browser"
	case strings.Contains(t, "git"):
		return "shell"
	}
	return "tool:" + sanitizeSegment(t)
}

var segmentRE = regexp.MustCompile(`[^a-z0-9]+`)

func sanitizeSegment(s string) string {
	s = strings.ToLower(s)
	s = segmentRE.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// multiplexers are commands whose first argument carries the real meaning.
// `git` alone says nothing; `git rebase` says a lot.
var multiplexers = map[string]int{
	"git": 1, "gh": 2, "npm": 1, "pnpm": 1, "yarn": 1, "bun": 1, "npx": 1, "bunx": 1,
	"docker": 1, "kubectl": 1, "cargo": 1, "go": 1, "make": 1, "brew": 1, "terraform": 1,
	"vercel": 1, "supabase": 1, "shopify": 2, "prisma": 1, "drizzle-kit": 1, "wrangler": 1,
	"aws": 2, "gcloud": 2, "flyctl": 1, "fly": 1, "pip": 1, "pip3": 1, "poetry": 1, "uv": 1,
	"deno": 1, "rails": 1, "bundle": 1, "composer": 1, "dotnet": 1, "mvn": 1, "gradle": 1,
	"systemctl": 1, "pm2": 1, "turbo": 1, "nx": 1, "expo": 1, "eas": 1, "firebase": 1,
	"heroku": 1, "railway": 1, "netlify": 1, "stripe": 1, "ritual": 1, "reinstate": 1,
}

// scriptRunners take a script name that is itself the signal: `npm run build`
// and `npm run test` are different workflows.
var scriptRunners = map[string]struct{}{
	"npm": {}, "pnpm": {}, "yarn": {}, "bun": {}, "deno": {}, "turbo": {}, "nx": {},
}

var splitRE = regexp.MustCompile(`\s*(?:&&|\|\||;|\||\n)\s*`)

// NormalizeCommand reduces a shell command line to its workflow shape: the
// program plus the subcommand or script that identifies what it does, with
// flags, paths, and message bodies removed.
//
// A compound line yields the shapes of each segment joined with " && ", capped
// so a 40-command one-liner does not become its own unique fingerprint.
func NormalizeCommand(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	segments := splitRE.Split(raw, -1)
	shapes := make([]string, 0, len(segments))
	for _, seg := range segments {
		if s := normalizeSegment(seg); s != "" {
			shapes = append(shapes, s)
		}
		if len(shapes) >= 4 {
			break
		}
	}
	if len(shapes) == 0 {
		return ""
	}
	return strings.Join(shapes, " && ")
}

var assignmentRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

func normalizeSegment(seg string) string {
	fields := splitFields(seg)
	// Leading VAR=value assignments belong to the environment, not the command.
	for len(fields) > 0 && assignmentRE.MatchString(fields[0]) {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return ""
	}
	prog := filepath.Base(strings.Trim(fields[0], `"'`))
	prog = strings.TrimSuffix(prog, ".exe")
	if prog == "sudo" || prog == "env" || prog == "time" || prog == "nohup" {
		return normalizeSegment(strings.Join(fields[1:], " "))
	}
	if prog == "" {
		return ""
	}

	depth, isMux := multiplexers[prog]
	if !isMux {
		return prog
	}

	parts := []string{prog}
	taken := 0
	for i := 1; i < len(fields) && taken < depth; i++ {
		f := fields[i]
		if strings.HasPrefix(f, "-") {
			continue
		}
		f = strings.Trim(f, `"'`)
		if f == "" || looksLikePath(f) {
			continue
		}
		parts = append(parts, strings.ToLower(f))
		taken++
		// `npm run <script>` — the script name is the workflow, so take one
		// more field than the multiplexer depth would allow.
		if _, ok := scriptRunners[prog]; ok && strings.EqualFold(f, "run") {
			depth++
		}
	}
	return strings.Join(parts, " ")
}

func looksLikePath(s string) bool {
	if s == "." || s == ".." {
		return true
	}
	return strings.ContainsAny(s, "/\\") || strings.HasPrefix(s, "$") || strings.HasPrefix(s, "~")
}

// splitFields splits on whitespace while keeping quoted runs together, so a
// commit message never leaks its words into the command shape.
func splitFields(s string) []string {
	out := make([]string, 0, 8)
	var cur strings.Builder
	var quote rune
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
			cur.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
			cur.WriteRune(r)
		case r == ' ' || r == '\t':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// commandKeys are the argument names different vendors use for the shell
// command string.
var commandKeys = []string{"command", "cmd", "script", "shell_command", "commandLine", "input"}

// ExtractCommand pulls a shell command out of a tool-call argument map.
func ExtractCommand(args map[string]string) string {
	for _, k := range commandKeys {
		if v, ok := args[k]; ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
