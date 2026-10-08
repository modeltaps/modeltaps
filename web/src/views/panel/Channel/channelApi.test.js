import { beforeEach, describe, expect, it, vi } from 'vitest';

const get = vi.fn();
vi.mock('utils/api', () => ({ API: { get: (...args) => get(...args) } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { fetchChannelStatusCounts, fetchTagChannels, statusCountParams } from './channelApi';

describe('statusCountParams', () => {
  it('keeps the other filters and swaps in the chip status', () => {
    const keyword = { name: 'open', group: 'vip', type: 1, status: '2' };
    expect(statusCountParams(keyword, '3')).toEqual({ page: 1, size: 1, name: 'open', group: 'vip', type: 1, status: '3' });
    expect(statusCountParams(keyword, null)).toEqual({ page: 1, size: 1, name: 'open', group: 'vip', type: 1 });
  });

  it('normalizes the "all" sentinels like the list request', () => {
    expect(statusCountParams({ group: 'all', tag: 'all' }, '1')).toEqual({ page: 1, size: 1, group: '', tag: '', status: '1' });
  });
});

describe('fetchChannelStatusCounts', () => {
  beforeEach(() => get.mockReset());

  it('counts each chip with the current filters so counts match the filtered list', async () => {
    const totals = { undefined: 7, 1: 4, 2: 2, 3: 1 };
    get.mockImplementation((_url, opts) =>
      Promise.resolve({ data: { success: true, data: { data: [], total_count: totals[opts?.params?.status] } } })
    );
    await expect(fetchChannelStatusCounts({ name: 'open', status: '1' })).resolves.toEqual({ all: 7, enabled: 4, manual: 2, auto: 1 });
    expect(get).toHaveBeenCalledTimes(4);
    for (const [, { params }] of get.mock.calls) expect(params.name).toBe('open');
  });

  it('reports null for a failed count', async () => {
    get.mockResolvedValue({ data: { success: false, message: 'x' } });
    await expect(fetchChannelStatusCounts()).resolves.toEqual({ all: null, enabled: null, manual: null, auto: null });
  });
});

describe('fetchTagChannels', () => {
  beforeEach(() => get.mockReset());

  it('unwraps the paginated list response to the channel array', async () => {
    const channels = [{ id: 1, name: 'a', key_status: { configured: true, count: 1, masked: ['····1111'] } }];
    get.mockResolvedValue({ data: { success: true, data: { data: channels, page: 1, size: 10, total_count: 1 } } });
    await expect(fetchTagChannels('grp/x')).resolves.toEqual(channels);
    expect(get).toHaveBeenCalledWith('/api/channel_tag/grp%2Fx/list');
  });

  it('accepts a bare array and falls back to [] for empty payloads', async () => {
    get.mockResolvedValueOnce({ data: { success: true, data: [{ id: 2 }] } });
    await expect(fetchTagChannels('t')).resolves.toEqual([{ id: 2 }]);
    get.mockResolvedValueOnce({ data: { success: true, data: { data: null, total_count: 0 } } });
    await expect(fetchTagChannels('t')).resolves.toEqual([]);
    get.mockResolvedValueOnce({ data: { success: true, data: null } });
    await expect(fetchTagChannels('t')).resolves.toEqual([]);
  });

  it('throws the server message on failure', async () => {
    get.mockResolvedValue({ data: { success: false, message: 'boom' } });
    await expect(fetchTagChannels('t')).rejects.toThrow('boom');
  });
});
