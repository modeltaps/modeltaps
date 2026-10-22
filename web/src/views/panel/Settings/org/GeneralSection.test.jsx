// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import GeneralSection from './GeneralSection';

// 「成员可见全员用量」是组织上的普通开关:未设置即关闭,不再显示「沿用站点默认」的第三态。

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn(), put: vi.fn() } }));
vi.mock('utils/common', () => ({ showError: vi.fn(), showSuccess: vi.fn() }));
vi.mock('react-redux', () => ({ useSelector: (select) => select({ siteInfo: {} }) }));
vi.mock('contexts/OrgContext', () => ({ useOrg: () => ({ refreshOrganizations: vi.fn(), refreshOrgDetail: vi.fn() }) }));
vi.mock('hooks/useHighlightTarget', () => ({ default: () => {} }));
vi.mock('./DangerSection', () => ({ default: () => null }));

beforeEach(() => {
  vi.clearAllMocks();
  API.put.mockResolvedValue({ data: { success: true, message: '' } });
});

afterEach(cleanup);

const renderSection = (setting) =>
  render(
    createElement(GeneralSection, {
      orgId: 3,
      role: 'owner',
      detail: { organization: { id: 3, name: 'Acme', avatar_url: '', setting } },
      onRefresh: vi.fn()
    })
  );

const usageSwitch = () =>
  screen.getByText(i18n.t('orgPage.settings.usageVisible')).closest('.justify-between').querySelector('[role="switch"]');

const save = () => fireEvent.click(screen.getByRole('button', { name: i18n.t('orgPage.settings.save') }));

describe('GeneralSection usage visibility', () => {
  it('shows an unset value as off and saves the switch once toggled', async () => {
    renderSection({});
    expect(usageSwitch().getAttribute('aria-checked')).toBe('false');

    fireEvent.click(usageSwitch());
    save();
    await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
    expect(API.put.mock.calls[0][1].usage_visible_to_members).toBe(true);
  });

  it('shows an explicit true as on', () => {
    renderSection({ usage_visible_to_members: true });
    expect(usageSwitch().getAttribute('aria-checked')).toBe('true');
  });

  it('leaves the field out when only the name changes', async () => {
    renderSection({ usage_visible_to_members: false });
    fireEvent.change(document.getElementById('org-settings-name'), { target: { value: 'Acme 2' } });
    save();
    await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
    expect(API.put.mock.calls[0][1]).not.toHaveProperty('usage_visible_to_members');
  });
});
