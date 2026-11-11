import { useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';

import { attachDripInteraction } from './dripInteraction';

// ==============================|| PUBLIC HOME — DRIP ||============================== //
// The brand mark is the drop: it gathers at the end of a faint line (the tap, barely drawn), falls onto a still surface, and the ripple
// wakes the models lying on it, ring by ring. One beat every 5 s.
//
// Everything moves on CSS keyframes alone, so the prerendered static home page (no app bundle)
// animates the same as the app. The dot field is computed once here; reduced motion shows the drop
// resting above a quiet surface.

// the mark on its 20 px grid (16 x 20), from components/chrome/Logo.jsx
const MARK_D = 'M6.613 0.683A1.75 1.75 0 0 1 9.387 0.683L14.341 7.122A8 8 0 1 1 1.659 7.122ZM12 13a2 2 0 1 0 -4 0a2 2 0 1 0 4 0Z';

const BEAT = 5; // s
const IMPACT = 0.58; // share of the beat when the drop lands
const SPREAD = 0.34; // share of the beat the ripple takes to reach the widest ring
const SURFACE_Y = 236;
const RX_MAX = 236;
const RINGS = [34, 68, 104, 142, 182, 224];

// Models on the surface: perspective rings (flattened ellipses), nearer dots larger. Each wakes as
// the ripple passes it, so its delay follows its ring's radius. One dot is amber.
const DOTS = RINGS.flatMap((rx, k) => {
  const n = 8 + k * 5;
  const ry = rx * 0.26;
  return Array.from({ length: n }, (_, i) => {
    const a = (i / n) * Math.PI * 2 + k * 0.37;
    const depth = (Math.sin(a) + 1) / 2;
    return {
      key: `${k}-${i}`,
      cx: +(240 + rx * Math.cos(a)).toFixed(1),
      cy: +(SURFACE_Y + ry * Math.sin(a)).toFixed(1),
      r: +(0.8 + depth * 1.1).toFixed(2),
      delay: +((IMPACT + SPREAD * (rx / RX_MAX)) * BEAT).toFixed(3),
      warm: k === 3 && i === 5
    };
  });
});

const SPRAY = [
  { cx: 236, cy: 234, r: 1.6, dx: -10, delay: 0 },
  { cx: 244, cy: 234, r: 1.3, dx: 12, delay: 0 },
  { cx: 240, cy: 233, r: 1.1, dx: 2, delay: 0.05 }
];

export default function DripFlow() {
  const { t } = useTranslation();
  const ref = useRef(null);

  // tap / press-and-hold on the surface (dripInteraction.js); the static home page wires the same
  // function with an inline script, since it has no app bundle
  useEffect(() => attachDripInteraction(ref.current), []);

  return (
    <div ref={ref} className="drip" role="img" aria-label={t('home.hero.dripAria')}>
      <style>{DRIP_CSS}</style>
      <svg viewBox="0 0 480 320" fill="none" aria-hidden="true">
        <defs>
          <linearGradient id="drip-stem" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stopColor="currentColor" stopOpacity="0" />
            <stop offset="1" stopColor="currentColor" stopOpacity="0.2" />
          </linearGradient>
        </defs>
        {/* the tap is only hinted at: a hairline that fades in towards the point where the drop forms */}
        <rect className="drip-stem" x="239.6" y="-28" width="0.8" height="46" fill="url(#drip-stem)" />

        {DOTS.map((d) => (
          <circle
            key={d.key}
            className={`drip-dot${d.warm ? ' drip-dot-warm' : ''}`}
            cx={d.cx}
            cy={d.cy}
            r={d.r}
            style={{ animationDelay: `${d.delay}s` }}
          />
        ))}

        {[0, 0.22, 0.48].map((delay, i) => (
          <ellipse
            key={delay}
            className={`drip-ripple${i === 0 ? ' drip-ripple-first' : ''}`}
            cx="240"
            cy={SURFACE_Y}
            rx={RX_MAX}
            ry="62"
            style={{ animationDelay: `${delay}s` }}
          />
        ))}

        {SPRAY.map((s) => (
          <circle
            key={s.dx}
            className="drip-spray"
            cx={s.cx}
            cy={s.cy}
            r={s.r}
            style={{ '--drip-dx': `${s.dx}px`, animationDelay: `${s.delay}s` }}
          />
        ))}

        <g className="drip-drop">
          <path className="drip-mark" transform="scale(1.6)" d={MARK_D} />
        </g>
      </svg>
    </div>
  );
}

// Scoped styles (drip-* namespace). Colours come from the product tokens, so both themes work.
const DRIP_CSS = `
.drip { --drip-ease: cubic-bezier(0.22, 1, 0.36, 1); position: relative; width: 100%; aspect-ratio: 480 / 320; color: var(--foreground); }
.drip svg { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; }
.drip-mark { fill: var(--foreground); fill-rule: evenodd; }

/* The drop grows from the end of the tap: the scale origin is its top, so a small bead appears at
   the tip and swells downward, stretches a neck, lets go and falls. Positions keep its bottom on the
   surface (y 231) when it lands and flattens. */
.drip-drop { transform-box: fill-box; transform-origin: 50% 0%; animation: drip-fall ${BEAT}s infinite; }
@keyframes drip-fall {
  0%   { transform: translate(227px, 18px) scale(0.1, 0.08); opacity: 0; animation-timing-function: ease-out; }
  5%   { transform: translate(227px, 18px) scale(0.16, 0.14); opacity: 1; animation-timing-function: linear; }
  24%  { transform: translate(227px, 18px) scale(0.48, 0.45); animation-timing-function: linear; }
  38%  { transform: translate(227px, 18px) scale(0.82, 0.8); animation-timing-function: ease-in-out; }
  45%  { transform: translate(227px, 18px) scale(1, 1.02); animation-timing-function: ease-in-out; }
  49%  { transform: translate(227px, 20px) scale(0.93, 1.13); animation-timing-function: cubic-bezier(0.55, 0, 0.9, 0.5); }
  ${(IMPACT * 100 - 1).toFixed(1)}% { transform: translate(227px, 194px) scale(0.9, 1.16); opacity: 1; animation-timing-function: ease-out; }
  ${(IMPACT * 100).toFixed(1)}% { transform: translate(227px, 223px) scale(1.5, 0.25); opacity: 1; }
  ${(IMPACT * 100 + 3).toFixed(1)}% { transform: translate(227px, 228px) scale(1.7, 0.1); opacity: 0; }
  100% { transform: translate(227px, 228px) scale(1.7, 0.1); opacity: 0; }
}

.drip-spray { fill: var(--foreground); opacity: 0; animation: drip-spray ${BEAT}s infinite; }
@keyframes drip-spray {
  0%, ${(IMPACT * 100 - 0.5).toFixed(1)}% { opacity: 0; transform: translate(0, 0); }
  ${(IMPACT * 100).toFixed(1)}% { opacity: 1; transform: translate(0, 0); animation-timing-function: cubic-bezier(0.2, 0.7, 0.4, 1); }
  ${(IMPACT * 100 + 6).toFixed(1)}% { opacity: 1; transform: translate(var(--drip-dx), -16px); animation-timing-function: ease-in; }
  ${(IMPACT * 100 + 12).toFixed(1)}% { opacity: 0; transform: translate(calc(var(--drip-dx) * 1.5), 2px); }
  100% { opacity: 0; }
}

.drip-ripple { fill: none; stroke: color-mix(in srgb, var(--foreground) 42%, transparent); stroke-width: 1px; vector-effect: non-scaling-stroke; transform-box: fill-box; transform-origin: center; opacity: 0; animation: drip-ripple ${BEAT}s linear infinite; }
@keyframes drip-ripple {
  0%, ${(IMPACT * 100 - 0.1).toFixed(1)}% { transform: scale(0.02); opacity: 0; }
  ${(IMPACT * 100).toFixed(1)}% { transform: scale(0.02); opacity: 0.8; }
  ${((IMPACT + SPREAD) * 100).toFixed(1)}%, 100% { transform: scale(1); opacity: 0; }
}

.drip-dot { fill: var(--foreground); opacity: 0.16; transform-box: fill-box; transform-origin: center; animation: drip-wake ${BEAT}s linear infinite backwards; }
.drip-dot-warm { fill: var(--warning); }
@keyframes drip-wake {
  0% { opacity: 0.16; transform: scale(1); }
  3% { opacity: 1; transform: scale(1.9); }
  22%, 100% { opacity: 0.16; transform: scale(1); }
}

/* interaction (dripInteraction.js): drops released by a tap or a press-and-hold. While they are in
   flight the CSS loop steps aside; removing .drip-live restarts it from the top. */
.drip svg { cursor: pointer; touch-action: manipulation; -webkit-user-select: none; user-select: none; -webkit-touch-callout: none; }
.drip-live .drip-drop, .drip-live .drip-spray, .drip-live .drip-ripple { animation: none; opacity: 0; }
.drip-live .drip-dot { animation: none; }
.drip-live-ripple { fill: none; stroke: color-mix(in srgb, var(--foreground) 42%, transparent); stroke-width: 1px; vector-effect: non-scaling-stroke; }
.drip-live-spray { fill: var(--foreground); }
.drip-name { font: 500 8.5px ui-monospace, SFMono-Regular, Menlo, monospace; fill: var(--muted-foreground); letter-spacing: -0.01em; pointer-events: none; }

/* the hero text rises in after the first drop has started to gather (used by Hero) */
@keyframes drip-rise { from { opacity: 0; transform: translateY(8px); } }

@media (prefers-reduced-motion: reduce) {
  .drip-drop, .drip-spray, .drip-dot, .drip-ripple { animation: none; }
  .drip svg { cursor: default; }
  .drip-drop { transform: translate(227px, 120px); opacity: 1; }
  .drip-ripple-first { opacity: 0.5; transform: scale(0.55); }
}
`;
