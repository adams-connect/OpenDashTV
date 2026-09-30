package widgets

import (
	"context"
	"testing"
	"time"
)

func TestClockWidget_Fetch(t *testing.T) {
	widget := NewClockWidget()
	if widget.Name() != "clock" {
		t.Fatalf("expected widget name 'clock', got %q", widget.Name())
	}
	if widget.Interval() != 1*time.Minute {
		t.Fatalf("expected 1m interval, got %v", widget.Interval())
	}

	res, err := widget.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() returned error: %v", err)
	}

	payload, ok := res.(ClockPayload)
	if !ok {
		t.Fatalf("expected ClockPayload type, got %T", res)
	}

	if len(payload.TimeStr) != 5 || payload.TimeStr[2] != ':' {
		t.Errorf("expected HH:MM time format, got %q", payload.TimeStr)
	}
	if payload.DateStr == "" {
		t.Errorf("expected non-empty date string")
	}
}
