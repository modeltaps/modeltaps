// Forgiving text search shared by the console's search boxes.
//
// The query is split on whitespace and every word must appear somewhere in the fields. Case,
// full-width forms and separators (spaces, - _ . / : and the like) are ignored on both sides, so
// "gpt mini", "GPT-mini" and "gptmini" all find "gpt-5-mini" and "gpt-4o-mini".

const SEPARATORS = /[\s\-_./:,;|+()[\]{}'"`~·]+/g;

// Lowercased, NFKC-folded text without separators.
export function normalizeSearchText(value) {
  if (value === null || value === undefined) return '';
  return String(value).normalize('NFKC').toLowerCase().replace(SEPARATORS, '');
}

// The query's words, normalized; empty words are dropped.
export function searchTerms(query) {
  return String(query || '')
    .normalize('NFKC')
    .trim()
    .split(/\s+/)
    .map(normalizeSearchText)
    .filter(Boolean);
}

// Whether every word of `query` appears in one of `fields` (a value or an array of values; nested
// arrays are flattened). A blank query matches everything. Pass pre-split `terms` from
// searchTerms() as the query when filtering many rows.
export function matchesSearch(query, fields) {
  const terms = Array.isArray(query) ? query : searchTerms(query);
  if (!terms.length) return true;
  const list = (Array.isArray(fields) ? fields : [fields]).flat(Infinity);
  const haystacks = list.map(normalizeSearchText).filter(Boolean);
  if (!haystacks.length) return false;
  return terms.every((term) => haystacks.some((h) => h.includes(term)));
}
