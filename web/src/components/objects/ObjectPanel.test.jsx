// @vitest-environment jsdom
import { createElement as h, useState } from 'react';
import PropTypes from 'prop-types';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import ObjectPanel from './ObjectPanel';

vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function NameBody({ draft, setDraft }) {
  return h('input', { 'aria-label': 'name', value: draft.name, onChange: (e) => setDraft({ ...draft, name: e.target.value }) });
}
NameBody.propTypes = { draft: PropTypes.object, setDraft: PropTypes.func };

function InfoBody({ object }) {
  return h('p', null, `info ${object.name}`);
}
InfoBody.propTypes = { object: PropTypes.object };

const TABS = [
  { key: 'info', label: 'Info', Body: InfoBody },
  {
    key: 'name',
    label: 'Name',
    make: (o) => ({ name: o.name }),
    patch: (d) => (d.name ? { name: d.name } : { error: 'required' }),
    Body: NameBody
  }
];

function Harness({ onSave, onClose = () => {}, initialTab = 'info' }) {
  const [object, setObject] = useState({ id: 1, name: 'alpha' });
  const [tab, setTab] = useState(initialTab);
  return h(ObjectPanel, {
    titleId: 't',
    object,
    title: object.name,
    tabs: TABS,
    tab,
    onTabChange: setTab,
    onClose,
    onSave: async (patch) => {
      const res = await onSave(patch);
      return res;
    },
    onSaved: async () => setObject((o) => ({ ...o, ...onSave.mock.calls.at(-1)[0] }))
  });
}
Harness.propTypes = { onSave: PropTypes.func, onClose: PropTypes.func, initialTab: PropTypes.string };

const save = () => screen.getByRole('button', { name: i18n.t('objects.panel.save') });

describe('ObjectPanel', () => {
  it('shows no footer on a read-only tab', () => {
    render(h(Harness, { onSave: vi.fn() }));
    expect(screen.getByText('info alpha')).toBeTruthy();
    expect(screen.queryByRole('button', { name: i18n.t('objects.panel.save') })).toBeNull();
  });

  it('saves only the open tab and resets it from the stored object', async () => {
    const onSave = vi.fn().mockResolvedValue({ success: true });
    render(h(Harness, { onSave, initialTab: 'name' }));
    expect(save().disabled).toBe(true);
    fireEvent.change(screen.getByLabelText('name'), { target: { value: 'beta' } });
    expect(save().disabled).toBe(false);
    expect(screen.getAllByText(i18n.t('objects.panel.unsaved')).length).toBeGreaterThan(0);
    fireEvent.click(save());
    await waitFor(() => expect(onSave).toHaveBeenCalledWith({ name: 'beta' }));
    await waitFor(() => expect(save().disabled).toBe(true));
    expect(screen.getByLabelText('name').value).toBe('beta');
  });

  it('discards edits and refuses a patch with an error', async () => {
    const onSave = vi.fn();
    render(h(Harness, { onSave, initialTab: 'name' }));
    fireEvent.change(screen.getByLabelText('name'), { target: { value: '' } });
    fireEvent.click(save());
    expect(onSave).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: i18n.t('objects.panel.discard') }));
    expect(screen.getByLabelText('name').value).toBe('alpha');
  });

  it('asks before switching tabs or closing with unsaved changes', () => {
    const onClose = vi.fn();
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
    render(h(Harness, { onSave: vi.fn(), onClose, initialTab: 'name' }));
    fireEvent.change(screen.getByLabelText('name'), { target: { value: 'gamma' } });

    fireEvent.click(screen.getByRole('tab', { name: 'Info' }));
    expect(confirm).toHaveBeenCalledWith(i18n.t('objects.panel.switchConfirm'));
    expect(screen.getByLabelText('name').value).toBe('gamma');

    fireEvent.click(screen.getByRole('button', { name: i18n.t('common.close') }));
    expect(onClose).not.toHaveBeenCalled();

    confirm.mockReturnValue(true);
    fireEvent.click(screen.getByRole('tab', { name: 'Info' }));
    expect(screen.getByText('info alpha')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: i18n.t('common.close') }));
    expect(onClose).toHaveBeenCalled();
  });
});
