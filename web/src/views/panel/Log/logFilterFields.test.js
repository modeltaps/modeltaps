import { describe, expect, it } from 'vitest';

import { buildLogFilterFields, LOG_FILTER_PARAM_KEYS } from './logFilterFields';

const keys = (opts) => buildLogFilterFields(opts).map((f) => f.key);

// The request IP of a user's own logs is visible to them, so the personal log page can filter by
// it; site administrators keep it with the other admin dimensions, and organization logs never
// show it.
describe('log filter fields by context', () => {
  it('lets a personal context filter by source IP, without the admin dimensions', () => {
    const personal = keys({ effectiveAdmin: false, isOrgContext: false });
    expect(personal).toContain('source_ip');
    expect(personal).toContain('request_id');
    expect(personal).not.toContain('channel_id');
    expect(personal).not.toContain('username');
    expect(personal).not.toContain('upstream_request_id');
  });

  it('keeps every admin dimension for site administrators, with source IP only once', () => {
    const admin = keys({ effectiveAdmin: true, isOrgContext: false });
    expect(admin).toEqual(expect.arrayContaining(['channel_id', 'username', 'source_ip', 'upstream_request_id']));
    expect(admin.filter((k) => k === 'source_ip')).toHaveLength(1);
  });

  it('does not offer source IP in an organization context', () => {
    const org = keys({ effectiveAdmin: false, isOrgContext: true, orgMembers: [] });
    expect(org).toContain('member_id');
    expect(org).not.toContain('source_ip');
  });

  it('serializes source IP with the multi-value parameters of the backend', () => {
    const field = buildLogFilterFields({ effectiveAdmin: false, isOrgContext: false }).find((f) => f.key === 'source_ip');
    expect(field.paramInclude).toBe('source_ips');
    expect(field.paramExclude).toBe('exclude_source_ip');
    expect(LOG_FILTER_PARAM_KEYS).toEqual(expect.arrayContaining(['source_ips', 'exclude_source_ip']));
  });
});
