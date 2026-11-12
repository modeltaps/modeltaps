import { API } from 'utils/api';

// Calls for model supplies and supply pauses. Each returns the response body
// ({ success, message, data }); a failed request throws (the API client already shows it).

const body = (res) => res.data || {};

export const listSupplies = (params = {}) => API.get('/api/model_supply/', { params }).then(body);
export const createSupply = (supply) => API.post('/api/model_supply/', supply).then(body);
export const updateSupply = (supply) => API.put('/api/model_supply/', supply).then(body);
export const deleteSupply = (id) => API.delete(`/api/model_supply/${id}`).then(body);

// How the channel names the public model upstream: { upstream_model, source }, where source is
// "mapping", "channel", "library" (the platform's spelling, e.g. Vertex's @default) or "".
export const suggestUpstream = ({ model, channel_id }) =>
  API.get('/api/model_supply/upstream_suggestion', { params: { model, channel_id } }).then(body);

// The order a request for the model would try its supplies in, with each supply's state and
// share; `group` limits it to supplies whose channel serves that user group.
export const previewSupplies = ({ model, group = '' }) =>
  API.get('/api/model_supply/preview', { params: group ? { model, group } : { model } }).then(body);

// User groups a request can come from, as [{ symbol, name }] sorted by symbol; channels list
// the symbols they serve.
export async function listUserGroups() {
  try {
    const { success, data } = body(await API.get('/api/group/'));
    if (!success || !Array.isArray(data)) return [];
    return data
      .map((g) => (typeof g === 'string' ? { symbol: g, name: '' } : { symbol: g.symbol, name: g.name || '' }))
      .filter((g) => g.symbol)
      .sort((a, b) => a.symbol.localeCompare(b.symbol));
  } catch {
    return [];
  }
}

// New upstream models waiting on channels for an admin:
// [{ upstream_model, suggestion: { model, source, exists, status }, channels: [{ channel_id, channel_name, ... }] }].
export const listPendingModels = () => API.get('/api/model_supply/pending').then(body);
// Supplies `model` with the upstream model on `channel_ids` (every channel it waits on when
// empty). With `create`, a model that is not public yet is created first, as a draft.
export const attachPendingModel = ({ upstream_model, model, channel_ids = [], create = false }) =>
  API.post('/api/model_supply/pending/attach', { upstream_model, model, channel_ids, create }).then(body);
export const ignorePendingModel = ({ upstream_model, channel_ids = [] }) =>
  API.post('/api/model_supply/pending/ignore', { upstream_model, channel_ids }).then(body);
// Every catalog row (public models, aliases, routing policies), for picking a public model.
export const listCatalogRows = () => API.get('/api/model_info/catalog').then(body);

export const listPauses = () => API.get('/api/supply_pause/').then(body);
export const pauseSupply = ({ channel_id, model, seconds, probe = false }) =>
  API.post('/api/supply_pause/', { channel_id, model, seconds, probe }).then(body);
export const resumePause = (id) => API.delete(`/api/supply_pause/${id}`).then(body);
// Pause rules in force: { enabled, custom, rules (site rules), defaults, channel_rules? }.
export const fetchPauseRules = (channelId) =>
  API.get('/api/supply_pause/rules', { params: channelId ? { channel_id: channelId } : {} }).then(body);

// The supply switches live in the root-only options. Returns { supplyEnabled, pauseRulesEnabled }
// as booleans, or null when they cannot be read (not root, or the request failed).
export async function fetchSupplySwitches() {
  try {
    const { success, data } = body(await API.get('/api/option/'));
    if (!success || !Array.isArray(data)) return null;
    const value = (key) => data.find((o) => o.key === key)?.value;
    const supply = value('ModelSupplyEnabled');
    if (supply === undefined) return null;
    return { supplyEnabled: supply === 'true', pauseRulesEnabled: value('PauseRulesEnabled') !== 'false' };
  } catch {
    return null;
  }
}
