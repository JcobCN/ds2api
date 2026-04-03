package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/auth"
	"ds2api/internal/config"
	"ds2api/internal/deepseek"
	"ds2api/internal/util"
)

type DeepSeekCaller interface {
	Login(ctx context.Context, acc config.Account) (string, error)
	CreateSession(ctx context.Context, a *auth.RequestAuth, maxAttempts int) (string, error)
	GetSessionCountForToken(ctx context.Context, token string) (*deepseek.SessionStats, error)
}

type Handler struct {
	Store *Store
	DS    DeepSeekCaller
}

func RegisterRoutes(r chi.Router, h *Handler) {
	r.Group(func(pr chi.Router) {
		pr.Use(requireLoopback)
		pr.Post("/bootstrap", h.bootstrap)
		pr.Group(func(ar chi.Router) {
			ar.Use(h.requirePluginKey)
			ar.Post("/login", h.login)
			ar.Get("/status", h.status)
			ar.Post("/logout", h.logout)
			ar.Get("/models", h.models)
			ar.Post("/test", h.test)
		})
	})
}

func requireLoopback(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRequest(r) {
			util.WriteJSON(w, http.StatusForbidden, map[string]any{"detail": "plugin endpoints are restricted to localhost"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLoopbackRequest(r *http.Request) bool {
	host := strings.TrimSpace(r.RemoteAddr)
	if host == "" {
		return true
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func (h *Handler) requirePluginKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractPluginKey(r)
		if token == "" || h.Store == nil || !h.Store.MatchAPIKey(token) {
			util.WriteJSON(w, http.StatusUnauthorized, map[string]any{"detail": "invalid plugin api key"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func extractPluginKey(r *http.Request) string {
	if key := strings.TrimSpace(r.Header.Get("X-Plugin-Key")); key != "" {
		return key
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}

func (h *Handler) bootstrap(w http.ResponseWriter, r *http.Request) {
	state, err := h.Store.Bootstrap()
	if err != nil {
		util.WriteJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{
		"success":         true,
		"api_key":         state.APIKey,
		"base_url":        "/v1",
		"plugin_api_base": "/plugin",
		"logged_in":       state.Account.Identifier() != "",
		"identifier":      state.Account.Identifier(),
	})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Mobile   string `json:"mobile"`
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	acc := config.Account{
		Email:    strings.TrimSpace(req.Email),
		Mobile:   strings.TrimSpace(req.Mobile),
		Password: strings.TrimSpace(req.Password),
	}
	if acc.Identifier() == "" {
		util.WriteJSON(w, http.StatusBadRequest, map[string]any{"detail": "email or mobile is required"})
		return
	}
	if acc.Password == "" {
		util.WriteJSON(w, http.StatusBadRequest, map[string]any{"detail": "password is required"})
		return
	}
	token, err := h.DS.Login(r.Context(), acc)
	if err != nil {
		util.WriteJSON(w, http.StatusBadGateway, map[string]any{"detail": err.Error()})
		return
	}
	acc.Token = token
	if err := h.Store.SetAccount(acc); err != nil {
		util.WriteJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "identifier": acc.Identifier(), "logged_in": true})
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	state := h.Store.Snapshot()
	resp := map[string]any{
		"logged_in":  state.Account.Identifier() != "",
		"identifier": state.Account.Identifier(),
		"has_token":  strings.TrimSpace(state.Account.Token) != "",
	}
	if token := strings.TrimSpace(state.Account.Token); token != "" {
		if stats, err := h.DS.GetSessionCountForToken(r.Context(), token); err == nil && stats != nil {
			resp["session_count"] = stats.FirstPageCount
		}
	}
	util.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) logout(w http.ResponseWriter, _ *http.Request) {
	if err := h.Store.ClearAccount(); err != nil {
		util.WriteJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "logged_in": false})
}

func (h *Handler) models(w http.ResponseWriter, _ *http.Request) {
	util.WriteJSON(w, http.StatusOK, config.OpenAIModelsResponse())
}

func (h *Handler) test(w http.ResponseWriter, r *http.Request) {
	acc, ok := h.Store.GetAccount()
	if !ok {
		util.WriteJSON(w, http.StatusBadRequest, map[string]any{"detail": "plugin account not configured"})
		return
	}
	token := strings.TrimSpace(acc.Token)
	if token == "" {
		var err error
		token, err = h.DS.Login(r.Context(), acc)
		if err != nil {
			util.WriteJSON(w, http.StatusBadGateway, map[string]any{"detail": err.Error()})
			return
		}
		if err := h.Store.UpdateAccountToken(token); err != nil {
			util.WriteJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
			return
		}
	}
	authCtx := &auth.RequestAuth{UsePluginToken: true, DeepSeekToken: token, AccountID: "plugin", Account: acc}
	sessionID, err := h.DS.CreateSession(r.Context(), authCtx, 1)
	if err != nil {
		util.WriteJSON(w, http.StatusBadGateway, map[string]any{"detail": err.Error()})
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "identifier": acc.Identifier(), "session_id": sessionID})
}

func RequirePluginKey(store *Store, r *http.Request) error {
	if store == nil {
		return errors.New("plugin store unavailable")
	}
	token := extractPluginKey(r)
	if token == "" || !store.MatchAPIKey(token) {
		return errors.New("invalid plugin api key")
	}
	return nil
}
