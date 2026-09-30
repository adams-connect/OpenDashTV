/**
 * OpenDashTV Kiosk Frontend Entrypoint
 *
 * Implements lightweight Server-Sent Events (SSE) listener with targeted DOM patching.
 * We deliberately avoid virtual-DOM frameworks (React/Vue) to minimize memory allocations
 * and prevent GC pauses on the Raspberry Pi 3B+ (1GB shared RAM).
 */

import { initBurnInProtection } from './burnin.ts';

interface SafeConfig {
  port: number;
  burn_in_protection: boolean;
  night_mode_enabled: boolean;
  night_mode_start: string;
  night_mode_end: string;
  dim_level: number;
  latitude: number;
  longitude: number;
  city: string;
  units: string;
  is_configured: boolean;
  is_google_connected: boolean;
}

interface WeatherData {
  temperature: number;
  unit: string;
  condition: string;
  high: number;
  low: number;
  last_updated: string;
  cached: boolean;
}

interface TrafficData {
  origin: string;
  destination: string;
  duration_minutes: number;
  status: string;
  last_checked: string;
  cached: boolean;
}

interface ClockData {
  time_str: string;
  date_str: string;
  hour: number;
  minute: number;
}

class DashboardApp {
  private eventSource: EventSource | null = null;
  private reconnectDelay = 1000;
  private maxReconnectDelay = 30000;
  private isDestroyed = false;

  private clockEl: HTMLElement | null = null;
  private dateEl: HTMLElement | null = null;
  private cityEl: HTMLElement | null = null;
  private weatherTempEl: HTMLElement | null = null;
  private weatherConditionEl: HTMLElement | null = null;
  private weatherRangeEl: HTMLElement | null = null;
  private commuteTimeEl: HTMLElement | null = null;
  private commuteStatusEl: HTMLElement | null = null;
  private statusDotEl: HTMLElement | null = null;
  private statusTextEl: HTMLElement | null = null;

  constructor() {
    this.cacheDomElements();
    this.startLocalClockTicker();
    this.fetchInitialConfig();
    this.connectEventStream();
    initBurnInProtection('app-viewport');
  }

  private cacheDomElements() {
    this.clockEl = document.getElementById('clock-display');
    this.dateEl = document.getElementById('date-display');
    this.cityEl = document.getElementById('location-city');
    this.weatherTempEl = document.getElementById('weather-temp');
    this.weatherConditionEl = document.getElementById('weather-condition');
    this.weatherRangeEl = document.getElementById('weather-range');
    this.commuteTimeEl = document.getElementById('commute-time');
    this.commuteStatusEl = document.getElementById('commute-status');
    this.statusDotEl = document.getElementById('connection-dot');
    this.statusTextEl = document.getElementById('connection-text');
  }

  /**
   * Fetch current configuration snapshot immediately on boot
   * so the dashboard displays the active city without waiting for SSE stream.
   */
  private async fetchInitialConfig() {
    try {
      const res = await fetch('/api/config');
      if (res.ok) {
        const cfg: SafeConfig = await res.json();
        this.updateConfig(cfg);
      }
    } catch (err) {
      console.warn('[OpenDashTV] Could not fetch initial config:', err);
    }
  }

  /**
   * Local system clock ticker provides immediate feedback even before network
   * or SSE stream initializes. Aligns directly to whole minute boundaries.
   */
  private startLocalClockTicker() {
    const updateClock = () => {
      const now = new Date();
      if (this.clockEl) {
        this.clockEl.textContent = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false });
      }
      if (this.dateEl) {
        this.dateEl.textContent = now.toLocaleDateString([], { weekday: 'long', month: 'short', day: 'numeric' });
      }

      // Schedule next check right at the next second rollover to avoid drift
      const msUntilNextSecond = 1000 - now.getMilliseconds();
      setTimeout(updateClock, msUntilNextSecond);
    };

    updateClock();
  }

  private connectEventStream() {
    if (this.isDestroyed) return;

    this.eventSource = new EventSource('/events');

    this.eventSource.onopen = () => {
      this.reconnectDelay = 1000;
      this.updateConnectionStatus(true);
    };

    // Listen for live config updates (e.g. from /setup submission)
    this.eventSource.addEventListener('config', (event: MessageEvent) => {
      try {
        const cfg: SafeConfig = JSON.parse(event.data);
        this.updateConfig(cfg);
      } catch (err) {
        console.error('[OpenDashTV] Error parsing config event:', err);
      }
    });

    // Listen for weather updates
    this.eventSource.addEventListener('weather', (event: MessageEvent) => {
      try {
        const weather: WeatherData = JSON.parse(event.data);
        this.updateWeather(weather);
      } catch (err) {
        console.error('[OpenDashTV] Error parsing weather event:', err);
      }
    });

    // Listen for clock updates
    this.eventSource.addEventListener('clock', (event: MessageEvent) => {
      try {
        const clock: ClockData = JSON.parse(event.data);
        if (this.clockEl && clock.time_str) this.clockEl.textContent = clock.time_str;
        if (this.dateEl && clock.date_str) this.dateEl.textContent = clock.date_str;
      } catch (err) {
        console.error('[OpenDashTV] Error parsing clock event:', err);
      }
    });

    // Listen for traffic updates
    this.eventSource.addEventListener('traffic', (event: MessageEvent) => {
      try {
        const traffic: TrafficData = JSON.parse(event.data);
        this.updateTraffic(traffic);
      } catch (err) {
        console.error('[OpenDashTV] Error parsing traffic event:', err);
      }
    });

    this.eventSource.onerror = () => {
      this.updateConnectionStatus(false);
      if (this.eventSource) {
        this.eventSource.close();
        this.eventSource = null;
      }

      // Exponential backoff with jitter to protect the Pi when Wi-Fi recovers
      const jitter = Math.floor(Math.random() * 500);
      const nextDelay = Math.min(this.reconnectDelay * 1.5, this.maxReconnectDelay) + jitter;
      this.reconnectDelay = nextDelay;

      setTimeout(() => this.connectEventStream(), nextDelay);
    };
  }

  private updateConnectionStatus(connected: boolean) {
    if (this.statusDotEl) {
      this.statusDotEl.classList.toggle('disconnected', !connected);
    }
    if (this.statusTextEl) {
      this.statusTextEl.textContent = connected ? 'Live' : 'Reconnecting...';
    }
  }

  public updateConfig(cfg: SafeConfig) {
    if (this.cityEl) {
      if (cfg.city && cfg.city.trim().length > 0) {
        this.cityEl.textContent = cfg.city;
      } else if (!cfg.is_configured) {
        this.cityEl.textContent = 'Setup Required (visit /setup)';
      } else {
        this.cityEl.textContent = `Lat: ${cfg.latitude.toFixed(2)}, Lon: ${cfg.longitude.toFixed(2)}`;
      }
    }
  }

  public updateWeather(w: WeatherData) {
    if (this.weatherTempEl) {
      this.weatherTempEl.textContent = `${Math.round(w.temperature)}${w.unit}`;
    }
    if (this.weatherConditionEl) {
      this.weatherConditionEl.textContent = w.condition;
    }
    if (this.weatherRangeEl) {
      this.weatherRangeEl.textContent = `High: ${Math.round(w.high)}${w.unit} · Low: ${Math.round(w.low)}${w.unit}`;
    }
  }

  public updateTraffic(t: TrafficData) {
    if (this.commuteTimeEl) {
      this.commuteTimeEl.textContent = `${t.duration_minutes} min`;
    }
    if (this.commuteStatusEl) {
      this.commuteStatusEl.textContent = t.status;
    }
  }

  public destroy() {
    this.isDestroyed = true;
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
    }
  }
}

// Bootstrap once DOM content is ready
document.addEventListener('DOMContentLoaded', () => {
  new DashboardApp();
});
