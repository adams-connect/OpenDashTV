package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/adams-connect/OpenDashTV/internal/config"
	"github.com/adams-connect/OpenDashTV/web"
)

// Server coordinates HTTP routing, static asset delivery, and SSE streaming.
type Server struct {
	cfg         *config.Config
	configPath  string
	mu          sync.RWMutex
	mux         *http.ServeMux
	httpServer  *http.Server
	hub         *Hub
	coordinator *Coordinator
}

// NewServer initializes HTTP routing with standard library primitives.
// Avoiding heavy framework middleware keeps binary size low and cold-boot time fast on the Pi 3B+.
func NewServer(cfg *config.Config, configPath string) *Server {
	hub := NewHub()
	s := &Server{
		cfg:        cfg,
		configPath: configPath,
		mux:        http.NewServeMux(),
		hub:        hub,
	}

	s.coordinator = NewCoordinator(s)
	s.routes()

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      s.mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /events", s.handleEvents)
	s.mux.HandleFunc("GET /setup", s.handleSetup)
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("POST /api/config", s.handleSaveConfig)

	// Serve compiled static assets (HTML, CSS, JS) directly from binary
	s.mux.Handle("/", http.FileServer(http.FS(web.DistFS())))
}

// Hub returns the active SSE hub for event broadcasting.
func (s *Server) Hub() *Hub {
	return s.hub
}

// Coordinator returns the background widget scheduler.
func (s *Server) Coordinator() *Coordinator {
	return s.coordinator
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Start begins listening on the configured TCP address.
func (s *Server) Start(ctx context.Context) error {
	if s.coordinator != nil {
		s.coordinator.Start(ctx)
	}

	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("starting HTTP listener on %s: %w", s.httpServer.Addr, err)
	}
	return nil
}

// Shutdown gracefully drains active connections within the provided context deadline.
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutting down HTTP server: %w", err)
	}
	return nil
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	s.hub.HandleEventsWithConnect(w, r, func(c *Client) {
		cfgData, _ := json.Marshal(s.safeConfig())
		s.hub.Send(c, "config", cfgData)
		if s.coordinator != nil {
			s.coordinator.TriggerRefresh(r.Context())
		}
	})
}

func (s *Server) safeConfig() safeConfigResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return safeConfigResponse{
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
}
