// ==============================|| CHROME — LOGO MARK (Modeltaps mark, currentColor) ||============================== //
// Single source of truth for the in-app brand mark: a solid drop with an off-centre round hole
// (the "tap" drop and its highlight). One closed path on the 256 grid shared with the favicon: the
// body circle (r 82 at 128,150), two straight flanks to a tip rounded at r 18, and the hole (r 22 at
// 151,169) cut with even-odd fill. The viewBox is cropped to the ink (x 46–210, y 24–232) so the mark
// sits flush with the page gutter and its gap to the wordmark is the real gap. Size it by height with
// w-auto (Footer / Hero h-3, Header / Sidebar h-5, Auth h-9).
// The favicon / PWA / og assets in public/brand/ are generated from the same path.

const LOGO_PATH =
  'M113.5 31.33A18 18 0 0 1 142.5 31.33L194.05 101.41A82 82 0 1 1 61.95 101.41ZM173 169a22 22 0 1 0 -44 0a22 22 0 1 0 44 0Z';

export function LogoMark({ className }) {
  return (
    <svg viewBox="46 24 164 208" fill="currentColor" className={className} role="img" aria-hidden="true">
      <path fillRule="evenodd" d={LOGO_PATH} />
    </svg>
  );
}
