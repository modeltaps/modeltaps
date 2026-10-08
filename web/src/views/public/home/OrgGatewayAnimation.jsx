import { useTranslation } from 'react-i18next';
import { LogoMark } from '@/components/chrome/Logo';

// ==============================|| PUBLIC HOME — ORG GATEWAY ANIMATION ||============================== //
// "One exit for the organization": members holding their own key (left) flow into the
// dashed organization boundary (Modeltaps hub), then fan out to two model
// sources (cloud providers / org-owned key). Monochrome product tokens only,
// light+dark safe, honors reduced-motion.

// viewBox 480x320. Members sit at x=46, hub at (240,150), lane marks at x=426.
const MEMBERS = [
  { n: 'm1', y: 40, d: 'M70 40 C135 40 151 150 203 150' },
  { n: 'm2', y: 110, d: 'M70 110 C135 110 160 150 203 150' },
  { n: 'm3', y: 190, d: 'M70 190 C135 190 160 150 203 150' },
  { n: 'm4', y: 260, d: 'M70 260 C135 260 151 150 203 150' }
];

// Right-hand lanes: cloud provider, org-owned key (key + cloud). Centred on y=150.
const LANES = [
  { n: 'l1', y: 113, d: 'M271 150 C332 150 350 113 404 113' },
  { n: 'l2', y: 187, d: 'M271 150 C332 150 350 187 404 187' }
];

const CLOUD = 'M17.5 19H9a7 7 0 1 1 6.71-9h1.79a4.5 4.5 0 1 1 0 9Z';
const KEY_ROUND =
  'M2.6 17.4A2 2 0 0 0 2 18.8V21a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h1a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h.2a2 2 0 0 0 1.4-.6l.8-.8a6.5 6.5 0 1 0-4-4z';

export default function OrgGatewayAnimation() {
  const { t } = useTranslation();

  return (
    <div className="capog" role="img" aria-label={t('home.hero.gatewayAria')}>
      <style>{CAPOG_CSS}</style>
      <svg className="capog-svg" viewBox="0 0 480 320" preserveAspectRatio="none" fill="none" aria-hidden="true">
        <rect className="capog-org" x="165" y="24" width="150" height="252" rx="20" />

        {MEMBERS.map((m) => (
          <path key={m.n} className={`capog-wire ${m.n}`} d={m.d} />
        ))}
        {LANES.map((l) => (
          <path key={l.n} className={`capog-wire ${l.n}`} d={l.d} />
        ))}
        {[...MEMBERS, ...LANES].map((p) => (
          <path key={`d-${p.n}`} className={`capog-dot d-${p.n}`} pathLength="100" d={p.d} />
        ))}

        {MEMBERS.map((m) => (
          <g key={`g-${m.n}`} className={`capog-mark ${m.n}`}>
            <circle className="capog-card" cx="46" cy={m.y} r="18" />
            <g className="capog-glyph" transform={`translate(${46 - 9} ${m.y - 9}) scale(0.75)`}>
              <circle cx="12" cy="9.3" r="3.2" />
              <path d="M5.8 18.6a6.2 6.2 0 0 1 12.4 0" />
            </g>
            <circle className="capog-card" cx="59" cy={m.y + 13} r="7" />
            <g className="capog-glyph" transform={`translate(59 ${m.y + 13})`}>
              <circle cx="-1.6" cy="0" r="1.7" />
              <path d="M0.1 0H3.6M2.4 0v1.6" />
            </g>
          </g>
        ))}

        <g className="capog-mark l1">
          <circle className="capog-card" cx="426" cy="113" r="22" />
          <g className="capog-glyph" transform="translate(415 102) scale(0.9)">
            <path d={CLOUD} />
          </g>
        </g>
        <g className="capog-mark l2">
          <circle className="capog-card" cx="426" cy="187" r="22" />
          <g className="capog-glyph capog-glyph-strong" transform="translate(415 176) scale(0.9)">
            <g transform="translate(-2.5 -2.5) scale(0.54)">
              <path d={CLOUD} />
            </g>
            <g transform="translate(3.6 4.6) scale(0.88)">
              <path d={KEY_ROUND} />
            </g>
          </g>
        </g>
      </svg>

      <span className="capog-hub">
        <LogoMark className="capog-hub-mark" />
      </span>
    </div>
  );
}

// Scoped styles (capog-* namespace). Every colour resolves from shadcn product tokens so
// the figure stays monochrome in both themes.
const CAPOG_CSS = `
.capog { position: relative; width: 100%; aspect-ratio: 480 / 320; color: var(--primary); }
.capog-svg { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; }
.capog-org { stroke: var(--border); stroke-width: 1.4px; stroke-dasharray: 6 6; vector-effect: non-scaling-stroke; fill: color-mix(in srgb, var(--foreground) 3%, transparent); }
.capog-wire { stroke: color-mix(in srgb, var(--foreground) 26%, transparent); stroke-width: 1.4px; vector-effect: non-scaling-stroke; }
.capog-dot { stroke: var(--primary); stroke-width: 5px; stroke-linecap: round; vector-effect: non-scaling-stroke; stroke-dasharray: .5 99.5; filter: drop-shadow(0 0 3px color-mix(in srgb, var(--primary) 70%, transparent)); animation: capog-flow 1.9s linear infinite; }
.capog-dot.d-m1 { animation-duration: 2.0s; animation-delay: -1.7s; }
.capog-dot.d-m2 { animation-duration: 1.7s; animation-delay: -1.1s; }
.capog-dot.d-m3 { animation-duration: 1.9s; animation-delay: -0.6s; }
.capog-dot.d-m4 { animation-duration: 2.2s; animation-delay: -1.9s; }
.capog-dot.d-l1 { animation-duration: 2.1s; animation-delay: -1.4s; }
.capog-dot.d-l2 { animation-duration: 1.8s; animation-delay: -0.9s; }
@keyframes capog-flow { from { stroke-dashoffset: 100; } to { stroke-dashoffset: 0; } }
.capog-card { fill: var(--card); stroke: var(--border); stroke-width: 1.2px; vector-effect: non-scaling-stroke; }
.capog-glyph { stroke: var(--muted-foreground); stroke-width: 1.5px; stroke-linecap: round; stroke-linejoin: round; fill: none; vector-effect: non-scaling-stroke; }
.capog-glyph-strong { stroke-width: 2.2px; }
.capog-hub { position: absolute; transform: translate(-50%, -50%); left: 50%; top: 46.9%; width: 13.5%; aspect-ratio: 1; border-radius: 50%; background: var(--card); box-shadow: 0 10px 26px color-mix(in srgb, var(--foreground) 12%, transparent), 0 0 0 1px var(--border); display: grid; place-items: center; }
.capog-hub::before { content: ""; position: absolute; inset: 0; border-radius: 50%; animation: capog-pulse 2.8s ease-out infinite; }
.capog-hub-mark { width: 35%; height: auto; color: var(--primary); position: relative; }
@keyframes capog-pulse { 0% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--primary) 26%, transparent); } 70%, 100% { box-shadow: 0 0 0 18px color-mix(in srgb, var(--primary) 0%, transparent); } }
@media (prefers-reduced-motion: reduce) {
  .capog-dot { display: none; }
  .capog-hub::before { animation: none; }
}
`;
