import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() } }));

import { API } from 'utils/api';
import {
  organizationReviewApi,
  orgRoutingPolicyApi,
  personalReviewApi,
  personalRoutingPolicyApi,
  platformRoutingPolicyApi,
  policyCallName
} from './routingPolicyApi';

describe('routing policy api scopes', () => {
  it('sends personal calls under /api/user/routing_policy/ and bills tries', async () => {
    API.get.mockResolvedValue({ data: { success: true, data: ['decider'] } });
    await personalRoutingPolicyApi.list();
    expect(API.get).toHaveBeenCalledWith('/api/user/routing_policy/', undefined);
    expect(await personalRoutingPolicyApi.listModels()).toEqual(['decider']);
    expect(API.get).toHaveBeenCalledWith('/api/user/routing_policy/models');
    await personalRoutingPolicyApi.listManaged();
    expect(API.get).toHaveBeenCalledWith('/api/user/routing_policy/managed');
    expect(personalRoutingPolicyApi.billedTry).toBe(true);
    expect(personalRoutingPolicyApi.getSamples).toBeTypeOf('function');
  });

  it('calls own policies as policy/<name> and platform ones by name', () => {
    expect(policyCallName(personalRoutingPolicyApi, { name: 'auto/coder' })).toBe('policy/auto/coder');
    expect(policyCallName(orgRoutingPolicyApi(5), { name: 'auto/coder' })).toBe('policy/auto/coder');
    expect(policyCallName(platformRoutingPolicyApi, { name: 'auto/coder' })).toBe('auto/coder');
  });

  it('gives organization members a read-only client', () => {
    expect(orgRoutingPolicyApi(5, { readOnly: true }).readOnly).toBe(true);
    expect(orgRoutingPolicyApi(5).readOnly).toBe(false);
  });

  it('reviews every personal policy from the platform console', async () => {
    API.get.mockResolvedValue({ data: { success: true, data: [] } });
    await personalReviewApi.list();
    expect(API.get).toHaveBeenCalledWith('/api/routing_policy/', { params: { scope: 'personal' } });
    expect(personalReviewApi.review).toBe(true);
  });

  beforeEach(() => {
    vi.clearAllMocks();
    API.get.mockResolvedValue({ data: { success: true, data: ['decider', 'cheap-model'] } });
    API.post.mockResolvedValue({ data: { success: true } });
    API.put.mockResolvedValue({ data: { success: true } });
  });

  it('sends organization calls under /api/org/:id/routing_policy/', async () => {
    const api = orgRoutingPolicyApi(5);
    await api.list();
    expect(API.get).toHaveBeenCalledWith('/api/org/5/routing_policy/', undefined);
    await api.setEnabled(9, false);
    expect(API.put).toHaveBeenCalledWith('/api/org/5/routing_policy/9/status', { enabled: false });
    await api.tryPolicy({ policy: { name: 'x' }, text: 'hi', decisionModels: ['decider'] });
    expect(API.post).toHaveBeenCalledWith('/api/org/5/routing_policy/try', {
      policy: { name: 'x' },
      text: 'hi',
      decision_models: ['decider']
    });
    expect(await api.listModels()).toEqual(['decider', 'cheap-model']);
    expect(API.get).toHaveBeenCalledWith('/api/org/5/routing_policy/models');
  });

  it('has no sample replay and bills tries in organizations only', () => {
    expect(orgRoutingPolicyApi(5).getSamples).toBeNull();
    expect(orgRoutingPolicyApi(5).billedTry).toBe(true);
    expect(platformRoutingPolicyApi.getSamples).toBeTypeOf('function');
    expect(platformRoutingPolicyApi.billedTry).toBe(false);
  });

  it('reviews every organization policy from the platform console', async () => {
    await organizationReviewApi.list();
    expect(API.get).toHaveBeenCalledWith('/api/routing_policy/', { params: { organization_id: -1 } });
    expect(organizationReviewApi.review).toBe(true);
    expect(organizationReviewApi.getSamples).toBeNull();
    expect(platformRoutingPolicyApi.review).toBe(false);
  });
});
