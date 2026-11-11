import { describe, expect, it } from 'vitest';

import { noticeSummary, resolveNotice } from './noticeState';

describe('resolveNotice', () => {
  it('a new announcement is unread', () => {
    expect(resolveNotice('Maintenance tonight', null)).toEqual({ notice: 'Maintenance tonight', unread: true, forgetRead: false });
  });

  it('an announcement the user already read stays available but is not unread', () => {
    expect(resolveNotice('Maintenance tonight', 'Maintenance tonight')).toEqual({
      notice: 'Maintenance tonight',
      unread: false,
      forgetRead: false
    });
  });

  it('a changed announcement is unread again', () => {
    expect(resolveNotice('Maintenance moved to Friday', 'Maintenance tonight').unread).toBe(true);
  });

  it('no announcement on the site: nothing to show, and the read text is dropped', () => {
    expect(resolveNotice('', 'Maintenance tonight')).toEqual({ notice: null, unread: false, forgetRead: true });
    expect(resolveNotice('  \n', 'Maintenance tonight')).toEqual({ notice: null, unread: false, forgetRead: true });
    expect(resolveNotice('', null)).toEqual({ notice: null, unread: false, forgetRead: false });
  });

  it('a failed request shows nothing and keeps the read text', () => {
    expect(resolveNotice(null, 'Maintenance tonight')).toEqual({ notice: null, unread: false, forgetRead: false });
    expect(resolveNotice(undefined, null)).toEqual({ notice: null, unread: false, forgetRead: false });
  });
});

describe('noticeSummary', () => {
  it('plain text: the first line that has text', () => {
    expect(noticeSummary('\n\n  Maintenance tonight 02:00-04:00  \nDetails below')).toBe('Maintenance tonight 02:00-04:00');
  });

  it('Markdown: drops heading, list, quote and emphasis markers and keeps link text', () => {
    expect(noticeSummary('## Price **update**\n\nSee the table.')).toBe('Price update');
    expect(noticeSummary('- New model: `claude-fable-5-2`')).toBe('New model: claude-fable-5-2');
    expect(noticeSummary('> Read the [changelog](https://example.com/changelog) first')).toBe('Read the changelog first');
    expect(noticeSummary('1. *Scheduled* maintenance')).toBe('Scheduled maintenance');
    expect(noticeSummary('![banner](https://example.com/a.png) Hello')).toBe('banner Hello');
  });

  it('keeps underscores and asterisks that are part of the text', () => {
    expect(noticeSummary('Set max_tokens to 4096 * 2')).toBe('Set max_tokens to 4096 * 2');
  });

  it('skips rules and table dividers', () => {
    expect(noticeSummary('---\n\nMaintenance tonight')).toBe('Maintenance tonight');
    expect(noticeSummary('|---|:--|\nRow')).toBe('Row');
  });

  it('HTML: strips tags and decodes common entities', () => {
    expect(noticeSummary('<div><h3>Rates &amp; limits</h3><p>From <b>Monday</b>.</p></div>')).toBe('Rates & limits');
    expect(noticeSummary('<style>p{color:red}</style><p>Maintenance&nbsp;tonight</p>')).toBe('Maintenance tonight');
    expect(noticeSummary('<p>Line one<br/>Line two</p>')).toBe('Line one');
  });

  it('has no summary for an empty announcement or one that is only a URL', () => {
    expect(noticeSummary('')).toBe('');
    expect(noticeSummary(null)).toBe('');
    expect(noticeSummary('https://example.com/notice')).toBe('');
    expect(noticeSummary('---')).toBe('');
  });
});
