import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { exportLogs } from './logExport';

function jsonResponse(body) {
  return new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json; charset=utf-8' } });
}

describe('exportLogs', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('en_US');
    vi.stubGlobal('localStorage', { getItem: () => null });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('shows a refused export instead of saving the JSON body as a CSV file', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse({
          success: false,
          message: 'Exports cover at most 31 days. Choose a shorter time range',
          code: 'log_export_range_too_long',
          params: { max_days: 31 }
        })
      )
    );
    const createObjectURL = vi.fn();
    vi.stubGlobal('URL', { ...URL, createObjectURL, revokeObjectURL: vi.fn() });

    await expect(exportLogs({ params: { start_timestamp: 1, end_timestamp: 2 }, userIsAdmin: false })).rejects.toThrow(
      'Exports cover at most 31 days. Choose a shorter time range.'
    );
    expect(createObjectURL).not.toHaveBeenCalled();
  });

  it('keeps the server message for a refusal without a known code', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ success: false, message: 'record not found' }))
    );
    await expect(exportLogs({ params: {}, userIsAdmin: true })).rejects.toThrow('record not found');
  });
});
