// ==============================|| CHROME — LOGO MARK (Modeltaps mark, currentColor) ||============================== //
// Single source of truth for the in-app brand mark: the "taproot m", a lowercase m whose right leg
// drops past the others and curls left like a root tip. One round-capped stroke (width 32) drawn on
// the same 256 grid as the favicon, but the viewBox is cropped to the ink (x 48–208, y 32–224) so the
// mark sits flush with the page gutter and its gap to the wordmark is the real gap. Size it by height
// with w-auto (Footer / Hero h-3, Header / Sidebar h-5, Auth h-9); at h-3 every edge lands on the pixel grid.
// The favicon / PWA / og assets in public/brand/ are generated from the same path.

export function LogoMark({ className }) {
  return (
    <svg
      viewBox="48 32 160 192"
      fill="none"
      stroke="currentColor"
      strokeWidth="32"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      role="img"
      aria-hidden="true"
    >
      <path d="M64 144V80a32 32 0 0 1 64 0v64M128 80a32 32 0 0 1 64 0v96a32 32 0 0 1-32 32" />
    </svg>
  );
}
