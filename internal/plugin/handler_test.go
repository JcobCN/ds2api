package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/auth"
	"ds2api/internal/config"
	"ds2api/internal/deepseek"
)

type testDeepSeek struct{}

func (testDeepSeek) Login(_ context.Context, acc config.Account) (string, error) {
	return "token-for-" + acc.Identifier(), nil
}

func (testDeepSeek) CreateSession(_ context.Context, _ *auth.RequestAuth, _ int) (string, error) {
	return "session-plugin", nil
}

func (testDeepSeek) GetSessionCountForToken(_ context.Context, _ string) (*deepseek.SessionStats, error) {
	return &deepseek.SessionStats{FirstPageCount: 0}, nil
}

func newTestRouter(t *testing.T) (*Store, http.Handler) {
	t.Helper()
	t.Setenv("DS2API_PLUGIN_STATE_PATH", t.TempDir()+"/plugin.json")
	store := LoadStore()
	h := &Handler{Store: store, DS: testDeepSeek{}}
	r := chi.NewRouter()
	r.Route("/plugin", func(pr chi.Router) {
		RegisterRoutes(pr, h)
	})
	return store, r
}

func TestBootstrapLoginStatusAndTest(t *testing.T) {
	store, router := newTestRouter(t)

	bootstrapReq := httptest.NewRequest(http.MethodPost, "/plugin/bootstrap", nil)
	bootstrapReq.RemoteAddr = "127.0.0.1:12345"
	bootstrapRes := httptest.NewRecorder()
	router.ServeHTTP(bootstrapRes, bootstrapReq)
	if bootstrapRes.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d body=%s", bootstrapRes.Code, bootstrapRes.Body.String())
	}
	var bootstrap map[string]any
	if err := json.Unmarshal(bootstrapRes.Body.Bytes(), &bootstrap); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	apiKey, _ := bootstrap["api_key"].(string)
	if apiKey == "" {
		t.Fatal("expected plugin api key")
	}

	body, _ := json.Marshal(map[string]string{
		"email":    "plugin@example.com",
		"password": "secret",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/plugin/login", bytes.NewReader(body))
	loginReq.RemoteAddr = "127.0.0.1:12345"
	loginReq.Header.Set("Authorization", "Bearer "+apiKey)
	loginRes := httptest.NewRecorder()
	router.ServeHTTP(loginRes, loginReq)
	if loginRes.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", loginRes.Code, loginRes.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/plugin/status", nil)
	statusReq.RemoteAddr = "127.0.0.1:12345"
	statusReq.Header.Set("Authorization", "Bearer "+apiKey)
	statusRes := httptest.NewRecorder()
	router.ServeHTTP(statusRes, statusReq)
	if statusRes.Code != http.StatusOK {
		t.Fatalf("status code = %d body=%s", statusRes.Code, statusRes.Body.String())
	}
	var status map[string]any
	if err := json.Unmarshal(statusRes.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if loggedIn, _ := status["logged_in"].(bool); !loggedIn {
		t.Fatalf("expected logged_in true, got %v", status["logged_in"])
	}
	if store.Snapshot().Account.Token == "" {
		t.Fatal("expected stored token after login")
	}

	testReq := httptest.NewRequest(http.MethodPost, "/plugin/test", nil)
	testReq.RemoteAddr = "127.0.0.1:12345"
	testReq.Header.Set("Authorization", "Bearer "+apiKey)
	testRes := httptest.NewRecorder()
	router.ServeHTTP(testRes, testReq)
	if testRes.Code != http.StatusOK {
		t.Fatalf("test code = %d body=%s", testRes.Code, testRes.Body.String())
	}
}

func TestPluginEndpointRejectsNonLoopback(t *testing.T) {
	_, router := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/plugin/bootstrap", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", res.Code, res.Body.String())
	}
}
