package server

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/adams-connect/OpenDashTV/internal/widgets"
)

// Coordinator schedules widget polling and streams updates to the SSE hub.
type Coordinator struct {
	server  *Server
	clock   *widgets.ClockWidget
	weather *widgets.WeatherWidget
	traffic *widgets.TrafficWidget
}

// NewCoordinator initializes the background widget scheduler.
func NewCoordinator(s *Server) *Coordinator {
	return &Coordinator{
		server:  s,
		clock:   widgets.NewClockWidget(),
		weather: widgets.NewWeatherWidget(s.cfg),
		traffic: widgets.NewTrafficWidget(s.cfg),
	}
}

// Start begins the scheduling loops for all dashboard widgets.
func (c *Coordinator) Start(ctx context.Context) {
	// 1. Initial immediate broadcast of all widget states
	c.broadcastClock(ctx)
	c.broadcastWeather(ctx)
	c.broadcastTraffic(ctx)

	// 2. Schedule loops
	clockTicker := time.NewTicker(30 * time.Second)
	weatherTicker := time.NewTicker(15 * time.Minute)
	trafficTicker := time.NewTicker(15 * time.Minute)

	go func() {
		defer clockTicker.Stop()
		defer weatherTicker.Stop()
		defer trafficTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-clockTicker.C:
				c.broadcastClock(ctx)
			case <-weatherTicker.C:
				c.broadcastWeather(ctx)
			case <-trafficTicker.C:
				c.broadcastTraffic(ctx)
			}
		}
	}()
}

// TriggerRefresh immediately queries weather and traffic for newly updated coordinates.
func (c *Coordinator) TriggerRefresh(ctx context.Context) {
	go func() {
		refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		c.broadcastWeather(refreshCtx)
		c.broadcastTraffic(refreshCtx)
	}()
}

func (c *Coordinator) broadcastClock(ctx context.Context) {
	if c.server.Hub().ClientCount() == 0 {
		return
	}
	res, err := c.clock.Fetch(ctx)
	if err != nil {
		return
	}
	data, _ := json.Marshal(res)
	c.server.Hub().Broadcast("clock", data)
}

func (c *Coordinator) broadcastWeather(ctx context.Context) {
	if c.server.Hub().ClientCount() == 0 {
		return
	}
	res, err := c.weather.Fetch(ctx)
	if err != nil {
		log.Printf("[Coordinator] Weather fetch error: %v", err)
		return
	}
	data, _ := json.Marshal(res)
	c.server.Hub().Broadcast("weather", data)
}

func (c *Coordinator) broadcastTraffic(ctx context.Context) {
	if c.server.Hub().ClientCount() == 0 {
		return
	}
	res, err := c.traffic.Fetch(ctx)
	if err != nil {
		return
	}
	data, _ := json.Marshal(res)
	c.server.Hub().Broadcast("traffic", data)
}
