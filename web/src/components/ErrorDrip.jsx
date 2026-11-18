import PropTypes from 'prop-types';

// ==============================|| ERROR DRIP ||============================== //
// The home page's drip, told as what went wrong. Same pieces: the tap barely drawn as a hairline,
// the brand drop, a still surface whose dots are the models. Each scene plays once and rests.
//
//   missing  the drop carries the requested address and falls into a patch of water with no models
//            on it; the ripple runs out and wakes nothing, the address is left on the surface
//   crash    the drop lands hard and splashes; the models stay asleep except the one amber dot
//   update   a new drop is gathering at the tap: reload to let it fall
//
// Colours come from the product tokens, so both themes work. Reduced motion shows the last frame.

// the mark on its 20 px grid (16 x 20), from components/chrome/Logo.jsx
const MARK_D = 'M6.613 0.683A1.75 1.75 0 0 1 9.387 0.683L14.341 7.122A8 8 0 1 1 1.659 7.122ZM12 13a2 2 0 1 0 -4 0a2 2 0 1 0 4 0Z';

const CX = 240;
const SURFACE_Y = 176;
const RINGS = [30, 60, 92, 126, 162, 200];
// models on the surface: perspective rings, nearer dots larger (as on the home page)
const ringDots = (from) =>
  RINGS.flatMap((rx, k) => {
    if (k < from) return [];
    const n = 8 + k * 5;
    const ry = rx * 0.26;
    return Array.from({ length: n }, (_, i) => {
      const a = (i / n) * Math.PI * 2 + k * 0.37;
      const depth = (Math.sin(a) + 1) / 2;
      return {
        key: `${k}-${i}`,
        cx: +(CX + rx * Math.cos(a)).toFixed(1),
        cy: +(SURFACE_Y + ry * Math.sin(a)).toFixed(1),
        r: +(0.8 + depth * 1.1).toFixed(2),
        warm: k === 2 && i === 13
      };
    });
  });
const FULL_FIELD = ringDots(0);
// the missing scene keeps only the far rings: the middle of the surface holds nothing
const EMPTY_FIELD = ringDots(3);

const SPLASH = [
  { x: -22, y: -2, r: 1.6 },
  { x: -12, y: 5, r: 1.1 },
  { x: 15, y: -3, r: 1.4 },
  { x: 26, y: 4, r: 1 },
  { x: 4, y: 7, r: 0.9 }
];

const shorten = (s, max = 34) => (s.length > max ? `${s.slice(0, max - 1)}…` : s);

export default function ErrorDrip({ variant, label, ariaLabel }) {
  const field = variant === 'missing' ? EMPTY_FIELD : FULL_FIELD;

  return (
    <div className={`ed ed-${variant}`} role="img" aria-label={ariaLabel}>
      <style>{ED_CSS}</style>
      <svg viewBox="0 0 480 236" fill="none" aria-hidden="true">
        <defs>
          <linearGradient id="ed-stem" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stopColor="currentColor" stopOpacity="0" />
            <stop offset="1" stopColor="currentColor" stopOpacity="0.2" />
          </linearGradient>
        </defs>
        <rect x="239.6" y="-8" width="0.8" height="28" fill="url(#ed-stem)" />

        {field.map((d) => (
          <circle key={d.key} className={d.warm ? 'ed-dot ed-dot-warm' : 'ed-dot'} cx={d.cx} cy={d.cy} r={d.r} />
        ))}

        {variant === 'missing' && <ellipse className="ed-rest" cx={CX} cy={SURFACE_Y} rx="22" ry="5.7" />}
        {variant === 'missing' &&
          [0, 0.24].map((delay) => (
            <ellipse key={delay} className="ed-ripple" cx={CX} cy={SURFACE_Y} rx="200" ry="52" style={{ animationDelay: `${delay}s` }} />
          ))}

        {variant === 'crash' && (
          <>
            <ellipse className="ed-puddle" cx={CX} cy={SURFACE_Y - 1} rx="15" ry="2.6" />
            {SPLASH.map((s) => (
              <circle
                key={s.x}
                className="ed-splash"
                cx={CX}
                cy={SURFACE_Y - 2}
                r={s.r}
                style={{ '--ed-x': `${s.x}px`, '--ed-y': `${s.y}px` }}
              />
            ))}
          </>
        )}

        {variant === 'missing' && label && (
          <text className="ed-label" x={CX} y={SURFACE_Y + 26} textAnchor="middle">
            {shorten(label)}
          </text>
        )}

        <g className="ed-drop">
          <path className="ed-mark" transform="scale(1.2)" d={MARK_D} />
        </g>
      </svg>
    </div>
  );
}

ErrorDrip.propTypes = {
  variant: PropTypes.oneOf(['missing', 'crash', 'update']).isRequired,
  // missing: the requested address, written on the surface where the drop fell
  label: PropTypes.string,
  ariaLabel: PropTypes.string
};

// The drop group is 19.2 x 24 (the mark at 1.2x) with its scale origin at the top, so it grows from
// the tap's tip (x 240, y 20); x 230.4 centres it. Its bottom meets the surface (y 176) on impact.
const ED_CSS = `
.ed { --ed-ease: cubic-bezier(0.22, 1, 0.36, 1); position: relative; width: 100%; aspect-ratio: 480 / 236; color: var(--foreground); }
.ed svg { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; }
.ed-mark { fill: var(--foreground); fill-rule: evenodd; }
.ed-drop { transform-box: fill-box; transform-origin: 50% 0%; animation: 2.6s 0.2s both; }
.ed-dot { fill: var(--foreground); opacity: 0.16; transform-box: fill-box; transform-origin: center; }
.ed-dot-warm { fill: var(--warning); }

/* missing: gathers, falls into the empty middle, the ripple wakes nothing, the address is left */
.ed-missing .ed-drop { animation-name: ed-fall; }
@keyframes ed-fall {
  0%   { transform: translate(230.4px, 20px) scale(0.12, 0.1); opacity: 0; animation-timing-function: ease-out; }
  8%   { transform: translate(230.4px, 20px) scale(0.2, 0.18); opacity: 1; animation-timing-function: linear; }
  40%  { transform: translate(230.4px, 20px) scale(1, 1.02); animation-timing-function: ease-in-out; }
  46%  { transform: translate(230.4px, 22px) scale(0.93, 1.13); animation-timing-function: cubic-bezier(0.55, 0, 0.9, 0.5); }
  63%  { transform: translate(230.4px, 148px) scale(0.9, 1.16); opacity: 1; animation-timing-function: ease-out; }
  64%  { transform: translate(230.4px, 170px) scale(1.5, 0.25); opacity: 1; }
  70%, 100% { transform: translate(230.4px, 174px) scale(1.7, 0.1); opacity: 0; }
}
.ed-ripple { fill: none; stroke: color-mix(in srgb, var(--foreground) 42%, transparent); stroke-width: 1px; vector-effect: non-scaling-stroke; transform-box: fill-box; transform-origin: center; animation: ed-ripple 2.6s 0.2s linear both; }
@keyframes ed-ripple {
  0%, 63.9% { transform: scale(0.02); opacity: 0; }
  64% { transform: scale(0.02); opacity: 0.8; }
  100% { transform: scale(1); opacity: 0; }
}
.ed-label { font: 500 11px ui-monospace, SFMono-Regular, Menlo, monospace; fill: var(--muted-foreground); animation: ed-label 2.6s 0.2s var(--ed-ease) both; }
.ed-rest { fill: none; stroke: color-mix(in srgb, var(--foreground) 36%, transparent); stroke-width: 1px; vector-effect: non-scaling-stroke; transform-box: fill-box; transform-origin: center; animation: ed-rest 2.6s 0.2s var(--ed-ease) both; }
@keyframes ed-rest {
  0%, 63.9% { transform: scale(0.1); opacity: 0; }
  64% { transform: scale(0.1); opacity: 1; }
  90%, 100% { transform: scale(1); opacity: 1; }
}
.ed-missing .ed-dot { opacity: 0.22; }
@keyframes ed-label {
  0%, 68% { opacity: 0; transform: translateY(-4px); }
  88%, 100% { opacity: 1; transform: translateY(0); }
}

/* crash: already formed, falls hard, splashes; only the amber model lights and stays lit */
.ed-crash .ed-drop { animation-name: ed-drop-hard; animation-duration: 1.4s; }
@keyframes ed-drop-hard {
  0%   { transform: translate(230.4px, 20px) scale(1, 1.02); opacity: 0; animation-timing-function: ease-out; }
  12%  { transform: translate(230.4px, 20px) scale(1, 1.02); opacity: 1; animation-timing-function: ease-in-out; }
  20%  { transform: translate(230.4px, 22px) scale(0.93, 1.13); animation-timing-function: cubic-bezier(0.55, 0, 0.9, 0.5); }
  50%  { transform: translate(230.4px, 148px) scale(0.9, 1.16); opacity: 1; }
  52%, 100% { transform: translate(230.4px, 172px) scale(1.6, 0.18); opacity: 0; }
}
.ed-puddle { fill: var(--foreground); opacity: 0.85; transform-box: fill-box; transform-origin: center; animation: ed-puddle 1.4s 0.2s var(--ed-ease) both; }
@keyframes ed-puddle {
  0%, 50% { transform: scale(0.2, 1); opacity: 0; }
  60%, 100% { transform: scale(1); opacity: 0.85; }
}
.ed-splash { fill: var(--foreground); animation: ed-splash 1.4s 0.2s both; }
@keyframes ed-splash {
  0%, 50% { opacity: 0; transform: translate(0, 0); }
  51% { opacity: 1; transform: translate(0, 0); animation-timing-function: cubic-bezier(0.2, 0.7, 0.4, 1); }
  62% { transform: translate(calc(var(--ed-x) * 0.7), -14px); animation-timing-function: ease-in; }
  74%, 100% { opacity: 0.55; transform: translate(var(--ed-x), var(--ed-y)); }
}
.ed-crash .ed-dot-warm { animation: ed-alarm 1.4s 0.2s var(--ed-ease) both; }
@keyframes ed-alarm {
  0%, 60% { opacity: 0.16; transform: scale(1); }
  76% { opacity: 1; transform: scale(2.2); }
  100% { opacity: 1; transform: scale(2); }
}

/* update: a new drop gathers at the tap and hangs there */
.ed-update .ed-drop { animation-name: ed-gather; animation-duration: 2.2s; animation-timing-function: var(--ed-ease); }
@keyframes ed-gather {
  0%   { transform: translate(230.4px, 20px) scale(0.12, 0.1); opacity: 0; }
  12%  { transform: translate(230.4px, 20px) scale(0.2, 0.18); opacity: 1; }
  100% { transform: translate(230.4px, 20px) scale(0.86, 0.9); opacity: 1; }
}

@media (prefers-reduced-motion: reduce) {
  .ed * { animation-duration: 1ms !important; animation-delay: 0s !important; }
}
`;
