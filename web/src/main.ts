/**
 * OpenDashTV Kiosk Frontend Entrypoint
 *
 * Implements lightweight Server-Sent Events (SSE) listener with targeted DOM patching.
 * We deliberately avoid virtual-DOM frameworks (React/Vue) to minimize memory allocations
 * and prevent GC pauses on the Raspberry Pi 3B+ (1GB shared RAM).
 */

import { initBurnInProtection } from './burnin.ts';

interface DashboardState {
  timeStr: string;
  dateStr: string;
  weatherTemp?: string;
  weatherCondition?: string;
  commuteDuration?: string;
  unreadEmails?: number;
  calendarEvents?: Array<{ title: string; time: string }>;
  isNightMode?: boolean;
}

class DashboardApp {
  private eventSource: EventSource | null = null;
  private reconnectDelay = 1000;
  private maxReconnectDelay = 30000;
  private isDestroyed = false;

  private clockEl: HTMLElement | null = null;
  private dateEl: HTMLElement | null = null;
  private statusDotEl: HTMLElement | null = null;
  private statusTextEl: HTMLElement | null = null;

  constructor() {
    this.cacheDomElements();
    this.startLocalClockTicker();
    this.connectEventStream();
    initBurnInProtection('app-viewport');
  }

  private cacheDomElements() {
    this.clockEl = document.getElementById('clock-display');
    this.dateEl = document.getElementById('date-display');
    this.statusDotEl = document.getElementById('connection-dot');
    this.statusTextEl = document.getElementById('connection-text');
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
      this.reconnectDelay = 1000; // Reset backoff on successful handshake
      this.updateConnectionStatus(true);
    };

    this.eventSource.onmessage = (event) => {
      try {
        const payload: DashboardState = JSON.parse(event.data);
        this.renderState(payload);
      } catch (err) {
        console.error('[OpenDashTV] Failed to parse SSE event data:', err);
      }
    };

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

  private renderState(state: DashboardState) {
    if (state.isNightMode !== undefined) {
      document.body.classList.toggle('night-mode', state.isNightMode);
    }

    // Defensive DOM updates: modify only changed text nodes to keep composite times <16ms
    if (state.timeStr && this.clockEl && this.clockEl.textContent !== state.timeStr) {
      this.clockEl.textContent = state.timeStr;
    }
    if (state.dateStr && this.dateEl && this.dateEl.textContent !== state.dateStr) {
      this.dateEl.textContent = state.dateStr;
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
