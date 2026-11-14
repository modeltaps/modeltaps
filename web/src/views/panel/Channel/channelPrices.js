// Pure helpers for a channel's upstream price list: which price belongs to a row of the channel's
// model list, and how it reads in the table. Prices come from the backend in USD, token prices per
// million tokens.

// The name the channel sends upstream for a listed model: its mapping target (without the "+"
// that only changes billing), else the name itself. Upstream prices are keyed by that name.
export function upstreamNameOf(name, mapping = {}) {
  const mapped = typeof mapping[name] === 'string' ? mapping[name].trim().replace(/^\+/, '') : '';
  return mapped || name;
}

export function priceIndex(prices = []) {
  return new Map(prices.map((p) => [p.model, p]));
}

// "3", "0.075", "0.0001": up to four significant digits, no trailing zeros.
export function amount(value) {
  const n = Number(value) || 0;
  return n === 0 ? '0' : Number(n.toPrecision(4)).toString();
}

export const usd = (value) => `$${amount(value)}`;

// The short form the table shows: input and output per million tokens (bare numbers; the column
// headers carry "$/M"), or for a model not priced per token one text for both columns: a
// per-image, per-second or per-request price, free, or other.
export function priceSummary(price) {
  if (!price) return null;
  const { input = 0, output = 0, image = 0, request = 0, video_second: videoSecond = 0 } = price;
  if (input || output) return { kind: 'tokens', input: amount(input), output: amount(output) };
  if (image) return { kind: 'image', value: usd(image) };
  if (videoSecond) return { kind: 'video', value: usd(videoSecond) };
  if (price.other) return { kind: 'other' };
  if (request) return { kind: 'request', value: usd(request) };
  return { kind: 'free' };
}

// The input or output price per million tokens, for sorting; null when the model is not priced
// per token (or has no price).
export function tokenPrice(price, key) {
  if (!price || !(Number(price.input) || Number(price.output))) return null;
  return Number(price[key]) || 0;
}

// `rows` ordered by a token price (`key` 'input' | 'output', `dir` 'asc' | 'desc'); rows without
// one follow, in their original order. Ties keep the original order too. `priceOf(row)` gives the
// row's price entry.
export function sortByTokenPrice(rows, key, dir, priceOf) {
  const sign = dir === 'desc' ? -1 : 1;
  return rows
    .map((row, i) => ({ row, i, v: tokenPrice(priceOf(row), key) }))
    .sort((a, b) => {
      if (a.v === null || b.v === null) return (a.v === null) - (b.v === null) || a.i - b.i;
      return sign * (a.v - b.v) || a.i - b.i;
    })
    .map((x) => x.row);
}
