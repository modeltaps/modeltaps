import { useTranslation } from 'react-i18next';
import { LogoMark } from '@/components/chrome/Logo';

// ==============================|| PUBLIC HOME — ORG GATEWAY ANIMATION ||============================== //
// Symmetric line-art "bowtie": six arcs gather from the left edge (members / apps / API keys) into the
// Modeltaps mark at the centre, then fan back out to the right edge (model providers, neutral dots).
// Two dashed rings per side: inner left = org budget / allow-list check, inner right = channel routing
// and load balancing. Dots on the rings: primary = passed, warning = rate-limited / channel switched.
// Shadcn product tokens only, light+dark safe, fully static under reduced-motion.

// viewBox 480x320, centre (240,160). Left geometry is authored once and mirrored (x -> 480 - x).
const W = 480;
const ARC_YS = [30, 82, 134, 186, 238, 290];
const leftArc = (y) => `M0 ${y} C112 ${y} 150 160 240 160`;
const rightArc = (y) => `M240 160 C330 160 368 ${y} 480 ${y}`;

// Each ring spans every arc, its top / bottom ~8 units past the outermost arc at the ring's x.
const RINGS = [
  { x: 76, rx: 16, ry: 115.3 },
  { x: 158, rx: 9, ry: 45.7 }
];

// Arc / ring crossings on each ring's outer half (solved numerically from the curves above), tone per point.
const RING_DOTS = [
  { x: 71.3, y: 49.8, tone: 'primary' },
  { x: 63.2, y: 91.2, tone: 'muted' },
  { x: 60.3, y: 136.8, tone: 'primary' },
  { x: 60.3, y: 183.2, tone: 'primary' },
  { x: 63.2, y: 228.8, tone: 'primary' },
  { x: 71.3, y: 270.2, tone: 'muted' },
  { x: 154.1, y: 118.9, tone: 'primary' },
  { x: 150.7, y: 133.5, tone: 'primary' },
  { x: 149.2, y: 150.9, tone: 'primary' },
  { x: 149.2, y: 169.1, tone: 'warn' },
  { x: 150.7, y: 186.5, tone: 'muted' },
  { x: 154.1, y: 201.1, tone: 'primary' }
];

const SCATTER = [
  { x: 30, y: 56 },
  { x: 104, y: 40 },
  { x: 40, y: 108 },
  { x: 24, y: 212 },
  { x: 116, y: 68 },
  { x: 196, y: 128 },
  { x: 36, y: 268 },
  { x: 188, y: 96, tone: 'primary' },
  { x: 120, y: 248 }
];

// Slow primary pulses travelling left -> centre -> right.
const FLOWS = [
  { n: 'f1', from: 82, to: 238 },
  { n: 'f2', from: 186, to: 134 },
  { n: 'f3', from: 290, to: 30 }
];
const flowPath = (f) => `${leftArc(f.from)} C330 160 368 ${f.to} 480 ${f.to}`;

const mirror = (list) => [...list, ...list.map((p) => ({ ...p, x: W - p.x }))];

export default function OrgGatewayAnimation() {
  const { t } = useTranslation();

  return (
    <div className="capog" role="img" aria-label={t('home.hero.gatewayAria')}>
      <style>{CAPOG_CSS}</style>
      <svg className="capog-svg" viewBox="0 0 480 320" fill="none" aria-hidden="true">
        <path className="capog-axis" d="M0 160H480" />

        {ARC_YS.map((y) => (
          <path key={`l-${y}`} className="capog-line" d={leftArc(y)} />
        ))}
        {ARC_YS.map((y) => (
          <path key={`r-${y}`} className="capog-line" d={rightArc(y)} />
        ))}

        {mirror(RINGS).map((r) => (
          <ellipse key={`e-${r.x}`} className="capog-ring" cx={r.x} cy="160" rx={r.rx} ry={r.ry} />
        ))}

        {mirror(SCATTER).map((p) => (
          <circle key={`s-${p.x}-${p.y}`} className={`capog-pt capog-pt-${p.tone || 'soft'}`} cx={p.x} cy={p.y} r="1.6" />
        ))}

        {FLOWS.map((f) => (
          <path key={f.n} className={`capog-flow ${f.n}`} pathLength="100" d={flowPath(f)} />
        ))}

        {mirror(RING_DOTS).map((p) => (
          <circle key={`d-${p.x}-${p.y}`} className={`capog-pt capog-pt-${p.tone}`} cx={p.x} cy={p.y} r="2.6" />
        ))}
      </svg>

      <span className="capog-hub">
        <LogoMark className="capog-hub-mark" />
      </span>
    </div>
  );
}

// Scoped styles (capog-* namespace). Every colour resolves from shadcn product tokens
// (--warning is the existing product amber, defined for both themes).
const CAPOG_CSS = `
.capog { position: relative; width: 100%; aspect-ratio: 480 / 320; }
.capog-svg { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; }
.capog-line, .capog-axis, .capog-ring { stroke-width: 1px; vector-effect: non-scaling-stroke; }
.capog-line { stroke: color-mix(in srgb, var(--foreground) 22%, transparent); }
.capog-axis { stroke: color-mix(in srgb, var(--foreground) 16%, transparent); }
.capog-ring { stroke: color-mix(in srgb, var(--foreground) 34%, transparent); stroke-dasharray: 3 4; }
.capog-pt-primary { fill: var(--primary); }
.capog-pt-warn { fill: var(--warning); }
.capog-pt-muted { fill: var(--muted-foreground); }
.capog-pt-soft { fill: color-mix(in srgb, var(--foreground) 28%, transparent); }
.capog-flow { stroke: var(--primary); stroke-width: 3.5px; stroke-linecap: round; vector-effect: non-scaling-stroke; stroke-dasharray: .5 99.5; opacity: .75; animation: capog-flow 9s linear infinite; }
.capog-flow.f2 { animation-duration: 11s; animation-delay: -4s; }
.capog-flow.f3 { animation-duration: 13s; animation-delay: -9s; }
@keyframes capog-flow { from { stroke-dashoffset: 100; } to { stroke-dashoffset: 0; } }
.capog-hub { position: absolute; left: 50%; top: 50%; width: 12.5%; aspect-ratio: 1; transform: translate(-50%, -50%); border-radius: 50%; background: var(--card); box-shadow: 0 0 0 1px var(--border); display: grid; place-items: center; }
.capog-hub-mark { width: 34%; height: auto; color: var(--primary); }
@media (prefers-reduced-motion: reduce) {
  .capog-flow { animation: none; display: none; }
}
`;
