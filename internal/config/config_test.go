package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("non_existent_config.yaml")
	if err == nil {
		t.Fatal("expected error loading non-existent config file, got nil")
	}
	if !strings.Contains(err.Error(), "reading config file") {
		t.Errorf("expected error message to indicate read failure, got %v", err)
	}
}

func TestLoad_EmptyPathReturnsDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("expected nil error on empty path (defaults), got %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Location.Units != UnitsImperial {
		t.Errorf("expected default imperial units, got %s", cfg.Location.Units)
	}
	if !cfg.Display.BurnInProtection {
		t.Errorf("expected burn in protection enabled by default on 24/7 TV panels")
	}
	if cfg.IsLocationConfigured() {
		t.Errorf("default config should indicate unconfigured location to trigger setup splash")
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.yaml")
	if err := os.WriteFile(badFile, []byte("server:\n  port: [not_an_int"), 0600); err != nil {
		t.Fatalf("writing bad yaml: %v", err)
	}

	_, err := Load(badFile)
	if err == nil {
		t.Fatal("expected error parsing malformed YAML, got nil")
	}
	if !strings.Contains(err.Error(), "parsing config yaml") {
		t.Errorf("expected parsing error, got %v", err)
	}
}

func TestLoad_UnknownFieldsRejected(t *testing.T) {
	tmpDir := t.TempDir()
	badKeyFile := filepath.Join(tmpDir, "unknown_key.yaml")
	yamlContent := `
server:
  port: 8080
  unknown_unsupported_key: 123
`
	if err := os.WriteFile(badKeyFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}

	_, err := Load(badKeyFile)
	if err == nil {
		t.Fatal("expected error on unknown YAML field due to KnownFields(true), got nil")
	}
}

func TestValidate_Coordinates(t *testing.T) {
	tests := []struct {
		name    string
		lat     float64
		lon     float64
		wantErr bool
	}{
		{"Null Island (Valid origin)", 0.0, 0.0, false},
		{"Chicago, IL", 41.8781, -87.6298, false},
		{"Tokyo, Japan", 35.6762, 139.6503, false},
		{"North Pole Boundary", 90.0, 0.0, false},
		{"South Pole Boundary", -90.0, 0.0, false},
		{"International Date Line East", 0.0, 180.0, false},
		{"International Date Line West", 0.0, -180.0, false},
		{"Latitude Exceeded Positive", 90.0001, 0.0, true},
		{"Latitude Exceeded Negative", -90.0001, 0.0, true},
		{"Longitude Exceeded Positive", 0.0, 180.0001, true},
		{"Longitude Exceeded Negative", 0.0, -180.0001, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Location.Latitude = tt.lat
			cfg.Location.Longitude = tt.lon

			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v for coords (%f, %f)", err, tt.wantErr, tt.lat, tt.lon)
			}
		})
	}
}

func TestValidate_Units(t *testing.T) {
	tests := []struct {
		units    Units
		expected Units
		wantErr  bool
	}{
		{Units("metric"), UnitsMetric, false},
		{Units("METRIC"), UnitsMetric, false},
		{Units("imperial"), UnitsImperial, false},
		{Units("IMPERIAL"), UnitsImperial, false},
		{Units(""), UnitsImperial, false}, // empty falls back to imperial
		{Units("celsius"), "", true},
		{Units("kelvin"), "", true},
		{Units("furlongs"), "", true},
	}

	for _, tt := range tests {
		t.Run(string(tt.units), func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Location.Units = tt.units
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() for units %q error = %v, wantErr %v", tt.units, err, tt.wantErr)
			}
			if !tt.wantErr && cfg.Location.Units != tt.expected {
				t.Errorf("expected normalized units %s, got %s", tt.expected, cfg.Location.Units)
			}
		})
	}
}

func TestValidate_NightMode(t *testing.T) {
	tests := []struct {
		name      string
		enabled   bool
		startTime string
		endTime   string
		dimLevel  int
		wantErr   bool
	}{
		{"Disabled with invalid times passes", false, "invalid", "invalid", -10, false},
		{"Standard night shift", true, "22:00", "06:30", 30, false},
		{"Midnight rollover boundary", true, "00:00", "23:59", 50, false},
		{"Invalid hour out of 24h range", true, "25:00", "06:00", 30, true},
		{"Invalid minute format", true, "22:60", "06:00", 30, true},
		{"Missing leading zero", true, "9:00", "06:00", 30, true},
		{"Negative dim level", true, "22:00", "06:00", -1, true},
		{"Dim level exceeding 100", true, "22:00", "06:00", 101, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Display.NightMode.Enabled = tt.enabled
			cfg.Display.NightMode.StartTime = tt.startTime
			cfg.Display.NightMode.EndTime = tt.endTime
			cfg.Display.NightMode.DimLevel = tt.dimLevel

			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() for night mode %s error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestValidate_ServerPort(t *testing.T) {
	tests := []struct {
		port    int
		wantErr bool
	}{
		{80, false},
		{8080, false},
		{65535, false},
		{0, true},
		{-1, true},
		{65536, true},
		{70000, true},
	}

	for _, tt := range tests {
		cfg := DefaultConfig()
		cfg.Server.Port = tt.port
		err := cfg.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("Validate() for port %d error = %v, wantErr %v", tt.port, err, tt.wantErr)
		}
	}
}

func TestValidate_TrafficPollInterval(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Traffic.PollInterval = 10 * time.Second // Too low: potential API spam and flash wear
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for traffic poll interval under 1 minute, got nil")
	}

	cfg.Traffic.PollInterval = 5 * time.Minute
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid 5m traffic poll interval, got %v", err)
	}
}

func TestEnvOverrides_PrecedenceOverFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test_config.yaml")

	yamlContent := `
server:
  port: 8080
  host: "127.0.0.1"
location:
  latitude: 37.7749
  longitude: -122.4194
  units: "metric"
google:
  client_id: "original_client_id"
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}

	// Set environment variables to verify overrides
	t.Setenv("OPENDASH_SERVER_PORT", "9090")
	t.Setenv("OPENDASH_LOCATION_LATITUDE", "40.7128")
	t.Setenv("OPENDASH_LOCATION_LONGITUDE", "-74.0060")
	t.Setenv("OPENDASH_LOCATION_UNITS", "imperial")
	t.Setenv("OPENDASH_GOOGLE_CLIENT_ID", "overridden_client_id")
	t.Setenv("OPENDASH_GOOGLE_CLIENT_SECRET", "super_secret_key")
	t.Setenv("OPENDASH_DISPLAY_BURN_IN_PROTECTION", "false")

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port overridden to 9090, got %d", cfg.Server.Port)
	}
	if cfg.Location.Latitude != 40.7128 {
		t.Errorf("expected lat overridden to 40.7128, got %f", cfg.Location.Latitude)
	}
	if cfg.Location.Longitude != -74.0060 {
		t.Errorf("expected lon overridden to -74.0060, got %f", cfg.Location.Longitude)
	}
	if cfg.Location.Units != UnitsImperial {
		t.Errorf("expected units overridden to imperial, got %s", cfg.Location.Units)
	}
	if cfg.Google.ClientID != "overridden_client_id" {
		t.Errorf("expected google client_id overridden, got %s", cfg.Google.ClientID)
	}
	if cfg.Google.ClientSecret != "super_secret_key" {
		t.Errorf("expected google client_secret overridden, got %s", cfg.Google.ClientSecret)
	}
	if cfg.Display.BurnInProtection != false {
		t.Errorf("expected burn_in_protection overridden to false, got true")
	}
	if !cfg.IsGoogleConfigured() {
		t.Errorf("expected IsGoogleConfigured() to be true when both client_id and secret are present")
	}
}

func TestEnvOverrides_InvalidTypesReturnError(t *testing.T) {
	t.Setenv("OPENDASH_SERVER_PORT", "not_a_number")

	_, err := Load("")
	if err == nil {
		t.Fatal("expected error when OPENDASH_SERVER_PORT is not an integer, got nil")
	}
	if !strings.Contains(err.Error(), "invalid OPENDASH_SERVER_PORT") {
		t.Errorf("expected error string to cite invalid OPENDASH_SERVER_PORT, got %v", err)
	}
}

func TestIsLocationConfigured(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.IsLocationConfigured() {
		t.Fatal("default config should not be considered configured")
	}

	cfg.Location.City = "Seattle, WA"
	if !cfg.IsLocationConfigured() {
		t.Fatal("setting City should mark location as configured")
	}

	cfg.Location.City = ""
	cfg.Location.Latitude = 47.6062
	if !cfg.IsLocationConfigured() {
		t.Fatal("setting non-zero coordinate should mark location as configured")
	}
}
