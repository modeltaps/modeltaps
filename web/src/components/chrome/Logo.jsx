// ==============================|| CHROME — LOGO MARK (Modeltaps mark, currentColor) ||============================== //
// The brand mark: a solid drop with an off-centre round hole.
//
// It is drawn in optical sizes on whole-pixel grids (12, 16 and 20 px tall): on each grid the body's
// left, right and bottom edges and the hole's four extremes sit on integer coordinates.
//
// UI sizes (12, 16, 20, 40) render as a pre-rendered bitmap used as a CSS mask over currentColor,
// with 1x / 2x / 3x files in public/brand/mark/. Chromium's GPU rasterizer draws vector paths with
// 4-sample MSAA (5 coverage levels), so an inline SVG this small looks visibly jagged on 1x screens next
// to smooth text; the PNGs are rendered with full analytic coverage. Pass `size` (rendered height in
// px) instead of h-* / w-* classes and keep the mark on a whole-pixel offset.
//
// Without a supported `size` it falls back to the 20 grid as inline SVG filling the className's box
// (large illustrations, where MSAA steps are not visible).

const GRIDS = {
  12: { w: 10, h: 12 },
  16: { w: 12, h: 16 },
  20: {
    w: 16,
    h: 20,
    d: 'M6.613 0.683A1.75 1.75 0 0 1 9.387 0.683L14.341 7.122A8 8 0 1 1 1.659 7.122ZM12 13a2 2 0 1 0 -4 0a2 2 0 1 0 4 0Z'
  }
};

// rendered height -> grid it is drawn on
const RASTER = { 12: 12, 16: 16, 20: 20, 40: 20 };

export function LogoMark({ size, className }) {
  const grid = RASTER[size];
  if (grid) {
    const g = GRIDS[grid];
    const base = `/brand/mark/${size}`;
    const mask = `image-set(url(${base}.png) 1x, url(${base}@2x.png) 2x, url(${base}@3x.png) 3x)`;
    return (
      <span
        aria-hidden="true"
        className={['inline-block shrink-0 bg-current', className].filter(Boolean).join(' ')}
        style={{
          width: (g.w * size) / g.h,
          height: size,
          maskImage: mask,
          WebkitMaskImage: mask,
          maskSize: '100% 100%',
          WebkitMaskSize: '100% 100%',
          maskRepeat: 'no-repeat',
          WebkitMaskRepeat: 'no-repeat'
        }}
      />
    );
  }
  const g = GRIDS[20];
  return (
    <svg viewBox={`0 0 ${g.w} ${g.h}`} fill="currentColor" className={className} role="img" aria-hidden="true">
      <path fillRule="evenodd" d={g.d} />
    </svg>
  );
}
