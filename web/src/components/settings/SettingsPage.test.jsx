// @vitest-environment jsdom
import { createElement, useState } from 'react';
import { MemoryRouter } from 'react-router';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

import i18n from 'i18n/i18n';
import SettingsPage from './SettingsPage';
import { CollectionCard, FormCard } from './cards';
import { SettingsPlaneContext } from './SettingsPlane';
import { SearchContext } from '@/components/chrome/SearchContext';

// The shared settings page: groups stay mounted (edits survive a switch), the search palette lists
// the cards with what they render across groups, form cards report unsaved edits as a dot.

beforeAll(() => i18n.changeLanguage('en_US'));
afterAll(() => i18n.changeLanguage('zh_CN'));
afterEach(cleanup);

function NameCard({ title }) {
  const [value, setValue] = useState('a');
  const [saved, setSaved] = useState('a');
  return createElement(
    FormCard,
    { title, hint: 'Footer hint', dirty: value !== saved, onSave: () => setSaved(value), onReset: () => setValue(saved) },
    createElement('label', { htmlFor: title }, `${title} field`),
    createElement('input', { id: title, value, onChange: (e) => setValue(e.target.value) })
  );
}

const rows = [{ id: 1, name: 'Webhook relay' }];
const groups = ['alpha', 'beta', 'gamma', 'delta'].map((id) => ({
  id,
  to: `/s/${id}`,
  label: `Group ${id}`,
  cards:
    id === 'beta'
      ? [
          { id: 'beta-form', render: () => createElement(NameCard, { title: 'Mail sending' }) },
          {
            id: 'beta-list',
            render: () => createElement(CollectionCard, { title: 'Bots', rows, columns: [{ key: 'name', header: 'Name' }] })
          }
        ]
      : [{ id: `${id}-form`, render: () => createElement(NameCard, { title: `Card ${id}` }) }]
}));

const renderPage = (active = 'alpha') =>
  render(createElement(MemoryRouter, null, createElement(SettingsPage, { label: 'Groups', groups, activeGroupId: active })));

const visible = (text) => !screen.getByText(text).closest('[hidden]');
const link = (id) => screen.getByRole('link', { name: new RegExp(`Group ${id}`) });

describe('SettingsPage', () => {
  it('shows the active group and keeps the others mounted but hidden', () => {
    renderPage('alpha');
    expect(visible('Card alpha')).toBe(true);
    expect(visible('Mail sending')).toBe(false);
    expect(link('alpha').getAttribute('aria-current')).toBe('page');
  });

  it('lists every card in the search palette with what it renders, collection rows included', () => {
    const registered = [];
    const openSearch = () => registered.push('opened');
    const search = { openSearch, registerPageIndex: (fn) => (registered.push(fn), () => registered.push('unregistered')) };
    const view = render(
      createElement(
        MemoryRouter,
        null,
        createElement(
          SearchContext.Provider,
          { value: search },
          createElement(SettingsPage, { label: 'Groups', groups, activeGroupId: 'alpha' })
        )
      )
    );
    const index = registered.find((r) => typeof r === 'function')();
    expect(index.map((e) => e.id)).toEqual([
      'alpha:alpha-form',
      'beta:beta-form',
      'beta:beta-list',
      'gamma:gamma-form',
      'delta:delta-form'
    ]);
    const bots = index.find((e) => e.id === 'beta:beta-list');
    expect(bots).toMatchObject({ title: 'Bots', group: 'Group beta', to: '/s/beta' });
    expect(bots.text).toContain('webhook relay');

    // The subnav's search button opens the palette; there is no search box on the page.
    expect(screen.queryByRole('searchbox')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /Search settings/ }));
    expect(registered).toContain('opened');
    view.unmount();
    expect(registered).toContain('unregistered');
  });

  it('marks the group of a dirty form card, and Undo clears it', () => {
    renderPage('beta');
    expect(link('beta').textContent).not.toContain('Unsaved changes');
    fireEvent.change(screen.getByLabelText('Mail sending field'), { target: { value: 'b' } });
    expect(link('beta').textContent).toContain('Unsaved changes');
    expect(screen.getAllByText('Unsaved changes').length).toBeGreaterThan(1);
    act(() => fireEvent.click(screen.getByRole('button', { name: 'Undo' })));
    expect(link('beta').textContent).not.toContain('Unsaved changes');
    expect(screen.queryAllByText('Unsaved changes', { selector: 'p' })).toHaveLength(0);
  });

  it('saving clears the dot', () => {
    renderPage('beta');
    fireEvent.change(screen.getByLabelText('Mail sending field'), { target: { value: 'b' } });
    const save = screen.getAllByRole('button', { name: 'Save' }).find((b) => !b.disabled);
    act(() => fireEvent.click(save));
    expect(link('beta').textContent).not.toContain('Unsaved changes');
  });
});

describe('SettingsPage on 设置 (its subnav lives in the settings sidebar)', () => {
  const renderInPlane = (plane, active) =>
    render(
      createElement(
        MemoryRouter,
        null,
        createElement(
          SettingsPlaneContext.Provider,
          { value: plane },
          createElement(SettingsPage, { label: 'Groups', groups, activeGroupId: active })
        )
      )
    );
  const planeWith = (published) => ({ publish: (status) => published.push(status), dirtyGroups: [] });

  it('shows only the cards of the active group and reports unsaved groups to the sidebar', () => {
    const published = [];
    renderInPlane(planeWith(published), 'beta');
    expect(screen.queryByRole('navigation')).toBeNull();
    expect(screen.queryByRole('heading', { name: 'Group beta' })).toBeNull();
    expect(visible('Mail sending')).toBe(true);
    expect(visible('Card alpha')).toBe(false);
    expect(published.at(-1)).toEqual({ dirtyGroups: [] });

    fireEvent.change(screen.getByLabelText('Mail sending field'), { target: { value: 'b' } });
    expect(published.at(-1)).toEqual({ dirtyGroups: ['beta'] });
  });

  it('names the part of a group in the palette entries', () => {
    const parted = groups.map((g, i) => ({ ...g, sectionLabel: i < 2 ? 'Organization settings' : 'Personal settings' }));
    let index = null;
    const search = { openSearch: () => {}, registerPageIndex: (fn) => ((index = fn), () => {}) };
    render(
      createElement(
        MemoryRouter,
        null,
        createElement(
          SearchContext.Provider,
          { value: search },
          createElement(
            SettingsPlaneContext.Provider,
            { value: planeWith([]) },
            createElement(SettingsPage, { label: 'Groups', groups: parted, activeGroupId: 'alpha' })
          )
        )
      )
    );
    expect(index().find((e) => e.id === 'delta:delta-form').group).toBe('Personal settings · Group delta');
  });
});
