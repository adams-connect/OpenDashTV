package auth

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTokenStore_EmptyStoreReturnsNil(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewTokenStore(filepath.Join(tmpDir, "missing.token"))

	tok, err := store.Load()
	if err != nil {
		t.Fatalf("expected nil error on missing token file, got %v", err)
	}
	if tok != nil {
		t.Fatalf("expected nil token for non-existent file, got %v", tok)
	}
}

func TestTokenStore_SaveAndLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "google_token.json")
	store := NewTokenStore(tokenPath)

	expiry := time.Now().Add(1 * time.Hour).Truncate(time.Second)
	original := &Token{
		AccessToken:  "mock-access-token-xyz",
		TokenType:    "Bearer",
		RefreshToken: "mock-refresh-token-abc",
		Expiry:       expiry,
	}

	if err := store.Save(original); err != nil {
		t.Fatalf("failed to save token: %v", err)
	}

	// 1. Read from in-memory cache
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load token from store: %v", err)
	}
	if loaded.AccessToken != "mock-access-token-xyz" {
		t.Errorf("expected access token 'mock-access-token-xyz', got %q", loaded.AccessToken)
	}

	// 2. Instantiate new store targeting the same file to verify disk persistence
	diskStore := NewTokenStore(tokenPath)
	fromDisk, err := diskStore.Load()
	if err != nil {
		t.Fatalf("failed to load token from fresh store on disk: %v", err)
	}
	if fromDisk.RefreshToken != "mock-refresh-token-abc" {
		t.Errorf("expected refresh token 'mock-refresh-token-abc', got %q", fromDisk.RefreshToken)
	}
}
