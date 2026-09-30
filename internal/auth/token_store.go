package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Token represents the persisted OAuth2 token credentials.
type Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
}

// TokenStore manages file-backed credential caching with strict 0600 file permissions.
// An in-memory cache layer prevents repetitive SD card read cycles on the Raspberry Pi.
type TokenStore struct {
	path string
	mu   sync.RWMutex
	cached *Token
}

// NewTokenStore initializes a store targeting the given file path.
func NewTokenStore(path string) *TokenStore {
	return &TokenStore{
		path: path,
	}
}

// Load retrieves the cached token, reading from disk only on cold start.
func (s *TokenStore) Load() (*Token, error) {
	s.mu.RLock()
	if s.cached != nil {
		tok := *s.cached
		s.mu.RUnlock()
		return &tok, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cached != nil {
		tok := *s.cached
		return &tok, nil
	}

	cleanPath := filepath.Clean(s.path)
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No token stored yet
		}
		return nil, fmt.Errorf("reading token cache %s: %w", cleanPath, err)
	}

	var tok Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, fmt.Errorf("decoding token cache %s: %w", cleanPath, err)
	}

	s.cached = &tok
	return &tok, nil
}

// Save persists the token to disk with atomic write and 0600 POSIX permissions.
func (s *TokenStore) Save(tok *Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("creating token directory %s: %w", filepath.Dir(s.path), err)
	}

	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling token: %w", err)
	}

	// Atomic write: write to temp file then rename, preventing corrupted state on power failure
	tmpFile := fmt.Sprintf("%s.tmp.%d", s.path, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("writing temporary token file %s: %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, s.path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("persisting token file %s: %w", s.path, err)
	}

	s.cached = tok
	return nil
}
