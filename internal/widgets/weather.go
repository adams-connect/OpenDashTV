package widgets

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/adams-connect/OpenDashTV/internal/config"
)

// WeatherPayload contains formatted weather conditions rendered on the kiosk.
type WeatherPayload struct {
	Temperature float64 `json:"temperature"`
	Unit        string  `json:"unit"` // "°F" or "°C"
	Condition   string  `json:"condition"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	LastUpdated string  `json:"last_updated"`
	Cached      bool    `json:"cached"`
}

// WeatherWidget queries Open-Meteo's public forecast API without requiring an API key.
// It maintains an in-memory last-known-good cache to ensure home Wi-Fi drops do not leave
// the TV with a blank weather card.
type WeatherWidget struct {
	cfg        *config.Config
	httpClient *http.Client
	mu         sync.RWMutex
	lastCached *WeatherPayload
}

// NewWeatherWidget initializes the weather provider.
func NewWeatherWidget(cfg *config.Config) *WeatherWidget {
	return &WeatherWidget{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (w *WeatherWidget) Name() string {
	return "weather"
}

func (w *WeatherWidget) Interval() time.Duration {
	return 15 * time.Minute
}

func (w *WeatherWidget) Fetch(ctx context.Context) (interface{}, error) {
	if !w.cfg.IsLocationConfigured() {
		return WeatherPayload{
			Condition: "Setup Location",
			Unit:      "°",
		}, nil
	}

	tempUnitParam := "fahrenheit"
	unitSymbol := "°F"
	if w.cfg.Location.Units == config.UnitsMetric {
		tempUnitParam = "celsius"
		unitSymbol = "°C"
	}

	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,weather_code&daily=temperature_2m_max,temperature_2m_min&temperature_unit=%s&timezone=auto",
		w.cfg.Location.Latitude,
		w.cfg.Location.Longitude,
		tempUnitParam,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return w.fallbackOrError(fmt.Errorf("creating weather request: %w", err))
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return w.fallbackOrError(fmt.Errorf("fetching open-meteo weather: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return w.fallbackOrError(fmt.Errorf("open-meteo returned status %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return w.fallbackOrError(fmt.Errorf("reading open-meteo response: %w", err))
	}

	type openMeteoResponse struct {
		Current struct {
			Temperature float64 `json:"temperature_2m"`
			WeatherCode int     `json:"weather_code"`
		} `json:"current"`
		Daily struct {
			Max []float64 `json:"temperature_2m_max"`
			Min []float64 `json:"temperature_2m_min"`
		} `json:"daily"`
	}

	var parsed openMeteoResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return w.fallbackOrError(fmt.Errorf("decoding weather payload: %w", err))
	}

	high, low := 0.0, 0.0
	if len(parsed.Daily.Max) > 0 {
		high = parsed.Daily.Max[0]
	}
	if len(parsed.Daily.Min) > 0 {
		low = parsed.Daily.Min[0]
	}

	payload := WeatherPayload{
		Temperature: parsed.Current.Temperature,
		Unit:        unitSymbol,
		Condition:   weatherCodeToDescription(parsed.Current.WeatherCode),
		High:        high,
		Low:         low,
		LastUpdated: time.Now().Format("15:04"),
		Cached:      false,
	}

	w.mu.Lock()
	w.lastCached = &payload
	w.mu.Unlock()

	return payload, nil
}

func (w *WeatherWidget) fallbackOrError(err error) (interface{}, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.lastCached != nil {
		cachedCopy := *w.lastCached
		cachedCopy.Cached = true
		return cachedCopy, nil
	}
	return nil, err
}

func weatherCodeToDescription(code int) string {
	switch code {
	case 0:
		return "Clear Sky"
	case 1, 2, 3:
		return "Partly Cloudy"
	case 45, 48:
		return "Foggy"
	case 51, 53, 55:
		return "Drizzle"
	case 61, 63, 65:
		return "Rain"
	case 71, 73, 75:
		return "Snow"
	case 95, 96, 99:
		return "Thunderstorm"
	default:
		return "Fair"
	}
}
