import { readdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { describe, expect, it } from 'vitest';

// Layering contract for floating content that is portaled to document.body.
// Such content escapes its parent's stacking context, so its z-index must be
// at least as high as every full-screen layer it can be opened from: the
// mobile sidebar drawer (MainLayout) and the dialog / sheet overlays. A
// portal that sits below any of those renders behind it and looks "dead"
// (regression: S5 user menu in the mobile drawer, 2026-09-06).

const uiDir = dirname(fileURLToPath(import.meta.url));
const read = (p) => readFileSync(p, 'utf8');
// Ignore full-line comments so documentation of the rule does not count as usage.
const stripComments = (src) => src.replace(/^\s*\/\/.*$/gm, '');
const zTokens = (src) => [...stripComments(src).matchAll(/\bz-(?:\[(\d+)\]|(\d+))(?![\w-])/g)].map((m) => Number(m[1] ?? m[2]));
const firstZ = (src, re, label) => {
  const m = src.match(re);
  if (!m) throw new Error(`cannot find z-index for ${label}`);
  return Number(m[1]);
};

describe('portal overlay layering', () => {
  const drawerZ = firstZ(read(join(uiDir, '../../layout/MobilePushFrame.jsx')), /absolute inset-y-0 z-\[(\d+)\]/, 'mobile push drawer');
  const dialogZ = firstZ(read(join(uiDir, 'dialog.jsx')), /fixed inset-0 z-\[(\d+)\]/, 'dialog overlay');
  const sheetZ = firstZ(read(join(uiDir, 'sheet.jsx')), /fixed inset-0 z-\[(\d+)\]/, 'sheet overlay');
  const floor = Math.max(drawerZ, dialogZ, sheetZ);

  const portalFiles = readdirSync(uiDir).filter((f) => f.endsWith('.jsx') && read(join(uiDir, f)).includes('createPortal'));

  it('has at least one body-portaled primitive to check', () => {
    expect(portalFiles).toContain('dropdown-menu.jsx');
  });

  it.each(portalFiles)('%s portal content stacks at or above drawer / dialog / sheet layers', (file) => {
    const zs = zTokens(read(join(uiDir, file)));
    expect(zs.length).toBeGreaterThan(0);
    for (const z of zs) expect(z, `${file} uses z-index ${z}, floor is ${floor}`).toBeGreaterThanOrEqual(floor);
  });
});
