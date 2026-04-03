package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ds2api/internal/auth"
)

type sessionReuseAuthStub struct{}

func (sessionReuseAuthStub) Determine(_ *http.Request) (*auth.RequestAuth, error) {
	return &auth.RequestAuth{
		UseConfigToken: false,
		DeepSeekToken:  "direct-token",
		CallerID:       "caller:test",
		TriedAccounts:  map[string]bool{},
	}, nil
}

func (sessionReuseAuthStub) DetermineCaller(_ *http.Request) (*auth.RequestAuth, error) {
	return &auth.RequestAuth{
		UseConfigToken: false,
		DeepSeekToken:  "direct-token",
		CallerID:       "caller:test",
		TriedAccounts:  map[string]bool{},
	}, nil
}

func (sessionReuseAuthStub) Release(_ *auth.RequestAuth) {}

type sessionReuseDSStub struct {
	createCalls int
	lastPayload map[string]any
}

func (m *sessionReuseDSStub) CreateSession(_ context.Context, _ *auth.RequestAuth, _ int) (string, error) {
	m.createCalls++
	return "created-session", nil
}

func (m *sessionReuseDSStub) GetPow(_ context.Context, _ *auth.RequestAuth, _ int) (string, error) {
	return "pow", nil
}

func (m *sessionReuseDSStub) CallCompletion(_ context.Context, _ *auth.RequestAuth, payload map[string]any, _ string, _ int) (*http.Response, error) {
	m.lastPayload = payload
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(
			"data: {\"p\":\"response/content\",\"v\":\"hello\"}\n" +
				"data: [DONE]\n",
		)),
	}, nil
}

func (m *sessionReuseDSStub) DeleteAllSessionsForToken(_ context.Context, _ string) error {
	return nil
}

func TestChatCompletionsReusesProvidedSessionID(t *testing.T) {
	ds := &sessionReuseDSStub{}
	h := &Handler{
		Store: mockOpenAIConfig{wideInput: true},
		Auth:  sessionReuseAuthStub{},
		DS:    ds,
	}

	reqBody := `{
		"model":"deepseek-chat",
		"messages":[{"role":"user","content":"hello"}],
		"stream":false,
		"chat_session_id":"session_123"
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer direct-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if ds.createCalls != 0 {
		t.Fatalf("expected no CreateSession call, got %d", ds.createCalls)
	}
	if got := ds.lastPayload["chat_session_id"]; got != "session_123" {
		t.Fatalf("expected reused chat_session_id, got %#v", got)
	}
}

func TestChatCompletionsCreatesSessionWhenMissing(t *testing.T) {
	ds := &sessionReuseDSStub{}
	h := &Handler{
		Store: mockOpenAIConfig{wideInput: true},
		Auth:  sessionReuseAuthStub{},
		DS:    ds,
	}

	reqBody := `{
		"model":"deepseek-chat",
		"messages":[{"role":"user","content":"hello"}],
		"stream":false
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer direct-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if ds.createCalls != 1 {
		t.Fatalf("expected one CreateSession call, got %d", ds.createCalls)
	}
	if got := ds.lastPayload["chat_session_id"]; got != "created-session" {
		t.Fatalf("expected created chat_session_id, got %#v", got)
	}
}

func TestResponsesReusesProvidedSessionID(t *testing.T) {
	ds := &sessionReuseDSStub{}
	h := &Handler{
		Store: mockOpenAIConfig{wideInput: true},
		Auth:  sessionReuseAuthStub{},
		DS:    ds,
	}

	reqBody := `{
		"model":"deepseek-chat",
		"input":"hello",
		"stream":false,
		"chat_session_id":"session_456"
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer direct-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Responses(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if ds.createCalls != 0 {
		t.Fatalf("expected no CreateSession call, got %d", ds.createCalls)
	}
	if got := ds.lastPayload["chat_session_id"]; got != "session_456" {
		t.Fatalf("expected reused chat_session_id, got %#v", got)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body["id"] == "" {
		t.Fatalf("expected response id in body: %#v", body)
	}
}
