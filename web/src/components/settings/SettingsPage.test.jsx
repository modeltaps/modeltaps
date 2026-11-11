// @vitest-environment jsdom
import { createElement, useState } from 'react';
import { MemoryRouter } from 'react-router';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

import i18n from 'i18n/i18n';
import SettingsPage from './SettingsPage';
import { CollectionCard, FormCard } from './cards';
import { SettingsPlaneContext } from './SettingsPlane';

// The shared settings page: groups stay mounted (edits survive a switch), search matches what the
// cards render across groups, form cards report unsaved edits as a dot on their group.

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

  it('searches card text across groups, collection rows included', () => {
    renderPage('alpha');
    const box = screen.getByRole('searchbox', { name: 'Search settings' });
    fireEvent.change(box, { target: { value: 'webhook' } });
    expect(visible('Bots')).toBe(true);
    expect(visible('Card alpha')).toBe(false);
    expect(screen.getByText('1 card')).toBeTruthy();

    fireEvent.change(box, { target: { value: 'nothing like this' } });
    expect(screen.getByText('No matching settings')).toBeTruthy();
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
  const planeWith = (query, published) => ({ query, setQuery: () => {}, publish: (status) => published.push(status) });

  it('shows only the cards of the active group and reports unsaved groups to the sidebar', () => {
    const published = [];
    renderInPlane(planeWith('', published), 'beta');
    expect(screen.queryByRole('navigation')).toBeNull();
    expect(screen.queryByRole('searchbox')).toBeNull();
    expect(visible('Mail sending')).toBe(true);
    expect(visible('Card alpha')).toBe(false);
    expect(published.at(-1)).toEqual({ searchable: true, dirtyGroups: [], counts: null });

    fireEvent.change(screen.getByLabelText('Mail sending field'), { target: { value: 'b' } });
    expect(published.at(-1)).toEqual({ searchable: true, dirtyGroups: ['beta'], counts: null });
  });

  it("searches with the sidebar's query and reports the matches per group", () => {
    const published = [];
    renderInPlane(planeWith('webhook', published), 'alpha');
    expect(visible('Bots')).toBe(true);
    expect(visible('Card alpha')).toBe(false);
    expect(published.at(-1).counts).toEqual({ alpha: 0, beta: 1, gamma: 0, delta: 0 });
  });

  it('names the part of a group in search results', () => {
    const parted = groups.map((g, i) => ({ ...g, sectionLabel: i < 2 ? 'Organization settings' : 'Personal settings' }));
    render(
      createElement(
        MemoryRouter,
        null,
        createElement(
          SettingsPlaneContext.Provider,
          { value: planeWith('card delta', []) },
          createElement(SettingsPage, { label: 'Groups', groups: parted, activeGroupId: 'alpha' })
        )
      )
    );
    expect(screen.getByText('Personal settings · Group delta')).toBeTruthy();
  });
});
