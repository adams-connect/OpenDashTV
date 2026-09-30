package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adams-connect/OpenDashTV/internal/config"
	"github.com/adams-connect/OpenDashTV/internal/server"
)

var (
	version = "0.1.0-dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	configPath := flag.String("config", "", "path to YAML configuration file (defaults to config.yaml if present)")
	showVersion := flag.Bool("version", false, "display version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("OpenDashTV %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	targetPath := *configPath
	if targetPath == "" {
		if envPath := os.Getenv("OPENDASH_CONFIG_PATH"); envPath != "" {
			targetPath = envPath
		} else if _, err := os.Stat("config.yaml"); err == nil {
			targetPath = "config.yaml"
		}
	}

	cfg, err := loadConfigSafe(targetPath)
	if err != nil {
		log.Fatalf("Fatal configuration error: %v", err)
	}

	log.Printf("Starting OpenDashTV %s [Target: Raspberry Pi 3B+ Fullscreen Kiosk]", version)
	log.Printf("Listening on %s:%d (ReadTimeout: %v, WriteTimeout: %v)",
		cfg.Server.Host, cfg.Server.Port, cfg.Server.ReadTimeout, cfg.Server.WriteTimeout)

	if cfg.IsLocationConfigured() {
		log.Printf("Location active: lat=%.4f, lon=%.4f (units: %s)",
			cfg.Location.Latitude, cfg.Location.Longitude, cfg.Location.Units)
	} else {
		log.Printf("First-run experience: Location unconfigured. Setup splash screen enabled at http://%s:%d/setup",
			cfg.Server.Host, cfg.Server.Port)
	}

	if cfg.Display.NightMode.Enabled {
		log.Printf("Night mode scheduled: %s - %s (dim level: %d%%)",
			cfg.Display.NightMode.StartTime, cfg.Display.NightMode.EndTime, cfg.Display.NightMode.DimLevel)
	}

	srv := server.NewServer(cfg, targetPath)

	// Start HTTP listener in background goroutine
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("HTTP server failure: %v", err)
		}
	}()

	// Trap process termination signals for graceful drain
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Println("Shutdown signal received; draining active connections...")

	// Enforce 5s shutdown deadline to ensure systemd Restart doesn't stall indefinitely
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Graceful shutdown warning: %v", err)
	}
	log.Println("OpenDashTV stopped cleanly.")
}

func loadConfigSafe(path string) (*config.Config, error) {
	if path == "" {
		log.Println("No configuration file specified; loading appliance defaults.")
		return config.Load("")
	}

	log.Printf("Loading configuration from %s", path)
	cfg, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", path, err)
	}
	return cfg, nil
}
