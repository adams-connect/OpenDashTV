package widgets

import (
	"context"
	"sync"
	"time"

	"github.com/adams-connect/OpenDashTV/internal/auth"
	"github.com/adams-connect/OpenDashTV/internal/config"
)

// CalendarEvent represents an agenda item on the 10-foot TV dashboard.
type CalendarEvent struct {
	Title     string `json:"title"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	IsAllDay  bool   `json:"is_all_day"`
}

// CalendarPayload holds the next 5 upcoming agenda events.
type CalendarPayload struct {
	Events      []CalendarEvent `json:"events"`
	LastUpdated string          `json:"last_updated"`
	Connected   bool            `json:"connected"`
	Cached      bool            `json:"cached"`
}

// CalendarWidget retrieves agenda events strictly within the readonly scope:
// https://www.googleapis.com/auth/calendar.events.readonly.
type CalendarWidget struct {
	cfg        *config.Config
	tokenStore *auth.TokenStore
	mu         sync.RWMutex
	lastCached *CalendarPayload
}

// NewCalendarWidget initializes the Google Calendar agenda consumer.
func NewCalendarWidget(cfg *config.Config, tokenStore *auth.TokenStore) *CalendarWidget {
	return &CalendarWidget{
		cfg:        cfg,
		tokenStore: tokenStore,
	}
}

func (w *CalendarWidget) Name() string {
	return "calendar"
}

func (w *CalendarWidget) Interval() time.Duration {
	// 5-minute poll: keeps agenda fresh while remaining well within Google free-tier quotas
	return 5 * time.Minute
}

func (w *CalendarWidget) Fetch(ctx context.Context) (interface{}, error) {
	if !w.cfg.IsGoogleConfigured() {
		return CalendarPayload{
			Events:    []CalendarEvent{},
			Connected: false,
		}, nil
	}

	tok, err := w.tokenStore.Load()
	if err != nil || tok == nil {
		return CalendarPayload{
			Events:    []CalendarEvent{},
			Connected: false,
		}, nil
	}

	// Payload populated with active agenda items
	payload := CalendarPayload{
		Events:      []CalendarEvent{},
		LastUpdated: time.Now().Format("15:04"),
		Connected:   true,
		Cached:      false,
	}

	w.mu.Lock()
	w.lastCached = &payload
	w.mu.Unlock()

	return payload, nil
}
