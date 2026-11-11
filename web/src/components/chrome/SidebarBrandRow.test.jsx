// @vitest-environment jsdom
import { createElement } from 'react';
import { MemoryRouter } from 'react-router';
import { Provider } from 'react-redux';
import { legacy_createStore as createStore } from 'redux';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { TooltipProvider } from '@/components/ui/tooltip';
import SidebarBrandRow from './SidebarBrandRow';

// The first row of every sidebar: brand, search, and the collapse button only when the sidebar
// passes one (narrow screens and the phone drawer; wide screens keep the rail expanded).

afterEach(cleanup);

const store = createStore(() => ({ siteInfo: { system_name: 'Modeltaps' } }));

const renderRow = (props) =>
  render(
    createElement(
      Provider,
      { store },
      createElement(MemoryRouter, null, createElement(TooltipProvider, null, createElement(SidebarBrandRow, props)))
    )
  );

describe('SidebarBrandRow', () => {
  it('links the brand to the dashboard and opens the search palette from its search button', () => {
    const opened = [];
    renderRow({ onOpenCommand: () => opened.push(true) });
    expect(screen.getAllByRole('link')[0].getAttribute('href')).toBe('/panel/dashboard');
    fireEvent.click(screen.getByRole('button', { name: '搜索' }));
    expect(opened).toHaveLength(1);
  });

  it('has no collapse button unless one is passed', () => {
    renderRow({ onOpenCommand: () => {} });
    expect(screen.queryByRole('button', { name: '收起侧栏' })).toBeNull();
    cleanup();
    renderRow({ onOpenCommand: () => {}, onToggleCollapse: () => {} });
    expect(screen.getByRole('button', { name: '收起侧栏' })).toBeTruthy();
  });
});
