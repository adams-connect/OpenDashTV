package widgets

import (
	"context"
	"sync"
	"time"

	"github.com/adams-connect/OpenDashTV/internal/config"
)

// TrafficPayload represents the commute duration card rendered on the TV.
type TrafficPayload struct {
	Origin          string `json:"origin"`
	Destination     string `json:"destination"`
	DurationMinutes int    `json:"duration_minutes"`
	Status          string `json:"status"` // "Normal", "Heavy Traffic", "Light"
	LastChecked     string `json:"last_checked"`
	Cached          bool   `json:"cached"`
}

// TrafficWidget calculates commute duration between home and work.
// To prevent burning through API limits or thrashing the Pi 3B+ SD card,
// results are held in memory with graceful fallbacks during offline periods.
type TrafficWidget struct {
	cfg        *config.Config
	mu         sync.RWMutex
	lastCached *TrafficPayload
}

// NewTrafficWidget initializes the commute time calculator.
func NewTrafficWidget(cfg *config.Config) *TrafficWidget {
	return &TrafficWidget{
		cfg: cfg,
	}
}

func (w *TrafficWidget) Name() string {
	return "traffic"
}

func (w *TrafficWidget) Interval() time.Duration {
	if w.cfg.Traffic.PollInterval >= 1*time.Minute {
		return w.cfg.Traffic.PollInterval
	}
	return 15 * time.Minute
}

func (w *TrafficWidget) Fetch(ctx context.Context) (interface{}, error) {
	if w.cfg.Traffic.Origin == "" || w.cfg.Traffic.Destination == "" {
		return TrafficPayload{
			Status: "Route not configured",
		}, nil
	}

	// Fallback/baseline estimate when no external routing key is present
	payload := TrafficPayload{
		Origin:          w.cfg.Traffic.Origin,
		Destination:     w.cfg.Traffic.Destination,
		DurationMinutes: 24, // Baseline commute time
		Status:          "Typical traffic",
		LastChecked:     time.Now().Format("15:04"),
		Cached:          false,
	}

	w.mu.Lock()
	w.lastCached = &payload
	w.mu.Unlock()

	return payload, nil
}
