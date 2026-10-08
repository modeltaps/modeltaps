import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import MobilePushFrame from './MobilePushFrame';

// Narrow-screen sidebar is a push drawer: the panel slides in from the left and
// the whole content column (header + page) shifts right by the drawer width.
// No dimming / blur overlay. While open, a transparent close area covers the
// pushed content so a tap anywhere on it closes the drawer.

const render = (open) =>
  renderToStaticMarkup(
    createElement(
      MobilePushFrame,
      {
        open,
        onClose: () => {},
        sidebar: createElement('aside', { 'data-testid': 'desktop-sidebar' }),
        drawer: createElement('nav', { 'data-testid': 'drawer-nav' })
      },
      createElement('main', { 'data-testid': 'content' })
    )
  );

const panelTag = (html) => html.match(/<div[^>]*data-testid="push-drawer-panel"[^>]*>/)?.[0] ?? '';
const columnTag = (html) => html.match(/<div[^>]*data-testid="push-content-column"[^>]*>/)?.[0] ?? '';

describe('MobilePushFrame', () => {
  it('renders the desktop sidebar, the drawer and the content column', () => {
    const html = render(false);
    expect(html).toContain('data-testid="desktop-sidebar"');
    expect(html).toContain('data-testid="drawer-nav"');
    expect(html).toContain('data-testid="content"');
  });

  it('closed: panel is parked off-screen, inert and hidden from assistive tech; content is not shifted', () => {
    const html = render(false);
    const panel = panelTag(html);
    expect(panel).toContain('-left-[var(--drawer-w)]');
    expect(panel).toContain('aria-hidden="true"');
    expect(panel).toContain('inert=""');
    expect(columnTag(html)).not.toContain('left-[var(--drawer-w)]');
    expect(html).not.toContain('data-testid="push-close-area"');
  });

  it('open: panel slides to the left edge and the content column is pushed right by the drawer width', () => {
    const html = render(true);
    const panel = panelTag(html);
    expect(panel).toContain('left-0');
    expect(panel).not.toContain('-left-[var(--drawer-w)]');
    expect(panel).not.toContain('inert=""');
    expect(columnTag(html)).toContain('left-[var(--drawer-w)]');
  });

  it('open: a transparent close area covers the pushed content instead of a dimmed / blurred backdrop', () => {
    const html = render(true);
    expect(html).toContain('data-testid="push-close-area"');
    expect(html).not.toContain('backdrop-blur');
    expect(html).not.toContain('bg-black/40');
  });

  it('push drawer only exists below the md breakpoint', () => {
    const html = render(true);
    expect(panelTag(html)).toContain('md:hidden');
    expect(columnTag(html)).toContain('md:left-0');
  });
});
