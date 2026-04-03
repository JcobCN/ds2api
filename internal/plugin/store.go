package plugin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ds2api/internal/config"
)

type State struct {
	APIKey    string         `json:"api_key,omitempty"`
	Account   config.Account `json:"account,omitempty"`
	UpdatedAt int64          `json:"updated_at,omitempty"`
}

type Store struct {
	mu    sync.RWMutex
	path  string
	state State
}

func LoadStore() *Store {
	s := &Store{path: config.PluginStatePath()}
	if raw, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(raw, &s.state)
	}
	return s
}

func (s *Store) Snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *Store) Path() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.path
}

func (s *Store) Bootstrap() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(s.state.APIKey) == "" {
		key, err := generateAPIKey()
		if err != nil {
			return State{}, err
		}
		s.state.APIKey = key
		s.state.UpdatedAt = time.Now().Unix()
		if err := s.saveLocked(); err != nil {
			return State{}, err
		}
	}
	return s.state, nil
}

func (s *Store) MatchAPIKey(candidate string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return subtleEqual(strings.TrimSpace(candidate), strings.TrimSpace(s.state.APIKey))
}

func (s *Store) GetAccount() (config.Account, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state.Account.Identifier() == "" {
		return config.Account{}, false
	}
	return s.state.Account, true
}

func (s *Store) SetAccount(acc config.Account) error {
	if acc.Identifier() == "" {
		return errors.New("missing email or mobile")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Account = acc
	s.state.UpdatedAt = time.Now().Unix()
	return s.saveLocked()
}

func (s *Store) ClearAccount() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Account = config.Account{}
	s.state.UpdatedAt = time.Now().Unix()
	return s.saveLocked()
}

func (s *Store) UpdateAccountToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Account.Identifier() == "" {
		return errors.New("plugin account not configured")
	}
	s.state.Account.Token = strings.TrimSpace(token)
	s.state.UpdatedAt = time.Now().Unix()
	return s.saveLocked()
}

func (s *Store) ClearToken() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Account.Token = ""
	s.state.UpdatedAt = time.Now().Unix()
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	body, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir plugin state dir: %w", err)
		}
	}
	return os.WriteFile(s.path, body, 0o644)
}

func generateAPIKey() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "dsplug_" + hex.EncodeToString(buf), nil
}

func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
