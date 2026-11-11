import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { NoticeBanner } from './NoticeBanner';
import { NoticeContext } from './NoticeContext';

// The banner is the only thing that announces a new site announcement: it shows while the
// announcement is unread and takes no space otherwise.

const render = (value) =>
  renderToStaticMarkup(
    createElement(
      NoticeContext.Provider,
      { value: { isOpen: false, openNotice: () => {}, closeNotice: () => {}, dismissNotice: () => {}, ...value } },
      createElement(NoticeBanner)
    )
  );

describe('NoticeBanner', () => {
  it('renders nothing without an announcement', () => {
    expect(render({ notice: null, unread: false })).toBe('');
  });

  it('renders nothing once the announcement has been read or closed', () => {
    expect(render({ notice: 'Maintenance tonight', unread: false })).toBe('');
  });

  it('shows the label, the first line, a button to read it all and a close button', () => {
    const html = render({ notice: '## Maintenance **tonight**\n\nFrom 02:00 to 04:00.', unread: true });
    expect(html).toContain('aria-label="公告"');
    expect(html).toContain('Maintenance tonight');
    expect(html).not.toContain('From 02:00');
    expect(html).toContain('>查看</button>');
    expect(html).toContain('aria-label="关闭"');
  });

  it('shows the label alone when the announcement has no text line (a URL)', () => {
    const html = render({ notice: 'https://example.com/notice', unread: true });
    expect(html).toContain('公告');
    expect(html).not.toContain('example.com');
  });
});
