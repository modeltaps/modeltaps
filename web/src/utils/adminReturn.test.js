// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';

import { adminReturnPath, rememberAdminReturn } from './adminReturn';

afterEach(() => sessionStorage.clear());

describe('admin return path', () => {
  it('remembers the last workbench page', () => {
    rememberAdminReturn('/panel/playground/speech?tab=stt');
    expect(adminReturnPath()).toBe('/panel/playground/speech?tab=stt');
  });

  // Before the user loads, MainLayout takes an admin page for a workbench page.
  it('never records a platform admin page', () => {
    rememberAdminReturn('/panel/logs');
    rememberAdminReturn('/panel/admin/models?tab=prices');
    expect(adminReturnPath()).toBe('/panel/logs');
  });

  it('ignores an admin page already stored and falls back to the dashboard', () => {
    sessionStorage.setItem('admin_return_path', '/panel/admin/models');
    expect(adminReturnPath()).toBe('/panel/dashboard');
  });
});
