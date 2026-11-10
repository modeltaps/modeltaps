import { API } from 'utils/api';

// Calls for model supplies and supply pauses. Each returns the response body
// ({ success, message, data }); a failed request throws (the API client already shows it).

const body = (res) => res.data || {};

export const listSupplies = (params = {}) => API.get('/api/model_supply/', { params }).then(body);
export const createSupply = (supply) => API.post('/api/model_supply/', supply).then(body);
export const updateSupply = (supply) => API.put('/api/model_supply/', supply).then(body);
export const deleteSupply = (id) => API.delete(`/api/model_supply/${id}`).then(body);

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
