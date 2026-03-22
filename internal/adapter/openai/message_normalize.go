package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"ds2api/internal/prompt"
)

func normalizeOpenAIMessagesForPrompt(raw []any, traceID string) []map[string]any {
	_ = traceID
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(asString(msg["role"])))
		switch role {
		case "assistant":
			content := buildAssistantContentForPrompt(msg)
			if content == "" {
				continue
			}
			out = append(out, map[string]any{
				"role":    "assistant",
				"content": content,
			})
		case "tool", "function":
			content := buildToolContentForPrompt(msg)
			out = append(out, map[string]any{
				"role":    "tool",
				"content": content,
			})
		case "user", "system", "developer":
			out = append(out, map[string]any{
				"role":    normalizeOpenAIRoleForPrompt(role),
				"content": normalizeOpenAIContentForPrompt(msg["content"]),
			})
		default:
			content := normalizeOpenAIContentForPrompt(msg["content"])
			if content == "" {
				continue
			}
			if role == "" {
				role = "user"
			}
			out = append(out, map[string]any{
				"role":    normalizeOpenAIRoleForPrompt(role),
				"content": content,
			})
		}
	}
	return out
}

func buildAssistantContentForPrompt(msg map[string]any) string {
	content := normalizeOpenAIContentForPrompt(msg["content"])
	toolCalls := normalizeAssistantToolCallsForPrompt(msg["tool_calls"])
	if toolCalls == "" {
		return strings.TrimSpace(content)
	}
	if strings.TrimSpace(content) == "" {
		return toolCalls
	}
	return strings.TrimSpace(content + "\n" + toolCalls)
}

func normalizeAssistantToolCallsForPrompt(v any) string {
	calls, ok := v.([]any)
	if !ok || len(calls) == 0 {
		return ""
	}
	b, err := json.Marshal(calls)
	if err != nil {
		return strings.TrimSpace(fmt.Sprintf("%v", calls))
	}
	return strings.TrimSpace(string(b))
}

func buildToolContentForPrompt(msg map[string]any) string {
	payload := map[string]any{
		"content": msg["content"],
	}
	if id := strings.TrimSpace(asString(msg["tool_call_id"])); id != "" {
		payload["tool_call_id"] = id
	}
	if id := strings.TrimSpace(asString(msg["id"])); id != "" {
		payload["id"] = id
	}
	if name := strings.TrimSpace(asString(msg["name"])); name != "" {
		payload["name"] = name
	}
	content := normalizeOpenAIContentForPrompt(payload)
	if strings.TrimSpace(content) == "" {
		return `{"content":"null"}`
	}
	return content
}

func normalizeOpenAIContentForPrompt(v any) string {
	return prompt.NormalizeContent(v)
}

func normalizeToolArgumentString(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if looksLikeConcatenatedJSON(trimmed) {
		// Keep original payload to avoid silent argument rewrites.
		return raw
	}
	return trimmed
}

func normalizeOpenAIRoleForPrompt(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "developer" {
		return "system"
	}
	return role
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func looksLikeConcatenatedJSON(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "}{") || strings.Contains(trimmed, "][") {
		return true
	}
	dec := json.NewDecoder(strings.NewReader(trimmed))
	var first any
	if err := dec.Decode(&first); err != nil {
		return false
	}
	var second any
	return dec.Decode(&second) == nil
}
