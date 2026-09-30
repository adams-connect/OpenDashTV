/**
 * OLED and Plasma TV Burn-In Protection
 *
 * Continuously displaying high-contrast widgets 24/7 on living room TVs causes
 * permanent phosphor/diode retention. To mitigate this without distracting viewers,
 * this loop applies subtle 1-2 pixel offsets to the root layout container every 10 minutes.
 */

// 8-point orbital drift pattern to distribute pixel fatigue evenly
const DRIFT_OFFSETS: Array<[number, number]> = [
  [0, 0],
  [1, 0],
  [1, 1],
  [0, 1],
  [-1, 1],
  [-1, 0],
  [-1, -1],
  [0, -1],
  [1, -1],
  [2, 0],
  [0, 2],
  [-2, 0],
  [0, -2],
];

export function initBurnInProtection(elementId = 'app-viewport', intervalMs = 10 * 60 * 1000): () => void {
  const target = document.getElementById(elementId);
  if (!target) {
    console.warn(`[BurnIn] Target element #${elementId} not found; skipping protection loop.`);
    return () => {};
  }

  let step = 0;

  const applyDrift = () => {
    const [dx, dy] = DRIFT_OFFSETS[step % DRIFT_OFFSETS.length];
    step++;

    // Use requestAnimationFrame so translation coincides with GPU vertical sync
    window.requestAnimationFrame(() => {
      target.style.transform = `translate(${dx}px, ${dy}px)`;
    });
  };

  // Run initial alignment
  applyDrift();

  const timer = window.setInterval(applyDrift, intervalMs);

  // Return teardown function for clean disposal
  return () => {
    window.clearInterval(timer);
  };
}
