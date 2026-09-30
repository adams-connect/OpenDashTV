package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/adams-connect/OpenDashTV/internal/config"
)

func TestRouter_Healthz(t *testing.T) {
	cfg := config.DefaultConfig()
	srv := NewServer(&cfg, "")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding healthz response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}
}

func TestRouter_GetConfig_RedactsSecrets(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Google.ClientID = "secret-client-id"
	cfg.Google.ClientSecret = "super-secret-key-12345"
	cfg.Location.City = "Chicago, IL"

	srv := NewServer(&cfg, "")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Verify that secret keys are NOT exposed in JSON response
	rawResp := rec.Body.String()
	if bytes.Contains([]byte(rawResp), []byte("super-secret-key-12345")) {
		t.Fatal("CRITICAL: ClientSecret was leaked in /api/config response!")
	}

	var resp safeConfigResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding config response: %v", err)
	}

	if resp.City != "Chicago, IL" {
		t.Errorf("expected city Chicago, IL, got %s", resp.City)
	}
	if !resp.IsGoogleConnected {
		t.Errorf("expected IsGoogleConnected to be true")
	}
}

func TestRouter_SaveConfig_ValidationAndPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := config.DefaultConfig()
	srv := NewServer(&cfg, configPath)

	// 1. Submit invalid latitude
	badPayload := []byte(`{"latitude": 95.0, "longitude": -87.0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(badPayload))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for lat 95.0, got %d", rec.Code)
	}

	// 2. Submit valid configuration
	validPayload := []byte(`{
		"city": "Denver, CO",
		"latitude": 39.7392,
		"longitude": -104.9903,
		"units": "imperial",
		"origin": "100 Broadway",
		"destination": "Airport"
	}`)
	req = httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(validPayload))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid config save, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. Verify in-memory state updated
	if srv.cfg.Location.City != "Denver, CO" {
		t.Errorf("expected in-memory city Denver, CO, got %s", srv.cfg.Location.City)
	}

	// 4. Verify disk persistence
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("loading saved config from disk: %v", err)
	}
	if loaded.Location.City != "Denver, CO" {
		t.Errorf("expected persisted city Denver, CO, got %s", loaded.Location.City)
	}
	if loaded.Location.Latitude != 39.7392 {
		t.Errorf("expected persisted latitude 39.7392, got %f", loaded.Location.Latitude)
	}
}
