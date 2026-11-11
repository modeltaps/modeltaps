// @vitest-environment jsdom
import { createElement } from 'react';
import { MemoryRouter } from 'react-router';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { TooltipProvider } from '@/components/ui/tooltip';
import { rememberAdminReturn } from 'utils/adminReturn';
import { AdminBackRow } from './Sidebar';

// Platform admin has no space switcher; one row leads back to the workbench page the user came from.

afterEach(() => {
  cleanup();
  sessionStorage.clear();
});

const renderRow = (props = {}) =>
  render(
    createElement(
      MemoryRouter,
      { initialEntries: ['/panel/analytics'] },
      createElement(TooltipProvider, null, createElement(AdminBackRow, props))
    )
  );

describe('AdminBackRow', () => {
  it('returns to the last workbench page', () => {
    rememberAdminReturn('/panel/log?p=2');
    renderRow();
    const link = screen.getByRole('link', { name: '返回工作台' });
    expect(link.getAttribute('href')).toBe('/panel/log?p=2');
    expect(link.textContent).toContain('工作台');
  });

  it('falls back to the dashboard', () => {
    renderRow({ collapsed: true });
    expect(screen.getByRole('link', { name: '返回工作台' }).getAttribute('href')).toBe('/panel/dashboard');
  });
});
