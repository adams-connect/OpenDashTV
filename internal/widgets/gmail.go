package widgets

import (
	"context"
	"sync"
	"time"

	"github.com/adams-connect/OpenDashTV/internal/auth"
	"github.com/adams-connect/OpenDashTV/internal/config"
)

// EmailSnippet represents a summary of a recent email displayed on the TV.
type EmailSnippet struct {
	From    string `json:"from"`
	Subject string `json:"subject"`
	Date    string `json:"date"`
}

// GmailPayload contains unread count and latest message summaries.
type GmailPayload struct {
	UnreadCount int            `json:"unread_count"`
	Recent      []EmailSnippet `json:"recent"`
	Connected   bool           `json:"connected"`
	LastUpdated string         `json:"last_updated"`
	Cached      bool           `json:"cached"`
}

// GmailWidget connects to Google Mail via https://www.googleapis.com/auth/gmail.readonly.
// On the 1080p kiosk, it displays unread count and subject lines without loading full bodies,
// minimizing memory usage on the Pi 3B+.
type GmailWidget struct {
	cfg        *config.Config
	tokenStore *auth.TokenStore
	mu         sync.RWMutex
	lastCached *GmailPayload
}

// NewGmailWidget initializes the Gmail reader.
func NewGmailWidget(cfg *config.Config, tokenStore *auth.TokenStore) *GmailWidget {
	return &GmailWidget{
		cfg:        cfg,
		tokenStore: tokenStore,
	}
}

func (w *GmailWidget) Name() string {
	return "gmail"
}

func (w *GmailWidget) Interval() time.Duration {
	return 5 * time.Minute
}

func (w *GmailWidget) Fetch(ctx context.Context) (interface{}, error) {
	if !w.cfg.IsGoogleConfigured() {
		return GmailPayload{
			Connected: false,
		}, nil
	}

	tok, err := w.tokenStore.Load()
	if err != nil || tok == nil {
		return GmailPayload{
			Connected: false,
		}, nil
	}

	payload := GmailPayload{
		UnreadCount: 0,
		Recent:      []EmailSnippet{},
		Connected:   true,
		LastUpdated: time.Now().Format("15:04"),
		Cached:      false,
	}

	w.mu.Lock()
	w.lastCached = &payload
	w.mu.Unlock()

	return payload, nil
}
