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
export function styles(palette: Partial<Palette> = {}, position = "bottom-left"): string {
  const c = { ...base, ...palette };
  const side = position === "bottom-right" ? "right" : "left";
  const opposite = position === "bottom-right" ? "left" : "right";
  return `
:host{all:initial}
*{box-sizing:border-box;font-family:system-ui,-apple-system,"Segoe UI",Tahoma,"Noto Sans Arabic",sans-serif}
.ec-root{position:fixed;z-index:2147483000;bottom:20px;${side}:20px;direction:rtl}
.ec-launcher{width:56px;height:56px;border-radius:50%;border:0;cursor:pointer;background:${c.launcher};
  color:#fff;box-shadow:0 8px 24px rgba(15,23,42,.25);display:flex;align-items:center;justify-content:center;
  transition:transform .18s ease,box-shadow .18s ease;position:relative}
.ec-launcher:hover{transform:translateY(-2px) scale(1.03);box-shadow:0 12px 28px rgba(15,23,42,.3)}
.ec-launcher svg{width:26px;height:26px}
.ec-badge{position:absolute;top:-4px;${opposite}:-4px;min-width:20px;height:20px;border-radius:10px;
  background:#ef4444;color:#fff;font-size:11px;font-weight:700;display:none;align-items:center;justify-content:center;padding:0 5px}
.ec-badge.on{display:flex}
.ec-pulse{position:absolute;inset:0;border-radius:50%;border:2px solid ${c.launcher};opacity:0}
.ec-pulse.on{animation:ec-pulse 1.6s ease-out 2}
@keyframes ec-pulse{0%{transform:scale(1);opacity:.55}100%{transform:scale(1.9);opacity:0}}
.ec-panel{position:absolute;bottom:72px;${side}:0;width:380px;height:560px;max-height:calc(100vh - 110px);
  background:${c.background};border-radius:18px;box-shadow:0 24px 60px rgba(15,23,42,.28);
  display:none;flex-direction:column;overflow:hidden;border:1px solid rgba(15,23,42,.08);color:${c.text}}
.ec-panel.open{display:flex;animation:ec-in .2s ease}
@keyframes ec-in{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:none}}
.ec-head{display:flex;align-items:center;gap:10px;padding:14px 16px;background:${c.primary};color:#fff}
.ec-avatar{width:34px;height:34px;border-radius:50%;background:rgba(255,255,255,.22);display:flex;
  align-items:center;justify-content:center;font-weight:700;font-size:13px}
.ec-head-text{flex:1;min-width:0}
.ec-title{font-size:14px;font-weight:700;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.ec-status{font-size:11px;opacity:.85;margin-top:2px}
.ec-dot{display:inline-block;width:7px;height:7px;border-radius:50%;background:#22c55e;margin-inline-end:5px}
.ec-dot.off{background:#f59e0b}
.ec-close{background:transparent;border:0;color:#fff;cursor:pointer;font-size:20px;line-height:1;padding:4px;opacity:.9}
.ec-banner{display:none;padding:7px 14px;font-size:11.5px;background:#fef3c7;color:#92400e;border-bottom:1px solid #fde68a}
.ec-banner.on{display:block}
.ec-msgs{flex:1;overflow-y:auto;padding:14px;display:flex;flex-direction:column;gap:8px;background:${c.background};
  scrollbar-width:thin}
.ec-msg{max-width:82%;padding:9px 12px;border-radius:14px;font-size:13px;line-height:1.65;white-space:pre-wrap;
  word-break:break-word;position:relative}
.ec-msg.visitor{align-self:flex-start;background:${c.primary};color:#fff;border-end-start-radius:4px}
.ec-msg.agent,.ec-msg.bot{align-self:flex-end;background:#f1f5f9;color:${c.text};border-end-end-radius:4px}
.ec-msg.system{align-self:center;background:transparent;color:#94a3b8;font-size:11px;padding:2px}
.ec-msg .ec-meta{display:flex;align-items:center;gap:4px;justify-content:flex-end;font-size:10px;opacity:.75;margin-top:4px}
.ec-msg.failed{background:#fee2e2;color:#991b1b;cursor:pointer}
.ec-empty{padding:24px 16px;text-align:center;color:#64748b;font-size:13px;line-height:1.9}
.ec-typing{display:none;padding:0 16px 6px;font-size:11.5px;color:#64748b}
.ec-typing.on{display:block}
.ec-dots span{display:inline-block;width:5px;height:5px;border-radius:50%;background:#94a3b8;margin-inline-start:2px;
  animation:ec-bounce 1s infinite}
.ec-dots span:nth-child(2){animation-delay:.15s}.ec-dots span:nth-child(3){animation-delay:.3s}
@keyframes ec-bounce{0%,60%,100%{transform:translateY(0)}30%{transform:translateY(-4px)}}
.ec-composer{display:flex;align-items:flex-end;gap:8px;padding:10px 12px;border-top:1px solid #e2e8f0;background:${c.background}}
.ec-input{flex:1;resize:none;border:1px solid #e2e8f0;border-radius:12px;padding:9px 12px;font-size:13px;
  max-height:96px;min-height:40px;outline:none;color:${c.text};background:${c.background};line-height:1.6}
.ec-input:focus{border-color:${c.primary};box-shadow:0 0 0 3px ${hexA(c.primary, 0.14)}}
.ec-send{border:0;border-radius:12px;background:${c.primary};color:#fff;width:40px;height:40px;cursor:pointer;
  display:flex;align-items:center;justify-content:center}
.ec-send:disabled{opacity:.45;cursor:not-allowed}
.ec-send svg{width:18px;height:18px;transform:scaleX(-1)}
.ec-powered{padding:4px 0 7px;text-align:center;font-size:10px;color:#94a3b8}
.ec-csat{display:none;margin:8px 12px;padding:12px;border-radius:12px;background:#f8fafc;border:1px solid #e2e8f0}
.ec-csat.on{display:block}
.ec-csat-title{font-size:12.5px;font-weight:600;margin-bottom:8px}
.ec-stars{display:flex;gap:6px;justify-content:center;margin-bottom:8px}
.ec-star{background:transparent;border:0;font-size:22px;cursor:pointer;color:#cbd5e1;line-height:1}
.ec-star.on{color:#f59e0b}
.ec-csat textarea{width:100%;border:1px solid #e2e8f0;border-radius:8px;padding:7px;font-size:12px;resize:none;min-height:44px}
.ec-csat button.ec-submit{margin-top:8px;width:100%;border:0;border-radius:8px;padding:8px;background:${c.primary};
  color:#fff;font-size:12.5px;cursor:pointer}
@media (max-width:480px){
  .ec-root{bottom:12px;${side}:12px}
  .ec-panel{position:fixed;inset:0;width:100%;height:100%;max-height:none;border-radius:0}
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
