import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() } }));

import { API } from 'utils/api';
import { organizationReviewApi, orgRoutingPolicyApi, platformRoutingPolicyApi } from './routingPolicyApi';

describe('routing policy api scopes', () => {
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
