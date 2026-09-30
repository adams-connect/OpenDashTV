# OpenDashTV

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](go.mod)
[![Platform](https://img.shields.io/badge/Platform-Raspberry%20Pi%203B+%20(ARMv7)-red.svg)](#hardware-notes)

OpenDashTV is a production-grade, resource-efficient home dashboard designed to run 24/7 as an appliance on a Raspberry Pi 3B+ (ARMv7, 1GB RAM) driving a 1080p TV in fullscreen kiosk mode.

---

## Key Architectural Principles

1. **Strict Embedded Footprint**: Single compiled Go binary (`dashboard-core`) embedding static assets (`//go:embed web/dist/*`). Operates well below a 75 MB resident RAM budget.
2. **SD Card Endurance**: In-memory caching for widget data and tokens to minimize MicroSD flash wear. Disk writes occur strictly on user-initiated configuration changes or OAuth token renewals.
3. **Low-GPU Kiosk Frontend**: Vanilla TypeScript + CSS bundled via Vite. Zero heavy virtual-DOM runtimes (no React/Vue). Avoids expensive `backdrop-filter` or deep `box-shadow` cascades so the Pi 3B+'s Broadcom VideoCore IV GPU maintains steady 60 FPS composites.
4. **TV Panel Protection**: Programmatic 1–2px sub-pixel CSS translation loop shifts the layout grid every 10 minutes to protect OLED and plasma panels from permanent burn-in. Ambient night mode automatically dims the display during sleep hours.
5. **Resilient Connectivity**: SSE stream with automatic exponential backoff and jitter; widgets cache last-known-good state during home Wi-Fi drops.
6. **Zero-Friction First Run**: If unconfigured, the TV displays an IP URL (`http://<pi-ip>:8080/setup`) and a pairing splash screen, allowing complete setup from a phone on the same Wi-Fi network.

---

## Directory Layout

```
OpenDashTV/
├── cmd/
│   └── dashboard/
│       └── main.go                 # Entrypoint: process signals, graceful drain, config wiring
├── internal/
│   ├── auth/
│   │   ├── google_device.go        # RFC 8628 Device Authorization Flow & backoff poll
│   │   └── token_store.go          # File-based token cache with strict POSIX permissions (0600)
│   ├── config/
│   │   ├── config.go               # YAML parser, environment overrides, coordinate validation
│   │   └── config_test.go          # Concrete test cases (missing fields, edge-case coords)
│   ├── server/
│   │   ├── router.go               # HTTP routing, static file embedding, safe config handler
│   │   ├── sse.go                  # SSE hub with slow-client disconnect handling
│   │   └── handlers_setup.go       # Setup portal endpoints (/setup, /api/config)
│   └── widgets/
│       ├── widget.go               # Core Widget interface (Name, Poll, Interval)
│       ├── clock.go                # Drift-free local system clock ticker
│       ├── weather.go              # Open-Meteo REST client with backoff on network drop
│       ├── gmail.go                # Gmail readonly client (unread count + recent snippets)
│       ├── calendar.go             # Google Calendar agenda client (next 5 agenda events)
│       └── traffic.go              # Commute duration calculator with fallback estimates
├── web/
│   ├── index.html                  # High-contrast 1080p kiosk layout (24px+ baseline)
│   ├── setup.html                  # Responsive phone setup screen
│   ├── src/
│   │   ├── main.ts                 # SSE consumer, targeted DOM patching
│   │   ├── burnin.ts               # Sub-pixel translation loop to prevent TV image retention
│   │   └── styles.css              # Low-GPU CSS (no expensive blurs/box-shadow cascades)
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts              # Bundles static build into web/dist
├── deploy/
│   ├── kiosk.sh                    # Chromium kiosk launcher with Pi 3B+ GPU tuning flags
│   └── opendashtv.service          # systemd unit file with automatic restart on crash
├── config.example.yaml             # Extensively annotated human-friendly example
├── Makefile                        # Multi-arch cross-compilation targets (Pi 3B+ armv7)
└── README.md                       # Pragmatic quickstart, hardware notes, and contribution guide
```

---

## Quickstart

### 1. Configuration

Copy the annotated template and customize your settings:

```bash
cp config.example.yaml config.yaml
```

All values can also be passed via environment variables (prefixed with `OPENDASH_`), e.g.:

```bash
export OPENDASH_SERVER_PORT=8080
export OPENDASH_LOCATION_LATITUDE=41.8781
export OPENDASH_LOCATION_LONGITUDE=-87.6298
export OPENDASH_LOCATION_UNITS=imperial
```

### 2. Building & Testing Locally

```bash
# Run unit tests
make test

# Build native binary
make build

# Run dashboard core
./bin/dashboard-core
```

### 3. Cross-Compiling for Raspberry Pi 3B+

Build the statically compiled ARMv7 32-bit binary from any machine:

```bash
make build-pi
# Binary output: bin/dashboard-core-armv7
```

If your Pi runs 64-bit Raspberry Pi OS:

```bash
make build-pi64
# Binary output: bin/dashboard-core-arm64
```

---

## Deployment on Raspberry Pi

1. **Copy binary and configuration**:
   ```bash
   sudo mkdir -p /opt/opendashtv/bin
   sudo cp bin/dashboard-core-armv7 /opt/opendashtv/bin/dashboard-core
   sudo cp config.example.yaml /opt/opendashtv/config.yaml
   sudo chmod +x /opt/opendashtv/bin/dashboard-core
   ```

2. **Install and enable systemd service**:
   ```bash
   sudo cp deploy/opendashtv.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable --now opendashtv.service
   ```

3. **Configure Kiosk Display**:
   Ensure `xset` and `unclutter` are installed, then configure the desktop session to run `deploy/kiosk.sh` on graphical boot.

---

## Hardware Notes (Pi 3B+)

- **No Onboard RTC**: The Raspberry Pi 3B+ does not have a battery-backed real-time clock. The `opendashtv.service` unit explicitly waits for `time-sync.target` before starting, ensuring accurate widget agenda calculations and TLS certificate validation.
- **GPU Compositing**: Avoid `backdrop-filter: blur(...)` in custom stylesheets; the Pi 3B+'s VideoCore IV GPU drops frames when executing live blur shaders across a 1080p canvas.
- **Memory Allocation**: In `/boot/config.txt`, allocating `gpu_mem=128` is recommended when driving Chromium in kiosk mode at 1080p.
