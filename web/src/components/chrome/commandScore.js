import { normalizeText } from '@/components/settings/settingsSearch';

// ==============================|| CHROME — COMMAND PALETTE SCORING ||============================== //
// How well an entry of the command palette matches a query, from 0 (no match) to 1, so the palette
// can sort what it finds (cmdk sorts items and groups by this score). An entry's text comes in
// three tiers: its name (primary), the words it is also known by and its key (secondary), and the
// rest of what it says (tertiary: a setting's description, a card's text). Every word of the query
// must appear somewhere; the higher the tier it appears in, the higher the entry ranks:
//
//   1     the name is the query              0.8  every word is in the name
//   0.95  the name starts with the query     0.7  every word is in the name, some by a synonym
//   0.9   the name contains the query        0.5  every word is in the name or the secondary words
//                                            0.2  some word is only in the tertiary text
//
// Synonyms map a word people type to words the console uses (`command.synonyms` in the locale
// files: 故障转移 → 重试, 2fa → 两步验证…), so a term from another gateway still finds the setting.

const TIER_BREAK = '\u0000';

// Packs the tiers into cmdk's keyword list (an array of strings), which the filter receives.
export const packTiers = ({ primary = [], secondary = [], tertiary = [] }) => [
  ...primary.filter(Boolean),
  TIER_BREAK,
  ...secondary.filter(Boolean),
  TIER_BREAK,
  ...tertiary.filter(Boolean)
];

export function unpackTiers(keywords = []) {
  const tiers = [[], [], []];
  let i = 0;
  for (const word of keywords) {
    if (word === TIER_BREAK) i = Math.min(i + 1, 2);
    else tiers[i].push(word);
  }
  return { primary: tiers[0], secondary: tiers[1], tertiary: tiers[2] };
}

// Normalizes a synonym table ({ word: "other words" | [..] }) to { word: [other words] }.
export function normalizeSynonyms(...tables) {
  const out = {};
  for (const table of tables) {
    if (!table || typeof table !== 'object') continue;
    for (const [word, value] of Object.entries(table)) {
      const key = normalizeText(word);
      const list = (Array.isArray(value) ? value : String(value).split('|')).map(normalizeText).filter(Boolean);
      if (key && list.length) out[key] = [...(out[key] || []), ...list];
    }
  }
  return out;
}

const has = (hay, word, synonyms, viaSynonym = true) =>
  hay.includes(word) || (viaSynonym && (synonyms[word] || []).some((s) => hay.includes(s)));

// The words of a query; a synonym of several words ("load balancing") counts as one.
function splitWords(q, synonyms) {
  const words = [];
  let rest = ` ${q} `;
  for (const phrase of Object.keys(synonyms)) {
    if (phrase.includes(' ') && rest.includes(` ${phrase} `)) {
      words.push(phrase);
      rest = rest.replace(` ${phrase} `, ' ');
    }
  }
  return [...words, ...rest.trim().split(' ').filter(Boolean)];
}

export function scoreTiers(query, { primary = [], secondary = [], tertiary = [] }, synonyms = {}) {
  const q = normalizeText(query);
  if (!q) return 1;
  const words = splitWords(q, synonyms);
  const names = primary.map(normalizeText).filter(Boolean);
  const name = names.join(' ');
  const second = `${name} ${secondary.map(normalizeText).join(' ')}`;
  const all = `${second} ${tertiary.map(normalizeText).join(' ')}`;
  if (!words.every((w) => has(all, w, synonyms))) return 0;
  if (names.some((n) => n === q)) return 1;
  if (names.some((n) => n.startsWith(q))) return 0.95;
  if (names.some((n) => n.includes(q))) return 0.9;
  if (words.every((w) => has(name, w, synonyms, false))) return 0.8;
  if (words.every((w) => has(name, w, synonyms))) return 0.7;
  if (words.every((w) => has(second, w, synonyms))) return 0.5;
  return 0.2;
}

// For a query written as a sentence ("上游老是 429，怎么让它少重试一点"), which no entry has every
// word of: how much of it an entry covers. The query becomes terms (Latin words and numbers of two
// characters or more, pairs of adjacent CJK characters, and the console words of any synonym it
// contains); each term counts 3 in the name, 2 in the secondary words and 1 in the rest. 0 when
// nothing matches.
const RUNS = /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}]+|[\p{L}\p{N}]+/gu;
const CJK = /^[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}]/u;

export function sentenceTerms(query, synonyms = {}) {
  const q = normalizeText(query);
  const terms = new Set();
  for (const run of q.match(RUNS) || []) {
    if (CJK.test(run)) {
      for (let i = 0; i + 1 < run.length; i += 1) terms.add(run.slice(i, i + 2));
    } else if (run.length >= 2) terms.add(run);
  }
  for (const [word, others] of Object.entries(synonyms)) {
    if (q.includes(word)) others.forEach((other) => terms.add(normalizeText(other)));
  }
  return [...terms].filter(Boolean);
}

// `weights` (term → weight, default 1) lets rare terms count more than common ones.
export function sentenceScore(terms, { primary = [], secondary = [], tertiary = [] }, weights = {}) {
  const name = primary.map(normalizeText).join(' ');
  const second = secondary.map(normalizeText).join(' ');
  const rest = tertiary.map(normalizeText).join(' ');
  let score = 0;
  for (const term of terms) {
    const weight = weights[term] ?? 1;
    if (name.includes(term)) score += 3 * weight;
    else if (second.includes(term)) score += 2 * weight;
    else if (rest.includes(term)) score += weight;
  }
  return score;
}

// Scores of many entries for one sentence. A term found in many of them says little ("上游" is in
// half the gateway settings), so each term is weighted by how rare it is among them; terms in
// none are dropped. Entries scoring under `share` of the best one get 0.
export function rankSentence(query, tiersList, synonyms = {}, share = 0.5) {
  const texts = tiersList.map(({ primary = [], secondary = [], tertiary = [] }) =>
    [...primary, ...secondary, ...tertiary].map(normalizeText).join(' ')
  );
  const weights = {};
  const terms = sentenceTerms(query, synonyms).filter((term) => {
    const count = texts.filter((text) => text.includes(term)).length;
    if (count === 0) return false;
    weights[term] = Math.log(1 + texts.length / count);
    return true;
  });
  const scores = tiersList.map((tiers) => sentenceScore(terms, tiers, weights));
  const best = Math.max(0, ...scores);
  return scores.map((score) => (best > 0 && score >= best * share ? score : 0));
}
