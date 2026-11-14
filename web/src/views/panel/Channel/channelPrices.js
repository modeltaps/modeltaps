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

// The short form the table shows: a token pair (bare numbers; the column header carries "$/M"),
// a per-image or per-request price, or free.
export function priceSummary(price) {
  if (!price) return null;
  const { input = 0, output = 0, image = 0, request = 0, video_second: videoSecond = 0 } = price;
  if (input || output) return { kind: 'tokens', text: `${amount(input)} / ${amount(output)}` };
  if (image) return { kind: 'image', value: usd(image) };
  if (videoSecond) return { kind: 'video', value: usd(videoSecond) };
  if (price.other) return { kind: 'other' };
  if (request) return { kind: 'request', value: usd(request) };
  return { kind: 'free' };
}

// Every price the upstream states for the model, for the cell's tooltip: [key, formatted value].
export function priceDetails(price) {
  if (!price) return [];
  return ['input', 'output', 'cache_read', 'cache_write', 'image', 'video_second', 'request']
    .filter((key) => Number(price[key]) > 0)
    .map((key) => [key, usd(price[key])]);
}
