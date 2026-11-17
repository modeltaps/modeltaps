import { describe, expect, it } from 'vitest';

import {
  admin,
  agentDevice,
  invitations,
  inviteLink,
  isAdminPath,
  isOrgPath,
  isSettingsPath,
  org,
  orgSlugOf,
  rescope,
  settings,
  unscope,
  workbench
} from './paths';

describe('path builders', () => {
  it('workbench pages, personal and in an organization', () => {
    expect(workbench.dashboard()).toBe('/panel/dashboard');
    expect(workbench.dashboard('acme')).toBe('/panel/orgs/acme/dashboard');
    expect(workbench.logs()).toBe('/panel/logs');
    expect(workbench.logs('tasks', 'acme')).toBe('/panel/orgs/acme/logs/tasks');
    expect(workbench.keys()).toBe('/panel/keys');
    expect(workbench.models()).toBe('/panel/models');
    expect(workbench.routingPolicies('acme')).toBe('/panel/orgs/acme/routing-policies');
    expect(workbench.playground()).toBe('/panel/playground/chat');
    expect(workbench.docs()).toBe('/panel/docs');
    expect(workbench.docs('image')).toBe('/panel/docs/image');
    expect(workbench.billing('invoices')).toBe('/panel/billing/invoices');
    expect(org.overview('acme')).toBe('/panel/orgs/acme/overview');
    expect(invitations()).toBe('/panel/invitations');
    expect(inviteLink('a b')).toBe('/panel/invitations?invite=a%20b');
    expect(agentDevice()).toBe('/panel/device');
  });

  it('settings hub and platform admin', () => {
    expect(settings.account('security')).toBe('/panel/settings/security');
    expect(settings.org('acme', 'members')).toBe('/panel/settings/orgs/acme/members');
    expect(admin.root()).toBe('/panel/admin');
    expect(admin.statsByUser()).toBe('/panel/admin/stats/users');
    expect(admin.providers()).toBe('/panel/admin/providers');
    expect(admin.providers(12)).toBe('/panel/admin/providers/12');
    expect(admin.providers('new')).toBe('/panel/admin/providers/new');
    expect(admin.models('vendors')).toBe('/panel/admin/models/vendors');
    expect(admin.settings('notify')).toBe('/panel/admin/settings/notify');
  });

  it('scopes', () => {
    expect(orgSlugOf('/panel/orgs/acme/usage')).toBe('acme');
    expect(orgSlugOf('/panel/usage')).toBeNull();
    expect(unscope('/panel/orgs/acme/logs/tasks')).toBe('/panel/logs/tasks');
    expect(rescope('/panel/logs/tasks', 'beta')).toBe('/panel/orgs/beta/logs/tasks');
    expect(rescope('/panel/orgs/acme/usage', 'beta')).toBe('/panel/orgs/beta/usage');
    expect(rescope('/panel/orgs/acme/usage', null)).toBe('/panel/usage');
    expect(isOrgPath('/panel/orgs/acme')).toBe(true);
    expect(isAdminPath('/panel/admin/users')).toBe(true);
    expect(isAdminPath('/panel/administrator')).toBe(false);
    expect(isSettingsPath('/panel/settings/orgs/acme/general')).toBe(true);
  });
});
