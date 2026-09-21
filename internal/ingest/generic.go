package ingest

import (
	"strings"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/session"
)

// The readers below share one record shape because the agents they read share
// one ancestry. Gemini CLI, Qwen Code, Kimi Code, Copilot CLI, and Pi all
// persist a list of messages carrying a role and either prose or a provider
// content array, and all of them borrow either the Gemini `parts` shape or the
// OpenAI `tool_calls` shape for tools.
//
// Writing five nearly identical readers would mean five places to fix when one
// of them adds a field, so the shared shape is handled once, leniently, and the
// per-agent files supply only what genuinely differs: where the records live,
// how a session is delimited, and where the workspace is recorded.

// roleOf normalizes the many spellings of a speaker.
func roleOf(m map[string]any) session.Actor {
	switch strings.ToLower(str(m, "role", "type", "speaker", "author", "sender")) {
	case "user", "human", "input":
		return session.ActorUser
	case "assistant", "model", "gemini", "ai", "agent", "output", "qwen", "kimi", "copilot":
		return session.ActorAssistant
	case "tool", "function", "tool_result", "function_response", "observation":
		return session.ActorTool
	case "system", "developer":
		return session.ActorSystem
	}
	return session.ActorUnknown
}

// appendGenericMessage turns one message-shaped record into turns.
func appendGenericMessage(b *turnBuilder, raw map[string]any, lim Limits, red *redact.Redactor, fallback time.Time) {
	actor := roleOf(raw)
	if actor == session.ActorSystem || actor == session.ActorUnknown {
		// An unknown role with prose is still worth keeping as assistant
		// output; an unknown role with nothing is noise.
		if txt := genericText(raw); txt == "" {
			return
		}
		actor = session.ActorAssistant
	}
	at := timeFrom(raw, "timestamp", "time", "createdAt", "created_at", "ts", "date")
	if at.IsZero() {
		at = fallback
	}

	text := genericText(raw)
	switch actor {
	case session.ActorUser:
		b.message(session.ActorUser, at, CleanPrompt(text))
	case session.ActorTool:
		b.toolResult(at, str(raw, "name", "tool", "toolName"), text, errorFromResult(raw, text))
	default:
		b.message(session.ActorAssistant, at, text)
	}

	appendGenericTools(b, raw, lim, red, at)
}

// genericText pulls prose out of whichever field the vendor used.
func genericText(raw map[string]any) string {
	// Qwen nests the Gemini parts array one level down, under message. Reading
	// only the top level parsed its roles correctly and its text not at all,
	// which yielded sessions with zero turns that were then dropped as empty.
	if msg := obj(raw, "message"); msg != nil {
		if inner := partsText(msg); inner != "" {
			return inner
		}
	}
	if s := partsText(raw); s != "" {
		return s
	}
	if v, ok := raw["content"]; ok {
		if s := textFromContent(v); s != "" {
			return s
		}
	}
	if v, ok := raw["message"]; ok {
		switch t := v.(type) {
		case string:
			return t
		case map[string]any:
			return textFromContent(t["content"])
		}
	}
	return str(raw, "text", "prompt", "value", "output")
}

// partsText renders a Gemini-style parts array.
func partsText(raw map[string]any) string {
	if parts := arr(raw, "parts"); parts != nil {
		var sb strings.Builder
		for _, p := range parts {
			part, ok := p.(map[string]any)
			if !ok {
				continue
			}
			if t := str(part, "text"); t != "" {
				sb.WriteString(t)
				sb.WriteString("\n")
			}
		}
		return strings.TrimSpace(sb.String())
	}
	return ""
}

// appendGenericTools handles the three tool encodings these agents use: the
// Gemini `parts[].functionCall`, the OpenAI `tool_calls[]`, and a flat
// `toolCalls[]` of {name, args}.
func appendGenericTools(b *turnBuilder, raw map[string]any, lim Limits, red *redact.Redactor, at time.Time) {
	// Tools nest exactly where text does. One level only: a record that
	// contained itself would otherwise recurse forever.
	if msg := obj(raw, "message"); msg != nil {
		appendToolParts(b, msg, lim, red, at)
	}
	appendToolParts(b, raw, lim, red, at)
}

// appendToolParts reads the three tool encodings out of one record level.
func appendToolParts(b *turnBuilder, raw map[string]any, lim Limits, red *redact.Redactor, at time.Time) {
	for _, p := range arr(raw, "parts") {
		part, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if call := obj(part, "functionCall"); call != nil {
			b.toolCall(at, str(call, "name"), flattenArgs(call["args"], lim, red))
		}
		if resp := obj(part, "functionResponse"); resp != nil {
			text := textFromContent(resp["response"])
			if text == "" {
				text = str(resp, "response")
			}
			b.toolResult(at, str(resp, "name"), text, errorFromResult(resp, text))
		}
	}

	for _, key := range []string{"tool_calls", "toolCalls", "function_calls", "functionCalls"} {
		for _, c := range arr(raw, key) {
			call, ok := c.(map[string]any)
			if !ok {
				continue
			}
			name := str(call, "name", "tool", "toolName")
			var args map[string]string
			if fn := obj(call, "function"); fn != nil {
				if name == "" {
					name = str(fn, "name")
				}
				args = flattenArgs(fn["arguments"], lim, red)
			}
			if args == nil {
				args = flattenArgs(firstPresent(call, "args", "arguments", "input", "parameters"), lim, red)
			}
			b.toolCall(at, name, args)
		}
	}

	// A record that is itself a tool result, with the payload beside the role.
	if out, ok := raw["tool_response"]; ok {
		text := textFromContent(out)
		b.toolResult(at, str(raw, "name", "tool"), text, errorFromResult(raw, text))
	}
}

// walkMessageArrays finds the message list in a session document, trying the
// field names these agents use in the order they became common.
func walkMessageArrays(doc map[string]any) []any {
	for _, key := range []string{"messages", "history", "chatHistory", "turns", "events", "conversation", "records", "items"} {
		if a := arr(doc, key); len(a) > 0 {
			return a
		}
	}
	// Some stores nest the conversation one level down.
	for _, key := range []string{"session", "state", "data", "chat"} {
		if inner := obj(doc, key); inner != nil {
			if a := walkMessageArrays(inner); len(a) > 0 {
				return a
			}
		}
	}
	return nil
}

// workspaceFrom finds the recorded working directory in a session document.
func workspaceFrom(doc map[string]any) string {
	for _, key := range []string{"cwd", "workspace", "workingDirectory", "working_directory", "projectRoot", "project_root", "directory", "root", "path"} {
		if v := str(doc, key); v != "" && strings.ContainsAny(v, "/\\") {
			return v
		}
	}
	for _, key := range []string{"info", "metadata", "meta", "session", "context", "state"} {
		if inner := obj(doc, key); inner != nil {
			if v := workspaceFrom(inner); v != "" {
				return v
			}
		}
	}
	return ""
}
