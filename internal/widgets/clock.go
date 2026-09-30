package widgets

import (
	"context"
	"time"
)

// ClockPayload represents the structured time payload emitted to the kiosk display.
type ClockPayload struct {
	TimeStr string `json:"time_str"` // e.g. "14:05"
	DateStr string `json:"date_str"` // e.g. "Tuesday, Sep 29"
	Hour    int    `json:"hour"`
	Minute  int    `json:"minute"`
}

// ClockWidget provides high-accuracy local time without external API calls.
// It syncs directly with the host Linux system clock, which is synchronized via NTP (systemd-timesyncd).
type ClockWidget struct{}

// NewClockWidget initializes the system clock provider.
func NewClockWidget() *ClockWidget {
	return &ClockWidget{}
}

func (w *ClockWidget) Name() string {
	return "clock"
}

// Interval returns 1 minute.
// Rather than fixed 60s sleep drift, callers should align next tick to the start of the next minute.
func (w *ClockWidget) Interval() time.Duration {
	return 1 * time.Minute
}

func (w *ClockWidget) Fetch(ctx context.Context) (interface{}, error) {
	now := time.Now()
	return ClockPayload{
		TimeStr: now.Format("15:04"),
		DateStr: now.Format("Monday, Jan 2"),
		Hour:    now.Hour(),
		Minute:  now.Minute(),
	}, nil
}
