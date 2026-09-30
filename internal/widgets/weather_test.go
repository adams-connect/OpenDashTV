package widgets

import (
	"context"
	"testing"

	"github.com/adams-connect/OpenDashTV/internal/config"
)

func TestWeatherWidget_UnconfiguredLocation(t *testing.T) {
	cfg := config.DefaultConfig()
	widget := NewWeatherWidget(&cfg)

	res, err := widget.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() returned error: %v", err)
	}

	payload, ok := res.(WeatherPayload)
	if !ok {
		t.Fatalf("expected WeatherPayload, got %T", res)
	}

	if payload.Condition != "Setup Location" {
		t.Errorf("expected 'Setup Location', got %q", payload.Condition)
	}
}

func TestWeatherCodeToDescription(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{0, "Clear Sky"},
		{1, "Partly Cloudy"},
		{45, "Foggy"},
		{61, "Rain"},
		{71, "Snow"},
		{95, "Thunderstorm"},
		{999, "Fair"},
	}

	for _, c := range cases {
		got := weatherCodeToDescription(c.code)
		if got != c.want {
			t.Errorf("code %d: got %q, want %q", c.code, got, c.want)
		}
	}
}
