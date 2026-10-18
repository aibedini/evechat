// WCAG contrast audit over the shared token set. Run in CI.
// Thresholds: 4.5:1 for normal text, 3:1 for large/bold text and UI borders.
const tokens = {
  "light text on light bg": ["#0f172a", "#ffffff", 4.5],
  "muted text on light bg": ["#64748b", "#ffffff", 4.5],
  "brand text on white": ["#2563eb", "#ffffff", 4.5],
  "white on brand": ["#ffffff", "#2563eb", 4.5],
  "white on brand-700": ["#ffffff", "#1d4ed8", 4.5],
  "light text on slate-50": ["#0f172a", "#f1f5f9", 4.5],
  "dark text on dark bg": ["#e2e8f0", "#0b1220", 4.5],
  "dark muted on dark bg": ["#94a3b8", "#0b1220", 4.5],
  "white on danger": ["#ffffff", "#dc2626", 4.5],
  "amber text on amber-50": ["#92400e", "#fef3c7", 4.5],
  "emerald text on emerald-50": ["#047857", "#ecfdf5", 4.5],
  "focus ring vs white": ["#b45309", "#ffffff", 3],
};

function channel(v) {
  const c = v / 255;
  return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
}

function luminance(hex) {
  const h = hex.replace("#", "");
  const r = parseInt(h.slice(0, 2), 16);
  const g = parseInt(h.slice(2, 4), 16);
  const b = parseInt(h.slice(4, 6), 16);
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
}

function ratio(a, b) {
  const la = luminance(a);
  const lb = luminance(b);
  const light = Math.max(la, lb);
  const dark = Math.min(la, lb);
  return (light + 0.05) / (dark + 0.05);
}

let failures = 0;
for (const [name, [fg, bg, min]] of Object.entries(tokens)) {
  const r = ratio(fg, bg);
  const ok = r >= min;
  if (!ok) failures++;
  console.log(`${ok ? "✓" : "✗"} ${name}: ${r.toFixed(2)}:1 (min ${min})`);
}
if (failures > 0) {
  console.error(`\n${failures} contrast pair(s) below WCAG AA`);
  process.exit(1);
}
console.log("\nall contrast pairs pass WCAG AA");
