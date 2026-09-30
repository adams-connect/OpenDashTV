package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Units defines the measurement system for weather and commute displays.
type Units string

const (
	UnitsMetric   Units = "metric"
	UnitsImperial Units = "imperial"
)

// Config represents the complete runtime configuration for OpenDashTV.
// Keeping this struct flat and lean helps ensure the resident memory footprint
// stays well below the 75MB target on 1GB Raspberry Pi 3B+ hardware.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Display  DisplayConfig  `yaml:"display"`
	Location LocationConfig `yaml:"location"`
	Google   GoogleConfig   `yaml:"google"`
	Traffic  TrafficConfig  `yaml:"traffic"`
}

type ServerConfig struct {
	// Port is the TCP port the HTTP and SSE server binds to.
	Port int `yaml:"port"`

	// Host defines the bind interface. "0.0.0.0" allows local subnet access (e.g. phone setup portal).
	Host string `yaml:"host"`

	// ReadTimeout and WriteTimeout guard against slow-loris conditions or stalled
	// connections when the local Wi-Fi link fluctuates on embedded hardware.
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type DisplayConfig struct {
	// BurnInProtection enables subtle 1-2px translation shifts every 10 minutes
	// to prevent image retention on OLED and plasma displays running 24/7.
	BurnInProtection bool            `yaml:"burn_in_protection"`
	NightMode        NightModeConfig `yaml:"night_mode"`
}

type NightModeConfig struct {
	Enabled bool `yaml:"enabled"`

	// StartTime and EndTime use 24-hour "HH:MM" wall-clock format.
	// Wall-clock strings avoid timezone translation drift between host OS and browser runtime.
	StartTime string `yaml:"start_time"`
	EndTime   string `yaml:"end_time"`

	// DimLevel represents brightness percentage (0-100) applied during night mode hours.
	DimLevel int `yaml:"dim_level"`
}

type LocationConfig struct {
	// Latitude (-90.0 to 90.0) and Longitude (-180.0 to 180.0) for Open-Meteo queries.
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`

	// Optional human-readable label shown in header (e.g., "Chicago, IL").
	City string `yaml:"city"`

	// Units controls temperature (C vs F), wind speed, and distance formatting.
	Units Units `yaml:"units"`
}

type GoogleConfig struct {
	// ClientID and ClientSecret for Google OAuth 2.0 Device Authorization Grant (RFC 8628).
	// On a headless Pi, device flow allows users to pair via phone at google.com/device.
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	Scopes       []string `yaml:"scopes"`
}

type TrafficConfig struct {
	Origin       string        `yaml:"origin"`
	Destination  string        `yaml:"destination"`
	PollInterval time.Duration `yaml:"poll_interval"`
}

// DefaultConfig provides resilient defaults suited for an out-of-the-box Pi appliance.
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Port:         8080,
			Host:         "0.0.0.0",
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		Display: DisplayConfig{
			BurnInProtection: true,
			NightMode: NightModeConfig{
				Enabled:   true,
				StartTime: "22:00",
				EndTime:   "06:30",
				DimLevel:  30,
			},
		},
		Location: LocationConfig{
			Latitude:  0.0,
			Longitude: 0.0,
			City:      "",
			Units:     UnitsImperial,
		},
		Google: GoogleConfig{
			Scopes: []string{
				"https://www.googleapis.com/auth/calendar.events.readonly",
				"https://www.googleapis.com/auth/gmail.readonly",
			},
		},
		Traffic: TrafficConfig{
			// Default 15m polling: high enough frequency for morning commute awareness
			// while respecting free-tier API quotas and avoiding unnecessary SD card churn.
			PollInterval: 15 * time.Minute,
		},
	}
}

// Load reads a YAML configuration file from disk, applies environment variable overrides,
// and validates all bounds. If path is empty, it returns the default configuration with env overrides applied.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		cleanPath := filepath.Clean(path)
		data, err := os.ReadFile(cleanPath)
		if err != nil {
			return nil, fmt.Errorf("reading config file %s: %w", cleanPath, err)
		}

		// Strict unmarshaling helps catch typos before launching headless on a TV.
		decoder := yaml.NewDecoder(strings.NewReader(string(data)))
		decoder.KnownFields(true)
		if err := decoder.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("parsing config yaml %s: %w", cleanPath, err)
		}
	}

	// Environment variables always override file values to support containerized
	// testing and systemd environment overlays without modifying disk.
	if err := applyEnvOverrides(&cfg); err != nil {
		return nil, fmt.Errorf("applying environment overrides: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating configuration: %w", err)
	}

	return &cfg, nil
}

// Validate checks configuration sanity and embedded hardware constraints.
func (c *Config) Validate() error {
	var errs []string

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Sprintf("server.port must be between 1 and 65535, got %d", c.Server.Port))
	}

	if c.Display.NightMode.Enabled {
		if err := validateTimeFormat(c.Display.NightMode.StartTime); err != nil {
			errs = append(errs, fmt.Sprintf("display.night_mode.start_time: %v", err))
		}
		if err := validateTimeFormat(c.Display.NightMode.EndTime); err != nil {
			errs = append(errs, fmt.Sprintf("display.night_mode.end_time: %v", err))
		}
		if c.Display.NightMode.DimLevel < 0 || c.Display.NightMode.DimLevel > 100 {
			errs = append(errs, fmt.Sprintf("display.night_mode.dim_level must be between 0 and 100, got %d", c.Display.NightMode.DimLevel))
		}
	}

	// Coordinate checks: allow 0,0 only if explicitly unconfigured or Null Island,
	// but strictly enforce the spherical bounds [-90, 90] and [-180, 180].
	if c.Location.Latitude < -90.0 || c.Location.Latitude > 90.0 {
		errs = append(errs, fmt.Sprintf("location.latitude must be between -90.0 and 90.0, got %f", c.Location.Latitude))
	}
	if c.Location.Longitude < -180.0 || c.Location.Longitude > 180.0 {
		errs = append(errs, fmt.Sprintf("location.longitude must be between -180.0 and 180.0, got %f", c.Location.Longitude))
	}

	switch strings.ToLower(string(c.Location.Units)) {
	case string(UnitsMetric), string(UnitsImperial):
		c.Location.Units = Units(strings.ToLower(string(c.Location.Units)))
	case "":
		c.Location.Units = UnitsImperial
	default:
		errs = append(errs, fmt.Sprintf("location.units must be 'metric' or 'imperial', got %q", c.Location.Units))
	}

	// Guard against aggressive polling intervals that could thrash MicroSD cards or exhaust API quotas
	if c.Traffic.PollInterval > 0 && c.Traffic.PollInterval < 1*time.Minute {
		errs = append(errs, fmt.Sprintf("traffic.poll_interval must be at least 1m to prevent quota exhaustion, got %v", c.Traffic.PollInterval))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// IsLocationConfigured returns true if non-zero coordinates or a city name have been set.
// A brand new Pi appliance starts unconfigured so the kiosk can display the setup QR splash screen.
func (c *Config) IsLocationConfigured() bool {
	return (c.Location.Latitude != 0.0 || c.Location.Longitude != 0.0) || strings.TrimSpace(c.Location.City) != ""
}

// IsGoogleConfigured returns true if both OAuth client credentials are provided.
func (c *Config) IsGoogleConfigured() bool {
	return strings.TrimSpace(c.Google.ClientID) != "" && strings.TrimSpace(c.Google.ClientSecret) != ""
}

func validateTimeFormat(t string) error {
	if len(t) != 5 || t[2] != ':' {
		return fmt.Errorf("must be 24-hour format HH:MM, got %q", t)
	}
	_, err := time.Parse("15:04", t)
	if err != nil {
		return fmt.Errorf("invalid 24-hour time %q: %w", t, err)
	}
	return nil
}

// applyEnvOverrides parses OPENDASH_* variables without external reflection heavyweights,
// keeping startup latency low and memory usage predictable.
func applyEnvOverrides(cfg *Config) error {
	if val := os.Getenv("OPENDASH_SERVER_PORT"); val != "" {
		port, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid OPENDASH_SERVER_PORT %q: %w", val, err)
		}
		cfg.Server.Port = port
	}

	if val := os.Getenv("OPENDASH_SERVER_HOST"); val != "" {
		cfg.Server.Host = val
	}

	if val := os.Getenv("OPENDASH_DISPLAY_BURN_IN_PROTECTION"); val != "" {
		parsed, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid OPENDASH_DISPLAY_BURN_IN_PROTECTION %q: %w", val, err)
		}
		cfg.Display.BurnInProtection = parsed
	}

	if val := os.Getenv("OPENDASH_DISPLAY_NIGHT_MODE_ENABLED"); val != "" {
		parsed, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid OPENDASH_DISPLAY_NIGHT_MODE_ENABLED %q: %w", val, err)
		}
		cfg.Display.NightMode.Enabled = parsed
	}

	if val := os.Getenv("OPENDASH_LOCATION_LATITUDE"); val != "" {
		lat, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return fmt.Errorf("invalid OPENDASH_LOCATION_LATITUDE %q: %w", val, err)
		}
		cfg.Location.Latitude = lat
	}

	if val := os.Getenv("OPENDASH_LOCATION_LONGITUDE"); val != "" {
		lon, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return fmt.Errorf("invalid OPENDASH_LOCATION_LONGITUDE %q: %w", val, err)
		}
		cfg.Location.Longitude = lon
	}

	if val := os.Getenv("OPENDASH_LOCATION_CITY"); val != "" {
		cfg.Location.City = val
	}

	if val := os.Getenv("OPENDASH_LOCATION_UNITS"); val != "" {
		cfg.Location.Units = Units(strings.ToLower(val))
	}

	if val := os.Getenv("OPENDASH_GOOGLE_CLIENT_ID"); val != "" {
		cfg.Google.ClientID = val
	}

	if val := os.Getenv("OPENDASH_GOOGLE_CLIENT_SECRET"); val != "" {
		cfg.Google.ClientSecret = val
	}

	if val := os.Getenv("OPENDASH_TRAFFIC_ORIGIN"); val != "" {
		cfg.Traffic.Origin = val
	}

	if val := os.Getenv("OPENDASH_TRAFFIC_DESTINATION"); val != "" {
		cfg.Traffic.Destination = val
	}

	return nil
}
