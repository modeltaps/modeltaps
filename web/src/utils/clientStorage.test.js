import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

// SEC-21: the sign-in lives in the httpOnly session cookie and the signed-in user only in memory
// (Redux, loaded from /api/user/self). Browser storage is readable by any script on the page, so
// it never holds the user record or a credential. `removeItem('user')` stays: it clears the
// record older versions left behind.

const srcDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

function sourceFiles(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) return entry.name === 'i18n' ? [] : sourceFiles(full);
    return /\.(jsx?|tsx?)$/.test(entry.name) && !/\.test\./.test(entry.name) ? [full] : [];
  });
}

const STORAGE_CALL = /\b(?:localStorage|sessionStorage|storage)\??\.(setItem|getItem)\(\s*['"`]([^'"`]+)['"`]/g;
const SENSITIVE_KEY = /^(user|token|access_?token|refresh_?token|session|key|api_?key|password)$/i;

describe('browser storage', () => {
  it('holds no user record or credential', () => {
    const hits = [];
    for (const file of sourceFiles(srcDir)) {
      for (const [call, , key] of readFileSync(file, 'utf8').matchAll(STORAGE_CALL)) {
        if (SENSITIVE_KEY.test(key)) hits.push(`${path.relative(srcDir, file)}: ${call}`);
      }
    }
    expect(hits).toEqual([]);
  });
});
