package ingest

import (
	"regexp"
	"strings"
)

// harnessTags name the XML-ish blocks every harness injects into the user role:
// reminders, environment dumps, hook output, IDE selection, slash-command
// expansion. None of it was typed by a human, all of it repeats across
// thousands of sessions, and left in place it dominates any clustering that
// treats a user record as a prompt.
var harnessTags = []string{
	"system-reminder", "local-command-caveat", "local-command-stdout", "local-command-stderr",
	"command-name", "command-message", "command-args", "command-contents",
	"user_info", "user-info", "environment_details", "env", "system_info",
	"context", "additional_context", "important_context", "project_context",
	"workspace_info", "session_context", "editor_context", "ide_context",
	"ide_selection", "ide_opened_file", "attached_files", "open_files", "files",
	"git_status", "timestamp", "hook", "user-prompt-submit-hook", "skills_instructions",
	"function_results", "tool_output", "current_working_directory",
	"rules", "memories", "user_rules", "project_rules", "custom_instructions",
	"instructions", "agents_md", "available_skills", "background_information",
}

// leadingBlockRE recognizes a harness block this build has not been taught by
// name. It only fires at the very start of a prompt and only on a tag carrying
// an underscore or hyphen, which is how every harness names these and how no
// ordinary HTML element a human would paste is named. RE2 has no
// backreferences, so the closing tag is located in code rather than in the
// pattern.
var leadingBlockRE = regexp.MustCompile(`(?is)\A<([a-z][a-z0-9]*(?:[_-][a-z0-9]+)+)>`)

// stripLeadingBlock removes one unknown harness block from the front of s and
// reports whether it removed anything. A block with no closing tag consumes the
// rest of the string: a truncated environment dump is not a prompt.
func stripLeadingBlock(s string) (string, bool) {
	m := leadingBlockRE.FindStringSubmatchIndex(s)
	if m == nil {
		return s, false
	}
	tag := s[m[2]:m[3]]
	rest := s[m[1]:]
	closing := "</" + tag + ">"
	if i := strings.Index(strings.ToLower(rest), closing); i >= 0 {
		return strings.TrimSpace(rest[i+len(closing):]), true
	}
	return "", true
}

// stripKnownBlocks removes every named harness block from s.
//
// This is done by scanning for each tag's own closing tag rather than with one
// alternation pattern, because these blocks nest: a <rules> section contains
// <context> subsections, and a non-greedy pattern would stop at the inner
// closing tag and leave the outer block's tail behind. A block whose closing
// tag never arrives — a transcript truncated mid-dump — consumes the rest of
// the string.
func stripKnownBlocks(s string) string {
	for _, tag := range harnessTags {
		open, closing := "<"+tag+">", "</"+tag+">"
		for {
			lower := strings.ToLower(s)
			i := strings.Index(lower, open)
			if i < 0 {
				break
			}
			j := strings.Index(lower[i+len(open):], closing)
			if j < 0 {
				s = s[:i]
				break
			}
			s = s[:i] + " " + s[i+len(open)+j+len(closing):]
		}
	}
	return s
}

var (
	userQueryRE = regexp.MustCompile(`(?s)<user_query>\s*(.*?)\s*(?:</user_query>|\z)`)
	fencedRE    = regexp.MustCompile("(?s)```.*?```")
)

// syntheticPrefixes mark a "user" record the harness wrote on the human's
// behalf: hook output, resumed-session banners, tool feedback. Treating these
// as prompts invents workflows nobody performed.
var syntheticPrefixes = []string{
	"caveat: the messages below were generated",
	"[request interrupted",
	"api error",
	"this session is being continued from a previous conversation",
	"result of calling the",
	"tool ran without output",
	"the user doesn't want to proceed",
	"[tool use was rejected",
	"please continue the conversation from where we left it off",
	"continue from where we left",
	"<no message>",
}

// CleanPrompt strips harness scaffolding from a user turn and returns the prose
// a human actually typed. An empty result means the record was not a prompt.
func CleanPrompt(s string) string {
	if s == "" {
		return ""
	}
	// Cursor wraps the real prompt; when the wrapper is present it is the only
	// part worth keeping.
	if m := userQueryRE.FindStringSubmatch(s); len(m) == 2 && strings.TrimSpace(m[1]) != "" {
		s = m[1]
	}
	s = stripKnownBlocks(s)
	s = strings.TrimSpace(s)
	for i := 0; i < 8; i++ {
		stripped, changed := stripLeadingBlock(s)
		if !changed {
			break
		}
		s = stripped
	}

	lower := strings.ToLower(s)
	for _, p := range syntheticPrefixes {
		if strings.HasPrefix(lower, p) {
			return ""
		}
	}
	return s
}

// IsPromptLike reports whether cleaned text is worth mining as an intent. Very
// short continuations ("yes", "go on") are real human turns but carry no intent
// of their own; the arc segmenter folds them into the preceding arc instead.
func IsPromptLike(s string) bool {
	return len(strings.Fields(s)) >= 3
}

// Gist reduces a prompt to its first meaningful sentence with code fences
// removed. Reports quote it, and a fenced diff pasted into a prompt would
// otherwise become the headline.
func Gist(s string, max int) string {
	s = fencedRE.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		line = strings.Join(strings.Fields(line), " ")
		if len(line) > max {
			line = strings.TrimSpace(line[:max]) + "…"
		}
		return line
	}
	return ""
}
