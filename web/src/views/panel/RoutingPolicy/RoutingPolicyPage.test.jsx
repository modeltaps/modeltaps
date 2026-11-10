// @vitest-environment jsdom
import { createElement as h } from 'react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import { OrgContext } from 'contexts/OrgContext';
import RoutingPolicy, { RoutingPolicyList } from './index';
import { orgRoutingPolicyApi } from './routingPolicyApi';

// The policy list opens a policy's panel from its row (?id=); each tab saves on its own and
// sends the whole policy with only that tab's fields changed.

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() } }));
vi.mock('utils/common', async (orig) => ({ ...(await orig()), showError: vi.fn(), showSuccess: vi.fn() }));

const POLICY = {
  id: 3,
  name: 'auto/coder',
  display_name: '',
  description: '',
  enabled: true,
  decision_model: 'small',
  question: 'q',
  routes: [
    { label: 'a', description: '', target_model: 'm1' },
    { label: 'b', description: '', target_model: 'm2' }
  ],
  fallback_model: 'big',
  scoring: 'auto',
  threshold: 0.5,
  pin_ttl_seconds: 3600,
  max_input_chars: 4000,
  decision_timeout_ms: 3000
};

// Shows the current query string so the tests can read what the page put in the URL.
function Spy() {
  return h('output', { 'data-testid': 'search' }, useLocation().search);
}
const search = () => screen.getByTestId('search').textContent;

beforeEach(() => {
  vi.clearAllMocks();
  API.get.mockImplementation((url) => {
    if (url === '/api/routing_policy/') return Promise.resolve({ data: { success: true, data: [POLICY] } });
    if (url === '/api/routing_policy/3') return Promise.resolve({ data: { success: true, data: POLICY } });
    if (url === '/api/routing_policy/3/stats') return Promise.resolve({ data: { success: true, data: { requests: 0 } } });
    return Promise.resolve({ data: { success: true, data: [] } });
  });
  API.put.mockResolvedValue({ data: { success: true, message: '' } });
});

afterEach(cleanup);

const renderPage = (entry = '/panel/routing_policy') =>
  render(
    h(
      MemoryRouter,
      { initialEntries: [entry] },
      h(Routes, null, h(Route, { path: '/panel/routing_policy', element: h('div', null, h(RoutingPolicy), h(Spy)) }))
    )
  );

describe('RoutingPolicy page', () => {
  it('opens the policy panel from its row and keeps it in the URL', async () => {
    renderPage();
    fireEvent.click((await screen.findAllByText('auto/coder'))[0]);
    await screen.findByRole('tab', { name: i18n.t('routingPolicy.panel.tab.overview'), selected: true });
    expect(search()).toBe('?id=3');
  });

  it('saves the settings tab with the rest of the policy unchanged', async () => {
    renderPage('/panel/routing_policy?id=3&tab=settings');
    const input = await screen.findByLabelText(i18n.t('routingPolicy.form.displayName'), { exact: false });
    fireEvent.change(input, { target: { value: 'Coder' } });
    fireEvent.click(screen.getByRole('button', { name: i18n.t('objects.panel.save') }));
    await waitFor(() => expect(API.put).toHaveBeenCalled());
    const [url, body] = API.put.mock.calls[0];
    expect(url).toBe('/api/routing_policy/3');
    expect(body.display_name).toBe('Coder');
    expect(body.routes).toEqual(POLICY.routes);
    expect(body.decision_model).toBe('small');
    expect(body.enabled).toBe(true);
  });

  it('opens the new policy sheet and lists what is still missing', async () => {
    renderPage('/panel/routing_policy?new=1');
    const create = await screen.findByRole('button', { name: i18n.t('objects.new.create') });
    expect(create.disabled).toBe(true);
    const missing = i18n.t('objects.new.missing', {
      items: ['name', 'decisionModel', 'routes', 'fallbackModel'].map((k) => i18n.t(`routingPolicy.form.${k}`)).join(i18n.t('objects.new.listSep'))
    });
    expect(screen.getByText(missing)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: i18n.t('common.cancel') }));
    await waitFor(() => expect(search()).toBe(''));
  });
});

const ORG_POLICY = { ...POLICY, id: 8, organization_id: 4, organization_name: 'Acme', overrides_platform: true };

describe('RoutingPolicy page, organization scope', () => {
  const renderReview = (entry) =>
    render(
      h(
        OrgContext.Provider,
        { value: { orgEnabled: true } },
        h(
          MemoryRouter,
          { initialEntries: [entry] },
          h(Routes, null, h(Route, { path: '/panel/routing_policy', element: h('div', null, h(RoutingPolicy), h(Spy)) }))
        )
      )
    );

  beforeEach(() => {
    API.get.mockImplementation((url, config) => {
      if (url === '/api/routing_policy/' && config?.params?.organization_id === -1) {
        return Promise.resolve({ data: { success: true, data: [ORG_POLICY] } });
      }
      if (url === '/api/routing_policy/8') return Promise.resolve({ data: { success: true, data: ORG_POLICY } });
      return Promise.resolve({ data: { success: true, data: [] } });
    });
  });

  it('switches to every organization policy and keeps the scope in the URL', async () => {
    renderReview('/panel/routing_policy');
    fireEvent.click(await screen.findByRole('radio', { name: i18n.t('routingPolicy.scope.organization') }));
    expect((await screen.findAllByText('Acme')).length).toBeGreaterThan(0);
    expect(search()).toBe('?scope=organization');
    expect(screen.getAllByText(i18n.t('routingPolicy.overridesPlatform')).length).toBeGreaterThan(0);
  });

  it('opens an organization policy read-only: overview only, the conflict explained', async () => {
    renderReview('/panel/routing_policy?scope=organization&id=8');
    await screen.findByText(i18n.t('routingPolicy.overridesPlatformHint'));
    expect(screen.queryByRole('tab', { name: i18n.t('routingPolicy.panel.tab.routes') })).toBeNull();
    expect(screen.queryByRole('button', { name: i18n.t('objects.panel.save') })).toBeNull();
  });
});

describe('RoutingPolicyList, organization client', () => {
  it('uses the organization endpoints and bills Try it', async () => {
    API.get.mockImplementation((url) => {
      if (url === '/api/org/5/routing_policy/') return Promise.resolve({ data: { success: true, data: [POLICY] } });
      if (url === '/api/org/5/routing_policy/3') return Promise.resolve({ data: { success: true, data: POLICY } });
      return Promise.resolve({ data: { success: true, data: [] } });
    });
    const api = orgRoutingPolicyApi(5);
    render(
      h(
        MemoryRouter,
        { initialEntries: ['/org?id=3&tab=try'] },
        h(Routes, null, h(Route, { path: '/org', element: h(RoutingPolicyList, { api }) }))
      )
    );
    await screen.findByText(i18n.t('routingPolicy.tryIt.introBilled'));
    expect(API.get).toHaveBeenCalledWith('/api/org/5/routing_policy/models');
    expect(API.get).toHaveBeenCalledWith('/api/org/5/routing_policy/3');
  });
});
