// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';

import { sanitizeHtml } from './sanitize';

describe('sanitizeHtml', () => {
  it('returns an empty string for empty or non-string input', () => {
    expect(sanitizeHtml('')).toBe('');
    expect(sanitizeHtml(null)).toBe('');
    expect(sanitizeHtml(42)).toBe('');
  });

  it('keeps formatting markup', () => {
    const html = '<h2>Notice</h2><p><strong>Bold</strong> and <a href="https://example.com/docs">a link</a></p>';
    expect(sanitizeHtml(html)).toBe(html);
  });

  it('strips scripts, event handlers and javascript: URLs', () => {
    const out = sanitizeHtml(
      '<p>hi</p><script>alert(1)</script><img src="x" onerror="alert(2)"><a href="javascript:alert(3)">x</a><iframe src="https://evil.test"></iframe>'
    );
    expect(out).not.toMatch(/<script|onerror|javascript:|<iframe/i);
    expect(out).toContain('<p>hi</p>');
  });

  it('neutralises mutation XSS through namespace confusion', () => {
    const out = sanitizeHtml('<svg><style><img src=x onerror=alert(1)></style></svg>');
    expect(out).not.toMatch(/onerror/i);
  });
});
