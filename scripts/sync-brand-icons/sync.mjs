#!/usr/bin/env node
// Regenerates common/brandicon/assets/*.svg and common/brandicon/manifest.json from a pinned
// @lobehub/icons-static-svg tarball. Uses only Node built-ins.
//
// Usage:
//   node scripts/sync-brand-icons/sync.mjs            # download from registry
//   node scripts/sync-brand-icons/sync.mjs --from x.tgz
// Env: NPM_REGISTRY (default https://registry.npmjs.org)
import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gunzipSync } from 'node:zlib';

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, '..', '..');
const outDir = join(root, 'common', 'brandicon');
const assetsDir = join(outDir, 'assets');
const MAX_TOTAL_BYTES = 1024 * 1024;

const cfg = JSON.parse(readFileSync(join(here, 'config.json'), 'utf8'));

function fail(msg) {
  console.error(`sync-brand-icons: ${msg}`);
  process.exit(1);
}

async function loadTarball() {
  const i = process.argv.indexOf('--from');
  if (i > 0) return readFileSync(process.argv[i + 1]);
  const registry = (process.env.NPM_REGISTRY || 'https://registry.npmjs.org').replace(/\/$/, '');
  const base = cfg.package.split('/')[1];
  const url = `${registry}/${cfg.package}/-/${base}-${cfg.version}.tgz`;
  const res = await fetch(url);
  if (!res.ok) fail(`download ${url}: HTTP ${res.status}`);
  return Buffer.from(await res.arrayBuffer());
}

function untar(buf) {
  const files = new Map();
  for (let off = 0; off + 512 <= buf.length; ) {
    const header = buf.subarray(off, off + 512);
    if (header.every((b) => b === 0)) break;
    const str = (s, n) => header.subarray(s, s + n).toString('utf8').replace(/\0.*$/s, '');
    const name = str(0, 100);
    const prefix = str(345, 155);
    const size = parseInt(str(124, 12).trim() || '0', 8);
    const type = String.fromCharCode(header[156] || 48);
    off += 512;
    if (type === '0') files.set(prefix ? `${prefix}/${name}` : name, buf.subarray(off, off + size));
    off += Math.ceil(size / 512) * 512;
  }
  return files;
}

function checkSvg(name, text) {
  if (!/^\s*<svg[\s>]/.test(text)) fail(`${name}: not an <svg> document`);
  if (/<script|<foreignObject|\son[a-z]+\s*=|javascript:/i.test(text)) fail(`${name}: unsafe content`);
  if (/(?:xlink:)?href\s*=\s*["'](?!#)/i.test(text)) fail(`${name}: external reference`);
}

function sortObj(obj) {
  return Object.fromEntries(Object.keys(obj).sort().map((k) => [k, obj[k]]));
}

const tgz = await loadTarball();
const integrity = `sha512-${createHash('sha512').update(tgz).digest('base64')}`;
if (integrity !== cfg.integrity) fail(`integrity mismatch: got ${integrity}`);
const files = untar(gunzipSync(tgz));

const icons = {};
const out = new Map();
for (const key of [...new Set(cfg.icons)].sort()) {
  if (!/^[a-z0-9]+$/.test(key)) fail(`invalid icon key ${key}`);
  const mono = files.get(`package/icons/${key}.svg`);
  if (!mono) fail(`icon ${key}.svg missing from ${cfg.package}@${cfg.version}`);
  const color = files.get(`package/icons/${key}-color.svg`);
  for (const [file, data] of [[`${key}.svg`, mono], [`${key}-color.svg`, color]]) {
    if (!data) continue;
    checkSvg(file, data.toString('utf8'));
    out.set(file, data);
  }
  icons[key] = { mono: true, color: Boolean(color), scale: cfg.scale?.[key] ?? 1 };
}

const total = [...out.values()].reduce((n, b) => n + b.length, 0);
if (total >= MAX_TOTAL_BYTES) fail(`total SVG size ${total} bytes exceeds ${MAX_TOTAL_BYTES}`);

const mapping = { aliases: cfg.aliases, channelTypes: cfg.channelTypes, modelPrefixes: cfg.modelPrefixes, domains: cfg.domains };
for (const [section, table] of Object.entries(mapping)) {
  for (const [from, to] of Object.entries(table)) {
    if (!icons[to]) fail(`${section}.${from} -> ${to}: unknown icon key`);
    if (from !== from.toLowerCase()) fail(`${section}.${from}: keys must be lowercase`);
  }
}
for (const alias of Object.keys(cfg.aliases)) {
  if (icons[alias]) fail(`alias ${alias} shadows an icon key`);
}

const manifest = {
  version: cfg.version,
  source: `${cfg.package}@${cfg.version}`,
  icons,
  aliases: sortObj(cfg.aliases),
  channelTypes: Object.fromEntries(Object.keys(cfg.channelTypes).sort((a, b) => a - b).map((k) => [k, cfg.channelTypes[k]])),
  modelPrefixes: sortObj(cfg.modelPrefixes),
  domains: sortObj(cfg.domains),
};

rmSync(assetsDir, { recursive: true, force: true });
mkdirSync(assetsDir, { recursive: true });
for (const [file, data] of out) writeFileSync(join(assetsDir, file), data);
writeFileSync(join(outDir, 'manifest.json'), `${JSON.stringify(manifest, null, 2)}\n`);

console.log(`sync-brand-icons: ${Object.keys(icons).length} icons, ${out.size} files, ${total} bytes -> ${readdirSync(assetsDir).length} assets`);
