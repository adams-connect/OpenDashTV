package widgets

import (
	"context"
	"time"
)

// Widget represents an independent data-producing dashboard component.
// Each widget encapsulates its own polling cadence, network backoff,
// and in-memory fallback state to ensure transient home Wi-Fi drops do not
// leave the TV display with blank or broken interface panels.
type Widget interface {
	// Name returns a unique, lowercase slug for this widget (e.g. "clock", "weather").
	Name() string

	// Interval specifies the duration between scheduled polling cycles.
	Interval() time.Duration

	// Fetch queries the data source and returns a JSON-serializable payload.
	Fetch(ctx context.Context) (interface{}, error)
}
