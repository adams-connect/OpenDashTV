#!/usr/bin/env bash
# OpenDashTV Chromium Kiosk Launcher
# Tuned specifically for Raspberry Pi 3B+ (Broadcom BCM2837, 1GB RAM) driving 1080p.
#
# Flags balance VideoCore IV GPU rasterization with strict memory containment
# to avoid OOM killer terminations during long continuous runs.

set -euo pipefail

# 1. Prevent screen blanking, screensaver, and DPMS sleep on the HDMI output
xset s off
xset -dpms
xset s noblank

# 2. Hide mouse cursor when idle
if command -v unclutter >/dev/null 2>&1; then
    unclutter -idle 0.5 -root &
fi

# 3. Wait for dashboard-core HTTP server to become responsive before launching browser
PORT="${OPENDASH_PORT:-8080}"
TARGET_URL="http://localhost:${PORT}"

echo "Waiting for OpenDashTV server on ${TARGET_URL}..."
while ! curl -s --head "${TARGET_URL}" >/dev/null 2>&1; do
    sleep 0.5
done
echo "OpenDashTV server detected. Launching Chromium kiosk..."

# 4. Clean exit locks left behind by sudden power cuts
sed -i 's/"exited_cleanly":false/"exited_cleanly":true/' ~/.config/chromium/Default/Preferences 2>/dev/null || true
sed -i 's/"exit_type":"Crashed"/"exit_type":"Normal"/' ~/.config/chromium/Default/Preferences 2>/dev/null || true

# 5. Launch Chromium in fullscreen kiosk mode with low-memory & GPU tuning flags:
# - disable-gpu-memory-buffer-video-frames: saves scarce shared RAM on Broadcom GPU
# - enable-gpu-rasterization: offloads 60fps CSS transform animations from CPU
# - check-for-update-interval: prevents background update checks eating bandwidth & CPU
exec chromium-browser \
    --kiosk \
    --noerrdialogs \
    --disable-infobars \
    --check-for-update-interval=31536000 \
    --disable-translate \
    --disable-features=TranslateUI \
    --overscroll-history-navigation=0 \
    --enable-gpu-rasterization \
    --disable-gpu-memory-buffer-video-frames \
    --incognito \
    --disable-pinch \
    --no-first-run \
    --fast \
    --fast-start \
    "${TARGET_URL}"
