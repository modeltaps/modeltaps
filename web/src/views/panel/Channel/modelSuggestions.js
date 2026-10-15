import { API } from 'utils/api';
import { typeConfig } from './typeConfig';

// Candidate models offered by the channel form's model picker: the type's default models,
// then the price-table models filed under that channel type.
//
// Channel types that forward the same catalog as another type read that type's price
// entries: Azure OpenAI (3, 55) serves OpenAI models (1) and Vertex AI Express (61) serves
// Gemini models (25). Types with a fixed model set of their own (Bedrock, Vertex AI, the
// OAuth channels) only use their defaults plus entries filed under their own type.
export const PRICE_TYPE_ALIASES = { 3: 1, 55: 1, 61: 25 };

export function buildModelSuggestions(type, prices = []) {
  const priceType = PRICE_TYPE_ALIASES[type] ?? type;
  const defaults = typeConfig[type]?.input?.models || [];
  const priced = prices.filter((p) => p && p.channel_type === priceType && p.model).map((p) => p.model);
  return Array.from(new Set([...defaults, ...priced]));
}

// Price list from GET /api/prices ([] on any failure: suggestions are a convenience and the
// form must keep working without them).
export async function fetchPriceList() {
  try {
    const res = await API.get('/api/prices');
    const { success, data } = res.data || {};
    return success && Array.isArray(data) ? data : [];
  } catch (e) {
    return [];
  }
}
