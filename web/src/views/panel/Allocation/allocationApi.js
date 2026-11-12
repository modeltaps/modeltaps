import { API } from 'utils/api';

// Calls for the allocation policy. Each returns the response body ({ success, message, data });
// a failed request throws (the API client already shows it).

const body = (res) => res.data || {};

// { supply_routing, tiers, providers, policy_models, manual_models, updated_at }
export const fetchAllocation = () => API.get('/api/allocation_policy/').then(body);
export const saveAllocation = (tiers) => API.put('/api/allocation_policy/', { tiers }).then(body);

// Switches public models (catalog ids) between the policy ('policy') and their own supply
// numbers ('manual').
export const setModelAllocation = ({ ids, allocation }) => API.post('/api/model_supply/allocation', { ids, allocation }).then(body);
