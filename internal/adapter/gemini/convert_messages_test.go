package gemini

import (
	"strings"
	"testing"
)

func TestGeminiMessagesFromRequestPreservesFunctionRoundtrip(t *testing.T) {
	req := map[string]any{
		"contents": []any{
			map[string]any{
				"role": "model",
				"parts": []any{
					map[string]any{
						"functionCall": map[string]any{
							"id":   "call_g1",
							"name": "search_web",
							"args": map[string]any{"query": "ai"},
						},
					},
				},
			},
			map[string]any{
				"role": "user",
				"parts": []any{
					map[string]any{
						"functionResponse": map[string]any{
							"id":       "call_g1",
							"name":     "search_web",
							"response": "ok",
						},
					},
				},
			},
		},
	}

	got := geminiMessagesFromRequest(req)
	if len(got) != 2 {
		t.Fatalf("expected two normalized messages, got %#v", got)
	}
	assistant, _ := got[0].(map[string]any)
	if assistant["role"] != "assistant" {
		t.Fatalf("expected assistant first, got %#v", assistant)
	}
	tc, _ := assistant["tool_calls"].([]any)
	if len(tc) != 1 {
		t.Fatalf("expected one tool call, got %#v", assistant["tool_calls"])
	}
	toolMsg, _ := got[1].(map[string]any)
	if toolMsg["role"] != "tool" || toolMsg["tool_call_id"] != "call_g1" {
		t.Fatalf("expected tool message with call id, got %#v", toolMsg)
	}
}

func TestGeminiMessagesFromRequestPreservesUnknownPartAsRawJSONText(t *testing.T) {
	req := map[string]any{
		"contents": []any{
			map[string]any{
				"role": "user",
				"parts": []any{
					map[string]any{"text": "hello"},
					map[string]any{"inlineData": map[string]any{"mimeType": "image/png"}},
				},
			},
		},
	}

	got := geminiMessagesFromRequest(req)
	if len(got) != 1 {
		t.Fatalf("expected one normalized message, got %#v", got)
	}
	msg, _ := got[0].(map[string]any)
	content, _ := msg["content"].(string)
	if !strings.Contains(content, "hello") || !strings.Contains(content, "inlineData") {
		t.Fatalf("expected unknown part preserved as raw json text, got %q", content)
	}
}
