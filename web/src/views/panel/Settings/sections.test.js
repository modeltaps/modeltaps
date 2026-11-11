import { describe, expect, it } from 'vitest';

import { resolveAccountSection, resolveOrgSection } from './sections';

describe('personal and organization settings sections', () => {
  it('account groups stay, the old access token section lands on its card', () => {
    for (const group of ['profile', 'security', 'privacy', 'preferences']) expect(resolveAccountSection(group)).toBeNull();
    expect(resolveAccountSection('tokens')).toBe('/panel/settings/account/security?highlight=access-token');
    expect(resolveAccountSection('nope')).toBe('/panel/settings/account/profile');
  });

  it('organization groups and the members page stay, budget moved into 常规', () => {
    expect(resolveOrgSection(3, 'general')).toBeNull();
    expect(resolveOrgSection(3, 'danger')).toBeNull();
    expect(resolveOrgSection(3, 'members')).toBeNull();
    expect(resolveOrgSection(3, 'budget')).toBe('/panel/settings/org/3/general?highlight=team-budget');
    expect(resolveOrgSection(3, 'nope')).toBe('/panel/settings/org/3/general');
  });

  it('the old organization 自动路由 page redirects to the workbench 路由策略 page', () => {
    expect(resolveOrgSection(3, 'routing')).toBe('/panel/routing');
  });
});
