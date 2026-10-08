import { useSyncExternalStore } from 'react';

// ==============================|| BRAND ICON — MANIFEST & RESOLVER ||============================== //
// The brand-icon manifest (icons + aliases/channelTypes/modelPrefixes/domains) is served by
// the backend at /api/brand-icon/manifest (see common/brandicon). It is fetched once per page
// load and shared by every BrandIcon; icon SVGs are always requested from the same origin so
// rendering never depends on a third-party address.

const API_BASE = (import.meta.env.VITE_APP_SERVER || '/').replace(/\/+$/, '');

export function brandIconUrl(key, variant) {
  const query = variant ? `?variant=${variant}` : '';
  return `${API_BASE}/api/brand-icon/${encodeURIComponent(key)}${query}`;
}

export function brandIconUploadUrl(assetId) {
  return `${API_BASE}/api/brand-icon/upload/${encodeURIComponent(assetId)}`;
}

export function brandIconDomainUrl(domain) {
  return `${API_BASE}/api/brand-icon/domain/${encodeURIComponent(domain)}`;
}

// Asset id of an admin-uploaded icon (`upload:{assetId}`); null for any other value.
export function uploadIconId(icon) {
  const value = String(icon ?? '').trim();
  if (!value.toLowerCase().startsWith('upload:')) return null;
  return value.slice('upload:'.length).trim() || null;
}

// status: idle | loading | ready | error
let state = { status: 'idle', manifest: null };
const listeners = new Set();

function setState(next) {
  state = next;
  listeners.forEach((listener) => listener());
}

export function setBrandIconManifest(manifest) {
  setState(manifest ? { status: 'ready', manifest } : { status: 'idle', manifest: null });
}

function loadManifest() {
  if (state.status !== 'idle' || typeof fetch !== 'function') return;
  setState({ status: 'loading', manifest: null });
  fetch(brandIconUrl('manifest'))
    .then((res) => (res.ok ? res.json() : Promise.reject(new Error(`HTTP ${res.status}`))))
    .then((manifest) => setState({ status: 'ready', manifest }))
    .catch(() => setState({ status: 'error', manifest: null }));
}

function subscribe(listener) {
  listeners.add(listener);
  loadManifest();
  return () => listeners.delete(listener);
}

const getSnapshot = () => state;

export function useBrandIconManifest() {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

const normalize = (value) =>
  String(value ?? '')
    .trim()
    .toLowerCase();

// Canonical icon key for a key or alias; null when unknown.
export function lookupBrandKey(manifest, value) {
  const key = normalize(value);
  if (!key || !manifest?.icons) return null;
  if (manifest.icons[key]) return key;
  const target = manifest.aliases?.[key];
  return target && manifest.icons[target] ? target : null;
}

// Admin-set icon: `brand:{key}` or a bare built-in key. Uploads are rendered by BrandIcon
// directly; external URLs are ignored (they would break the offline constraint).
function fromCustomIcon(manifest, icon) {
  const value = normalize(icon);
  if (!value) return null;
  if (value.startsWith('brand:')) return lookupBrandKey(manifest, value.slice('brand:'.length));
  if (value.includes(':') || value.includes('/')) return null;
  return lookupBrandKey(manifest, value);
}

function fromChannelType(manifest, channelType) {
  const num = Number(channelType);
  if (!num || Number.isNaN(num)) return null;
  return lookupBrandKey(manifest, manifest.channelTypes?.[String(num)]);
}

const prefixCache = new WeakMap();

function sortedPrefixes(manifest) {
  let list = prefixCache.get(manifest);
  if (!list) {
    list = Object.keys(manifest.modelPrefixes || {}).sort((a, b) => b.length - a.length);
    prefixCache.set(manifest, list);
  }
  return list;
}

// Longest-prefix match on the model name, then on its last path segment
// (e.g. `deepseek-ai/DeepSeek-V3` → `deepseek-v3`).
function fromModel(manifest, model) {
  const name = normalize(model);
  if (!name) return null;
  const candidates = [name];
  const segment = name.split('/').pop();
  if (segment && segment !== name) candidates.push(segment);
  const prefixes = sortedPrefixes(manifest);
  for (const candidate of candidates) {
    const prefix = prefixes.find((p) => candidate.startsWith(p));
    if (prefix) return lookupBrandKey(manifest, manifest.modelPrefixes[prefix]);
  }
  return null;
}

export function hostnameFromBaseUrl(baseUrl) {
  const raw = String(baseUrl ?? '').trim();
  if (!raw) return null;
  try {
    return new URL(raw).hostname.toLowerCase() || null;
  } catch {
    const host = raw
      .replace(/^\/+/, '')
      .split(/[/:?#]/)[0]
      .toLowerCase();
    return host && host.includes('.') ? host : null;
  }
}

// Host eligible for the favicon cache lookup (/api/brand-icon/domain/{host}, which normalizes
// it to the registrable domain); null for IP literals, single-label hosts or invalid input.
export function iconCacheHost(value) {
  const host = hostnameFromBaseUrl(value);
  if (!host || !/^[a-z0-9.-]+$/.test(host) || !host.includes('.')) return null;
  if (/^[0-9.]+$/.test(host)) return null;
  return host.replace(/\.+$/, '') || null;
}

// Most specific domain first: api.deepseek.com → deepseek.com.
function fromBaseUrl(manifest, baseUrl) {
  const host = hostnameFromBaseUrl(baseUrl);
  if (!host || !manifest.domains) return null;
  const labels = host.split('.');
  for (let i = 0; i < labels.length - 1; i += 1) {
    const target = manifest.domains[labels.slice(i).join('.')];
    if (target) return lookupBrandKey(manifest, target);
  }
  return null;
}

// Resolution order (first hit wins): admin custom icon > key/alias (brandKey, ownedBy) >
// channel type > model-name prefix > base_url domain. Returns null for a letter placeholder.
export function resolveBrandIconKey(manifest, { icon, brandKey, ownedBy, channelType, model, baseUrl } = {}) {
  if (!manifest?.icons) return null;
  return (
    fromCustomIcon(manifest, icon) ||
    lookupBrandKey(manifest, brandKey) ||
    lookupBrandKey(manifest, ownedBy) ||
    fromChannelType(manifest, channelType) ||
    fromModel(manifest, model) ||
    fromBaseUrl(manifest, baseUrl) ||
    null
  );
}
