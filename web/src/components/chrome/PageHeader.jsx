import PropTypes from 'prop-types';

// ==============================|| CHROME — PAGE HEADER (in-body title) ||============================== //
// Centralized page title + optional subtitle rendered at the top of <main>,
// above the routed <Outlet/>. Replaces the per-page title blocks so every
// panel page shares the same title position, spacing and type scale. The
// `actionsRef` slot lets a page place its primary actions on the title row
// (title left, actions right) via the PageActions portal. The `titleExtraRef`
// slot lets a page place controls inline to the RIGHT of the title, left-aligned
// on the same row (via PageTitleExtra); it uses `display:contents` so an empty
// slot adds no DOM/spacing on pages that don't use it. Relies on the <main>
// padding for left/top spacing; only adds the bottom gap to content.
// `leading` sits left of the title, centered on the title line (sidebar
// expand / mobile menu buttons).

export default function PageHeader({ title, subtitle, leading, actionsRef, titleExtraRef }) {
  return (
    <div className="mb-6 flex flex-wrap items-start gap-x-4 gap-y-3">
      <div className="flex min-w-0 grow basis-48 flex-wrap items-center gap-x-3 gap-y-2">
        <div className="flex min-w-0 items-start gap-2">
          {leading}
          <div className="min-w-0">
            <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
            {subtitle && <p className="mt-1 text-sm text-muted-foreground">{subtitle}</p>}
          </div>
        </div>
        <div ref={titleExtraRef} className="contents" />
      </div>
      <div ref={actionsRef} className="flex min-w-0 flex-wrap items-center justify-end gap-2" />
    </div>
  );
}

PageHeader.propTypes = {
  title: PropTypes.node,
  subtitle: PropTypes.node,
  leading: PropTypes.node,
  actionsRef: PropTypes.oneOfType([PropTypes.func, PropTypes.object]),
  titleExtraRef: PropTypes.oneOfType([PropTypes.func, PropTypes.object])
};
