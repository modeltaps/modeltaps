import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';
import { describe, expect, it } from 'vitest';

// Console URLs are built with routes/paths.js only, so the scheme can change in one place.
// Comments may mention paths; code may not hold a '/panel...' string literal.

const SRC = join(__dirname, '..');
const ALLOWED = new Set(['routes/paths.js', 'routes/legacyPaths.js']);

function files(dir) {
  return readdirSync(dir).flatMap((name) => {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) return name === 'node_modules' ? [] : files(full);
    return /\.(js|jsx)$/.test(name) && !/\.test\.(js|jsx)$/.test(name) ? [full] : [];
  });
}

// Drops comments (line and block) so documentation can name paths.
const code = (text) => text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:'"`])\/\/.*$/gm, '$1');

describe('console URLs', () => {
  it('are not written as string literals outside routes/paths.js', () => {
    const offenders = files(SRC)
      .filter((f) => !ALLOWED.has(relative(SRC, f)))
      .filter((f) => /['"`]\/panel(\/|['"`?$])/.test(code(readFileSync(f, 'utf8'))))
      .map((f) => relative(SRC, f));
    expect(offenders).toEqual([]);
  });
});
