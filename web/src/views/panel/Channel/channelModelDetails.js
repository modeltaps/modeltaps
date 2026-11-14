import { tokenPrice } from './channelPrices';

// Pure helpers for the details of a channel's model list (POST /api/channel/:id/model_details):
// the usual order, sorting by name, and whether the upstream charges more than the model sells for.

// Rows in the usual order: by the details' rank (families this site uses most first, the latest
// version first inside a family); rows without details yet keep their order after the rest.
// `detailOf(row)` gives the row's details or undefined.
export function orderByRank(rows, detailOf) {
  return rows
    .map((row, i) => ({ row, i, rank: detailOf(row)?.rank ?? null }))
    .sort((a, b) => {
      if (a.rank === null || b.rank === null) return (a.rank === null) - (b.rank === null) || a.i - b.i;
      return a.rank - b.rank || a.i - b.i;
    })
    .map((x) => x.row);
}

// Rows by name, A → Z for 'asc', Z → A for 'desc'.
export function sortByName(rows, dir, nameOf = (r) => r.name) {
  const sign = dir === 'desc' ? -1 : 1;
  return [...rows].sort((a, b) => sign * nameOf(a).localeCompare(nameOf(b)));
}

// The public model (of `publics`, from the details) whose sell price for `key` ('input' |
// 'output', per million tokens) is below what the upstream charges (`price`, the channel's
// upstream price entry): { model, sell, cost }, the cheapest one when several are; null when none
// is sold at a loss. Disabled supplies and public models not sold per token are not compared.
export function lossOf(price, publics = [], key) {
  const cost = tokenPrice(price, key);
  if (cost === null || cost === 0) return null;
  let worst = null;
  for (const p of publics) {
    const sell = p[`${key}_price`];
    if (!p.enabled || sell === null || sell === undefined) continue;
    // Prices converted from ratios differ in the last float digits; equal prices are no loss.
    if (cost - sell > cost * 1e-6 && (!worst || sell < worst.sell)) worst = { model: p.model, sell, cost };
  }
  return worst;
}
