export interface Palette {
  primary: string;
  text: string;
  background: string;
  launcher: string;
}

const base: Palette = {
  primary: "#2563eb",
  text: "#0f172a",
  background: "#ffffff",
  launcher: "#2563eb",
};

/** All widget CSS lives in one string inside the shadow root. */
export function styles(palette: Partial<Palette> = {}, position = "bottom-left", dark = false): string {
  // A palette with explicit undefined values must not wipe the defaults: a
  // project that only configured "primary" would otherwise render
  // "background: undefined" and end up with a transparent panel.
  const c: Palette = {
    primary: palette.primary || base.primary,
    text: palette.text || base.text,
    background: palette.background || base.background,
    launcher: palette.launcher || base.launcher,
  };
  const side = position === "bottom-right" ? "right" : "left";
  const opposite = position === "bottom-right" ? "left" : "right";
  const bg = dark ? "#0f172a" : c.background;
  const fg = dark ? "#e2e8f0" : c.text;
  const surface = dark ? "#1e293b" : "#f1f5f9";
  const border = dark ? "#334155" : "#e2e8f0";
  const muted = dark ? "#94a3b8" : "#64748b";
  const font = "'EveChatVazirmatn','Vazirmatn',system-ui,-apple-system,'Segoe UI',Tahoma,sans-serif";
  return `
:host{all:initial}
*{box-sizing:border-box;font-family:${font}}
.ec-root{position:fixed;z-index:2147483000;bottom:calc(20px + env(safe-area-inset-bottom,0px));
  ${side}:calc(20px + env(safe-area-inset-${side},0px));direction:rtl;
  --ec-vh:100vh;--ec-vv:100vh;--ec-kb:0px}
.ec-launcher{width:56px;height:56px;border-radius:50%;border:0;cursor:pointer;background:${c.launcher};
  color:#fff;box-shadow:0 8px 24px rgba(15,23,42,.25);display:flex;align-items:center;justify-content:center;
  position:relative}
.ec-launcher:focus-visible,.ec-send:focus-visible,.ec-close:focus-visible,.ec-star:focus-visible{outline:3px solid #b45309;outline-offset:2px}
.ec-launcher svg{width:26px;height:26px}
.ec-badge{position:absolute;top:-4px;${opposite}:-4px;min-width:20px;height:20px;border-radius:10px;
  background:#dc2626;color:#fff;font-size:11px;font-weight:700;display:none;align-items:center;justify-content:center;padding:0 5px}
.ec-badge.on{display:flex}
.ec-panel{position:absolute;bottom:72px;${side}:0;width:380px;height:560px;
  max-height:calc(100vh - 110px);max-height:calc(100dvh - 110px);
  background:${bg};border-radius:18px;box-shadow:0 24px 60px rgba(15,23,42,.28);
  display:none;flex-direction:column;overflow:hidden;border:1px solid ${border};color:${fg}}
.ec-panel.open{display:flex;animation:ec-in .2s ease}
@keyframes ec-in{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:none}}
.ec-head{display:flex;align-items:center;gap:10px;padding:14px 16px;background:${c.primary};color:#fff}
.ec-head-text{flex:1;min-width:0}
.ec-title{font-size:14px;font-weight:700;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.ec-status{font-size:11px;opacity:.85;margin-top:2px}
.ec-close{background:transparent;border:0;color:#fff;cursor:pointer;font-size:20px;line-height:1;padding:4px;opacity:.9}
.ec-banner{display:none;padding:7px 14px;font-size:11.5px;background:#fef3c7;color:#92400e;border-bottom:1px solid #fde68a}
.ec-banner.on{display:block}
.ec-msgs{flex:1;overflow-y:auto;padding:14px;display:flex;flex-direction:column;gap:8px;background:${bg};scrollbar-width:thin}
.ec-msg{max-width:82%;padding:9px 12px;border-radius:14px;font-size:13px;line-height:1.7;white-space:pre-wrap;
  word-break:break-word;position:relative}
.ec-msg.visitor{align-self:flex-start;background:${c.primary};color:#fff;border-end-start-radius:4px}
.ec-msg.agent,.ec-msg.bot{align-self:flex-end;background:${surface};color:${fg};border-end-end-radius:4px}
.ec-msg.system{align-self:center;background:transparent;color:${muted};font-size:11px;padding:2px}
.ec-msg .ec-meta{display:flex;align-items:center;gap:4px;justify-content:flex-end;font-size:10px;opacity:.75;margin-top:4px}
.ec-msg.failed{background:#fee2e2;color:#991b1b;cursor:pointer}
.ec-typing{display:none;padding:0 16px 6px;font-size:11.5px;color:${muted}}
.ec-typing.on{display:block}
.ec-dots span{display:inline-block;width:5px;height:5px;border-radius:50%;background:#94a3b8;margin-inline-start:2px;
  animation:ec-bounce 1s infinite}
.ec-dots span:nth-child(2){animation-delay:.15s}.ec-dots span:nth-child(3){animation-delay:.3s}
@keyframes ec-bounce{0%,60%,100%{transform:translateY(0)}30%{transform:translateY(-4px)}}
.ec-composer{display:flex;align-items:flex-end;gap:8px;padding:10px 12px;border-top:1px solid ${border};background:${bg}}
.ec-input{flex:1;resize:none;border:1px solid ${border};border-radius:12px;padding:9px 12px;font-size:13px;
  max-height:96px;min-height:40px;outline:none;color:${fg};background:${dark ? "#0b1220" : c.background};line-height:1.6}
.ec-input:focus{border-color:${c.primary};box-shadow:0 0 0 3px ${hexA(c.primary, 0.14)}}
.ec-send{border:0;border-radius:12px;background:${c.primary};color:#fff;width:40px;height:40px;cursor:pointer;
  display:flex;align-items:center;justify-content:center}
.ec-send:disabled{opacity:.45;cursor:not-allowed}
.ec-send svg{width:18px;height:18px;transform:scaleX(-1)}
.ec-csat{display:none;margin:8px 12px;padding:12px;border-radius:12px;background:${surface};border:1px solid ${border}}
.ec-csat.on{display:block}
.ec-csat-title{font-size:12.5px;font-weight:600;margin-bottom:8px}
.ec-stars{display:flex;gap:6px;justify-content:center;margin-bottom:8px}
.ec-star{background:transparent;border:0;font-size:22px;cursor:pointer;color:#cbd5e1}
.ec-star.on{color:#f59e0b}
/* Five breakpoints, mobile first: 360 / 480 / 768 / 1024 / 1440. */
@media (max-width:360px){
  .ec-panel{width:100vw}
  .ec-msg{max-width:92%}
  .ec-head{padding:10px 12px}
}
@media (max-width:480px){
  .ec-root{bottom:calc(12px + env(safe-area-inset-bottom,0px));
    ${side}:calc(12px + env(safe-area-inset-${side},0px))}
  .ec-panel{position:fixed;inset:0;width:100%;height:100%;max-height:none;border-radius:0;
    height:var(--ec-vh);height:100dvh}
  /* The keyboard shrinks the visual viewport: pin the panel to it so the
     composer stays above the keyboard instead of hiding behind it. */
  .ec-panel.kb{height:var(--ec-vv);top:0;bottom:auto}
  .ec-msgs{padding-bottom:calc(14px + var(--ec-kb))}
  .ec-composer{padding-bottom:calc(10px + env(safe-area-inset-bottom,0px))}
}
@media (min-width:481px) and (max-width:768px){
  .ec-panel{width:min(420px,calc(100vw - 32px));height:min(620px,calc(100dvh - 110px))}
}
@media (min-width:769px) and (max-width:1024px){
  .ec-panel{width:360px}
}
@media (min-width:1441px){
  .ec-panel{width:400px;height:600px}
}
@media (prefers-reduced-motion:reduce){
  .ec-panel.open{animation:none}
  .ec-dots span{animation:none}
}
`;
}

function hexA(hex: string, alpha: number): string {
  const clean = hex.replace("#", "");
  if (clean.length !== 6) return "rgba(37,99,235,.14)";
  const r = parseInt(clean.slice(0, 2), 16);
  const g = parseInt(clean.slice(2, 4), 16);
  const b = parseInt(clean.slice(4, 6), 16);
  return `rgba(${r},${g},${b},${alpha})`;
}

/** Injects the font-face rule into the host head (Shadow DOM ignores it). */
export function injectFont(apiUrl: string): void {
  if (typeof document === "undefined" || document.getElementById("evechat-font")) return;
  const link = document.createElement("style");
  link.id = "evechat-font";
  link.textContent = `@font-face{font-family:'EveChatVazirmatn';src:url('${apiUrl}/assets/fonts/vazirmatn-variable.woff2') format('woff2');font-weight:100 900;font-display:swap;}`;
  document.head.appendChild(link);
}
