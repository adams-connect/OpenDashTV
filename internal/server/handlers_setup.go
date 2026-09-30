package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/adams-connect/OpenDashTV/internal/config"
	"github.com/adams-connect/OpenDashTV/web"
	"gopkg.in/yaml.v3"
)

type safeConfigResponse struct {
	Port              int     `json:"port"`
	BurnInProtection  bool    `json:"burn_in_protection"`
	NightModeEnabled  bool    `json:"night_mode_enabled"`
	NightModeStart    string  `json:"night_mode_start"`
	NightModeEnd      string  `json:"night_mode_end"`
	DimLevel          int     `json:"dim_level"`
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
	City              string  `json:"city"`
	Units             string  `json:"units"`
	IsConfigured      bool    `json:"is_configured"`
	IsGoogleConnected bool    `json:"is_google_connected"`
}

type setupFormRequest struct {
	City        string  `json:"city"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Units       string  `json:"units"`
	Origin      string  `json:"origin"`
	Destination string  `json:"destination"`
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	resp := safeConfigResponse{
		Port:              s.cfg.Server.Port,
		BurnInProtection:  s.cfg.Display.BurnInProtection,
		NightModeEnabled:  s.cfg.Display.NightMode.Enabled,
		NightModeStart:    s.cfg.Display.NightMode.StartTime,
		NightModeEnd:      s.cfg.Display.NightMode.EndTime,
		DimLevel:          s.cfg.Display.NightMode.DimLevel,
		Latitude:          s.cfg.Location.Latitude,
		Longitude:         s.cfg.Location.Longitude,
		City:              s.cfg.Location.City,
		Units:             string(s.cfg.Location.Units),
		IsConfigured:      s.cfg.IsLocationConfigured(),
		IsGoogleConnected: s.cfg.IsGoogleConfigured(),
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req setupFormRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Malformed JSON payload"})
		return
	}

	// Validate bounds before accepting
	if req.Latitude < -90.0 || req.Latitude > 90.0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Latitude must be between -90 and 90"})
		return
	}
	if req.Longitude < -180.0 || req.Longitude > 180.0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Longitude must be between -180 and 180"})
		return
	}

	unitStr := strings.ToLower(strings.TrimSpace(req.Units))
	if unitStr != string(config.UnitsMetric) && unitStr != string(config.UnitsImperial) {
		unitStr = string(config.UnitsImperial)
	}

	s.mu.Lock()
	s.cfg.Location.City = strings.TrimSpace(req.City)
	s.cfg.Location.Latitude = req.Latitude
	s.cfg.Location.Longitude = req.Longitude
	s.cfg.Location.Units = config.Units(unitStr)
	if req.Origin != "" {
		s.cfg.Traffic.Origin = strings.TrimSpace(req.Origin)
	}
	if req.Destination != "" {
		s.cfg.Traffic.Destination = strings.TrimSpace(req.Destination)
	}
	updatedCfg := *s.cfg
	configPath := s.configPath
	s.mu.Unlock()

	// Persist atomically to disk if a config file location is designated
	if configPath != "" {
		if err := saveConfigToDisk(configPath, &updatedCfg); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to write config: %v", err)})
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "configuration saved"})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	// 1. Try serving from embedded binary assets
	if data, err := fs.ReadFile(web.DistFS(), "setup.html"); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}

	// 2. Fallback to local disk file for live frontend development
	setupPath := filepath.Join("web", "setup.html")
	if data, err := os.ReadFile(setupPath); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}

	// 3. Fallback minimal HTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!DOCTYPE html><html><body><h1>OpenDashTV Setup</h1><p>setup.html not found</p></body></html>`))
}

func saveConfigToDisk(path string, cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}

	tmpPath := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing temp config: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming config file: %w", err)
	}
	return nil
}
