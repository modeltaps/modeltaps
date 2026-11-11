// Pure helpers for the site announcement, kept apart from React so they are easy to test.

// localStorage key holding the announcement text the user last read (opened or dismissed).
export const NOTICE_STORAGE_KEY = 'notice';

// What to show for the announcement the server returned, given the text the user last read.
//   fetched: the announcement text; '' when the site has none; null when the request failed.
//   lastRead: the stored text, or null.
// Returns { notice, unread, forgetRead }: `notice` is null when there is nothing to show, and
// `forgetRead` asks the caller to drop the stored text (the site has no announcement any more,
// so the same text counts as new if it is published again). A failed request shows nothing and
// keeps the stored text.
export function resolveNotice(fetched, lastRead) {
  if (typeof fetched !== 'string') return { notice: null, unread: false, forgetRead: false };
  if (!fetched.trim()) return { notice: null, unread: false, forgetRead: lastRead != null };
  return { notice: fetched, unread: fetched !== lastRead, forgetRead: false };
}

const ENTITIES = { '&nbsp;': ' ', '&amp;': '&', '&lt;': '<', '&gt;': '>', '&quot;': '"', '&#39;': "'" };

// One line of plain text for the banner: the first line of the announcement that has text,
// without HTML tags or Markdown markers. The full, formatted announcement opens in the dialog.
// An announcement that is only a URL (shown in a frame there) has no summary.
export function noticeSummary(content) {
  if (typeof content !== 'string') return '';
  const trimmed = content.trim();
  if (/^https?:\/\/\S+$/.test(trimmed)) return '';

  const text = trimmed
    .replace(/<(style|script)\b[\s\S]*?<\/\1>/gi, '\n')
    .replace(/<br\s*\/?>|<\/(p|div|h[1-6]|li|tr|blockquote|pre)>/gi, '\n')
    .replace(/<[^>]+>/g, '')
    .replace(/&(nbsp|amp|lt|gt|quot|#39);/g, (entity) => ENTITIES[entity]);

  for (const raw of text.split('\n')) {
    const line = raw
      .replace(/^\s*(#{1,6}\s+|>\s*|[-*+]\s+|\d+[.)]\s+)*/, '')
      .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
      .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
      .replace(/(\*\*|__|~~|`)/g, '')
      .replace(/(^|\s)[*_]([^*_\s][^*_]*)[*_](?=\s|$|[.,;:!?，。；：！？])/g, '$1$2')
      .replace(/\s+/g, ' ')
      .trim();
    // Skip lines with no text of their own: blank lines, rules (---, ***) and table dividers.
    if (line && !/^[-*_=|:\s]+$/.test(line)) return line;
  }
  return '';
}
