import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { LogoMark } from '@/components/chrome/Logo';

// ==============================|| PUBLIC HOME — GATEWAY FLOW ||============================== //
// Requests stream in from the left along six curves, gather into the brand drop at the centre and
// fan back out on the right to the model providers. The dashed rings are the checks a request passes
// (organization budget and allow-list on the left, channel routing on the right); a few amber
// particles stop at the outer left ring, the requests that were rate-limited.
//
// Motion, in order: the curves draw in, the drop falls into the hub, then the particles start and
// the hub ripples gently as they pass. The scene tilts a few pixels with the pointer. Under
// prefers-reduced-motion everything is shown at rest: no particles, no ripple, no parallax.
// Particles use SMIL <animateMotion>, so they follow the curves in the SVG's own units at any size.

const W = 480;
const ARC_YS = [30, 82, 134, 186, 238, 290];
const leftArc = (y) => `M0 ${y} C112 ${y} 150 160 240 160`;
const rightArc = (y) => `M240 160 C330 160 368 ${y} 480 ${y}`;
const throughPath = (yIn, yOut) => `${leftArc(yIn)} C330 160 368 ${yOut} 480 ${yOut}`;

const RINGS = [
  { x: 76, rx: 16, ry: 115.3 },
  { x: 158, rx: 9, ry: 45.7 }
];
const mirrorRings = [...RINGS, ...RINGS.map((r) => ({ ...r, x: W - r.x }))];

// Deterministic particle set: each one enters on one curve and leaves on another, with its own
// speed and phase so the stream never looks like a marching row.
const PARTICLES = Array.from({ length: 30 }, (_, i) => {
  const dur = 5.5 + ((i * 37) % 30) / 10; // 5.5–8.4 s
  return {
    id: `p${i}`,
    d: throughPath(ARC_YS[i % 6], ARC_YS[(i * 5 + 2) % 6]),
    dur,
    begin: -((i * 1.37) % dur),
    r: i % 4 === 0 ? 2.6 : 1.8
  };
});

// Rate-limited requests: travel the first part of a left curve, stop at the outer ring and fade.
const BLOCKED = [
  { id: 'b0', y: 82, dur: 6.2, begin: -1.1 },
  { id: 'b1', y: 238, dur: 7.4, begin: -4.6 },
  { id: 'b2', y: 30, dur: 8.6, begin: -2.9 }
];

// Faint background specks at two depths, for the parallax.
const SPECKS = [
  { x: 30, y: 56 },
  { x: 104, y: 40 },
  { x: 40, y: 108 },
  { x: 24, y: 212 },
  { x: 116, y: 68 },
  { x: 196, y: 120 },
  { x: 36, y: 268 },
  { x: 120, y: 250 },
  { x: 190, y: 214 }
].flatMap((p) => [p, { x: W - p.x, y: 320 - p.y }]);

function useReducedMotion() {
  const [reduced, setReduced] = useState(
    () => typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  );
  useEffect(() => {
    const mq = window.matchMedia?.('(prefers-reduced-motion: reduce)');
    if (!mq) return undefined;
    const onChange = () => setReduced(mq.matches);
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  }, []);
  return reduced;
}

export default function GatewayFlow() {
  const { t } = useTranslation();
  const reduced = useReducedMotion();
  const ref = useRef(null);

  // Pointer parallax: write the pointer offset (-1..1) into CSS variables; the layers translate by
  // different amounts. One rAF per frame at most.
  useEffect(() => {
    if (reduced) return undefined;
    const node = ref.current;
    let frame = 0;
    const onMove = (event) => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const x = (event.clientX / window.innerWidth) * 2 - 1;
        const y = (event.clientY / window.innerHeight) * 2 - 1;
        node?.style.setProperty('--gf-x', x.toFixed(3));
        node?.style.setProperty('--gf-y', y.toFixed(3));
      });
    };
    window.addEventListener('pointermove', onMove, { passive: true });
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener('pointermove', onMove);
    };
  }, [reduced]);

  return (
    <div ref={ref} className={`gf${reduced ? ' gf-still' : ''}`} role="img" aria-label={t('home.hero.gatewayAria')}>
      <style>{GF_CSS}</style>

      <svg className="gf-layer gf-far" viewBox="0 0 480 320" fill="none" aria-hidden="true">
        {SPECKS.map((p) => (
          <circle key={`s-${p.x}-${p.y}`} className="gf-speck" cx={p.x} cy={p.y} r="1.4" />
        ))}
      </svg>

      <svg className="gf-layer gf-mid" viewBox="0 0 480 320" fill="none" aria-hidden="true">
        {ARC_YS.map((y, i) => (
          <path key={`l-${y}`} className="gf-line" pathLength="1" d={leftArc(y)} style={{ animationDelay: `${i * 60}ms` }} />
        ))}
        {ARC_YS.map((y, i) => (
          <path key={`r-${y}`} className="gf-line" pathLength="1" d={rightArc(y)} style={{ animationDelay: `${360 + i * 60}ms` }} />
        ))}
        {mirrorRings.map((r) => (
          <ellipse key={`e-${r.x}`} className="gf-ring" cx={r.x} cy="160" rx={r.rx} ry={r.ry} />
        ))}
      </svg>

      {!reduced && (
        <svg className="gf-layer gf-near gf-particles" viewBox="0 0 480 320" fill="none" aria-hidden="true">
          {PARTICLES.map((p) => (
            <circle key={p.id} className="gf-dot" r={p.r}>
              <animateMotion dur={`${p.dur}s`} begin={`${p.begin}s`} repeatCount="indefinite" path={p.d} />
            </circle>
          ))}
          {BLOCKED.map((b) => (
            <circle key={b.id} className="gf-dot gf-dot-warn" r="2.2">
              <animateMotion
                dur={`${b.dur}s`}
                begin={`${b.begin}s`}
                repeatCount="indefinite"
                path={leftArc(b.y)}
                keyPoints="0;0.3;0.3"
                keyTimes="0;0.45;1"
                calcMode="linear"
              />
              <animate
                attributeName="opacity"
                values="1;1;0;0"
                keyTimes="0;0.45;0.6;1"
                dur={`${b.dur}s`}
                begin={`${b.begin}s`}
                repeatCount="indefinite"
              />
            </circle>
          ))}
        </svg>
      )}

      <span className="gf-hub">
        <span className="gf-ripple" aria-hidden="true" />
        <span className="gf-ripple gf-ripple-2" aria-hidden="true" />
        <span className="gf-drop">
          <LogoMark className="gf-mark" />
        </span>
      </span>
    </div>
  );
}

// Scoped styles (gf-* namespace). Colours come from the product tokens, so both themes work.
// One easing curve for everything that enters; loops are linear.
const GF_CSS = `
.gf { --gf-x: 0; --gf-y: 0; --gf-ease: cubic-bezier(0.22, 1, 0.36, 1); position: relative; width: 100%; aspect-ratio: 480 / 320; }
.gf-layer { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; transition: transform 600ms var(--gf-ease); }
.gf-far { transform: translate3d(calc(var(--gf-x) * -10px), calc(var(--gf-y) * -6px), 0); }
.gf-mid { transform: translate3d(calc(var(--gf-x) * 4px), calc(var(--gf-y) * 3px), 0); }
.gf-near { transform: translate3d(calc(var(--gf-x) * 8px), calc(var(--gf-y) * 5px), 0); }

.gf-speck { fill: color-mix(in srgb, var(--foreground) 22%, transparent); }
/* No vector-effect here: with non-scaling-stroke the dash is measured in screen pixels while
   pathLength is in user units, so on a large screen the draw-in stops short of the path's end. */
.gf-line { stroke: color-mix(in srgb, var(--foreground) 22%, transparent); stroke-width: 0.7; stroke-dasharray: 1; stroke-dashoffset: 1; animation: gf-draw 1100ms var(--gf-ease) forwards; }
.gf-ring { stroke: color-mix(in srgb, var(--foreground) 34%, transparent); stroke-width: 1px; vector-effect: non-scaling-stroke; stroke-dasharray: 3 4; opacity: 0; animation: gf-fade 700ms var(--gf-ease) 700ms forwards; }

.gf-particles { opacity: 0; animation: gf-fade 900ms var(--gf-ease) 1300ms forwards; }
.gf-dot { fill: var(--foreground); }
.gf-dot-warn { fill: var(--warning); }

.gf-hub { position: absolute; left: 50%; top: 50%; width: 12.5%; aspect-ratio: 1; transform: translate(-50%, -50%); }
.gf-drop { position: absolute; inset: 0; border-radius: 50%; background: var(--card); box-shadow: 0 0 0 1px var(--border); display: grid; place-items: center; opacity: 0; transform: translateY(-36px) scale(0.9); animation: gf-drop 800ms var(--gf-ease) 500ms forwards; }
.gf-mark { width: 34%; height: auto; color: var(--foreground); }
.gf-ripple { position: absolute; inset: 0; border-radius: 50%; box-shadow: 0 0 0 1px color-mix(in srgb, var(--foreground) 40%, transparent); opacity: 0; animation: gf-ripple 3.2s linear 1600ms infinite; }
.gf-ripple-2 { animation-delay: 3200ms; }

@keyframes gf-draw { to { stroke-dashoffset: 0; } }
@keyframes gf-rise { from { opacity: 0; transform: translateY(8px); } }
@keyframes gf-fade { to { opacity: 1; } }
@keyframes gf-drop { to { opacity: 1; transform: none; } }
@keyframes gf-ripple { 0% { opacity: 0.5; transform: scale(1); } 70% { opacity: 0; transform: scale(2.2); } 100% { opacity: 0; transform: scale(2.2); } }

.gf-still .gf-line { animation: none; stroke-dashoffset: 0; }
.gf-still .gf-ring, .gf-still .gf-drop { animation: none; opacity: 1; transform: none; }
.gf-still .gf-ripple { display: none; }
.gf-still .gf-layer { transform: none; transition: none; }
`;
