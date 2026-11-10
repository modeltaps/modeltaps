// ==============================|| CHROME — LOGO MARK (Modeltaps mark, currentColor) ||============================== //
// The brand mark: a solid drop with an off-centre round hole, one even-odd path.
//
// It ships in three optical sizes drawn on whole-pixel grids (12, 16 and 20 px tall). On each grid the
// body's left, right and bottom edges and the hole's four extremes sit on integer coordinates, so at 1x
// the outline has no half-covered pixel columns or rows and the hole stays a clean 3–4 px opening. A
// single path scaled down from a large master lands between pixels and renders blurry and jagged.
//
// Pass `size` (the rendered height in px). A size that is a whole multiple of a grid uses that grid
// (20, 40, 60 → 20; 16, 32, 48 → 16; 12, 24 → 12); any other size falls back to the 20 grid scaled. The element gets explicit width and height,
// so do not size it with h-* / w-* classes; keep it on a whole-pixel offset. Without `size` the mark
// fills whatever box the className gives it (illustrations).
// public/brand/favicon.* use the 16 grid; the larger PWA / og assets use the 20 grid.

const GRIDS = {
  12: {
    w: 10,
    h: 12,
    d: 'M4.175 0.373A1.1 1.1 0 0 1 5.825 0.373L8.752 3.695A5 5 0 1 1 1.248 3.695ZM8 7.5a1.5 1.5 0 1 0 -3 0a1.5 1.5 0 1 0 3 0Z'
  },
  16: {
    w: 12,
    h: 16,
    d: 'M4.906 0.598A1.3 1.3 0 0 1 7.094 0.598L11.049 6.759A6 6 0 1 1 0.951 6.759ZM9 11.5a1.5 1.5 0 1 0 -3 0a1.5 1.5 0 1 0 3 0Z'
  },
  20: {
    w: 16,
    h: 20,
    d: 'M6.613 0.683A1.75 1.75 0 0 1 9.387 0.683L14.341 7.122A8 8 0 1 1 1.659 7.122ZM12 13a2 2 0 1 0 -4 0a2 2 0 1 0 4 0Z'
  }
};

function gridFor(size) {
  const h = [20, 16, 12].find((n) => size && size % n === 0) || 20;
  return GRIDS[h];
}

export function LogoMark({ size, className }) {
  const g = gridFor(size);
  const dims = size ? { width: (g.w * size) / g.h, height: size } : {};
  return (
    <svg
      viewBox={`0 0 ${g.w} ${g.h}`}
      fill="currentColor"
      {...dims}
      className={['shrink-0', className].filter(Boolean).join(' ')}
      role="img"
      aria-hidden="true"
    >
      <path fillRule="evenodd" d={g.d} />
    </svg>
  );
}
