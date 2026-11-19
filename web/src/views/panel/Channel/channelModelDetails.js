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

// How a channel's price for a model compares with the model's base sell price, per million
// tokens: { delta, above }. `delta` is the dearer side's cost over the sell price minus one (0.135:
// 13.5% above it, -0.18: 18% below), from the channel cost when known (fee included), else the
// upstream price. `above` is the backend's verdict that the cost is above what the price brings in
// (default group ratio, less payment costs), the same one the supplies list warns with; without a
// known cost it falls back to delta > 0. null when either side is not priced per token.
export function sellDeltaOf({ price, cost, sellInput, sellOutput }) {
  const known = cost?.known && cost.input != null ? cost : null;
  const ratios = [];
  for (const [key, sell] of [
    ['input', sellInput],
    ['output', sellOutput]
  ]) {
    const value = known ? Number(known[key] ?? 0) : tokenPrice(price, key);
    if (value === null || sell === null || sell === undefined || !(Number(sell) > 0)) continue;
    ratios.push(value / Number(sell) - 1);
  }
  if (ratios.length === 0) return null;
  const delta = Math.max(...ratios);
  return { delta, above: known ? Boolean(known.above_quote) : delta > 1e-6 };
}

// Which side of the base sell price a comparison falls on, for the list filters: 'above' (dearer
// than the sell price), 'below' (cheaper) or 'equal' (within rounding); null without one.
export function sellSideOf(cmp) {
  if (!cmp) return null;
  if (cmp.delta > 0.005) return 'above';
  if (cmp.delta < -0.005) return 'below';
  return 'equal';
}

// The comparison a channel's model list shows for one listed model: against the public models it
// serves (the enabled ones; all of them when none is, so a supply waiting for review still
// compares), the dearest one. `price` is the model's upstream price entry, `publics` the details'.
export function rowSellCmp(price, publics = []) {
  const usable = (publics || []).filter((p) => !p.gone);
  const enabled = usable.filter((p) => p.enabled);
  let worst = null;
  for (const p of enabled.length > 0 ? enabled : usable) {
    const cmp = sellDeltaOf({ price, cost: p.cost, sellInput: p.input_price, sellOutput: p.output_price });
    if (cmp && (!worst || cmp.delta > worst.delta)) worst = { ...cmp, public: p };
  }
  return worst;
}

// `rows` ordered by their comparison with the sell price (`dir` 'asc': cheapest first); rows
// without one follow in their original order. `cmpOf(row)` gives the row's rowSellCmp.
export function sortBySellDelta(rows, dir, cmpOf) {
  const sign = dir === 'desc' ? -1 : 1;
  return rows
    .map((row, i) => ({ row, i, d: cmpOf(row)?.delta ?? null }))
    .sort((a, b) => {
      if (a.d === null || b.d === null) return (a.d === null) - (b.d === null) || a.i - b.i;
      return sign * (a.d - b.d) || a.i - b.i;
    })
    .map((x) => x.row);
}
