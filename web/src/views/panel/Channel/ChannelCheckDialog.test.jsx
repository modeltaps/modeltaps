// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';

const checkChannelModel = vi.fn();
const fetchProviderModels = vi.fn();
vi.mock('./channelApi', () => ({
  checkChannelModel: (...args) => checkChannelModel(...args),
  fetchProviderModels: (...args) => fetchProviderModels(...args)
}));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

import ChannelCheckDialog from './ChannelCheckDialog';

const item = { id: 9, name: 'fake', models: 'gpt-4o-mini,claude-x,broken,o3-deep-research', test_model: '' };

const probe = (key, status, results, extra = {}) => ({ key, status, latency_ms: 1200, results, ...extra });

// A model that passes the base check but cannot read images and gets rate limited on tools.
const warningResult = (model) => ({
  model,
  status: 2,
  items: [
    probe('base', 1, [
      { name: 'challenge', status: 1, code: 'challenge_passed', params: { expected: '42' } },
      { name: 'latency', status: 1, code: 'latency_ok', params: { seconds: 1.2 } }
    ]),
    probe('image', 0, [{ name: 'image', status: 0, code: 'image_mismatch', params: { expected: '123456', answer: '1 < 2' } }]),
    probe('json', 3, [{ name: 'json_schema', status: 3, code: 'json_skipped_claude' }]),
    probe('tool', 0, [{ name: 'response', status: 0, code: 'upstream_error', params: { message: 'too many requests' } }], {
      category: 'rate_limit'
    }),
    probe('relay', 2, [{ name: 'request_id', status: 2, code: 'request_id_found', params: { count: 2 } }])
  ]
});

const failedResult = (model) => ({
  model,
  status: 0,
  items: [probe('base', 0, [{ name: 'response', status: 0, code: 'upstream_error', params: { message: 'model does not exist' } }])]
});

const rowOf = (model) => screen.getByText(model).closest('tr');

function deferred() {
  let resolve;
  const promise = new Promise((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

async function startAndConfirm() {
  fireEvent.click(screen.getByRole('button', { name: /Start check/ }));
  const confirm = await screen.findByRole('alertdialog');
  fireEvent.click(within(confirm).getByRole('button', { name: 'Execute' }));
}

describe('ChannelCheckDialog', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('en_US');
    checkChannelModel.mockReset();
    fetchProviderModels.mockReset();
    checkChannelModel.mockImplementation(async (_id, model) => (model === 'broken' ? failedResult(model) : warningResult(model)));
  });
  afterEach(cleanup);

  it('warns about real upstream calls and asks before starting', async () => {
    render(<ChannelCheckDialog open item={item} onClose={() => {}} />);

    expect(screen.getByRole('alert').textContent).toContain('up to 5 real requests');
    // Deep research models cost real money per request and stay unselected.
    expect(within(rowOf('o3-deep-research')).getByRole('checkbox').getAttribute('aria-checked')).toBe('false');
    expect(screen.getByText('3 selected')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: /Start check/ }));
    const confirm = await screen.findByRole('alertdialog');
    expect(confirm.textContent).toContain('Checking 3 model(s) sends up to 15 requests');
    fireEvent.click(within(confirm).getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByRole('alertdialog')).toBeNull();
    expect(checkChannelModel).not.toHaveBeenCalled();
  });

  it('checks each selected model once, two at a time, and shows results per check', async () => {
    let inFlight = 0;
    let maxInFlight = 0;
    checkChannelModel.mockImplementation(async (_id, model) => {
      inFlight += 1;
      maxInFlight = Math.max(maxInFlight, inFlight);
      await new Promise((r) => setTimeout(r, 5));
      inFlight -= 1;
      return model === 'broken' ? failedResult(model) : warningResult(model);
    });
    render(<ChannelCheckDialog open item={item} onClose={() => {}} />);
    await startAndConfirm();

    await waitFor(() => expect(within(rowOf('broken')).getByText('Failed')).toBeTruthy());
    await waitFor(() => expect(within(rowOf('claude-x')).getByText('Warning')).toBeTruthy());
    expect(checkChannelModel.mock.calls.map((c) => c[1]).sort()).toEqual(['broken', 'claude-x', 'gpt-4o-mini']);
    expect(checkChannelModel).toHaveBeenCalledWith(9, 'gpt-4o-mini');
    expect(maxInFlight).toBe(2);

    const row = rowOf('gpt-4o-mini');
    expect(within(row).getByRole('img', { name: 'Basic: Passed' })).toBeTruthy();
    expect(within(row).getByRole('img', { name: 'Vision: Failed' })).toBeTruthy();
    expect(within(row).getByRole('img', { name: 'JSON: N/A' })).toBeTruthy();
    expect(within(row).getByRole('img', { name: 'Relay: Warning' }).getAttribute('title')).toContain('2 request ID(s) in the error');
    expect(within(rowOf('o3-deep-research')).getByText('Not checked')).toBeTruthy();

    fireEvent.click(within(row).getByRole('button', { name: 'Details of gpt-4o-mini' }));
    expect(screen.getByText('Expected 42, answered correctly')).toBeTruthy();
    // Upstream text is shown as it is, not HTML-escaped.
    expect(screen.getByText(/The image shows 123456, got: 1 < 2/)).toBeTruthy();
    expect(screen.getByText('Rate limited or overloaded')).toBeTruthy();
    expect(screen.getByText('too many requests')).toBeTruthy();
  });

  it('retries only the failed models and keeps the other rows', async () => {
    render(<ChannelCheckDialog open item={item} onClose={() => {}} />);
    await startAndConfirm();
    const retry = await screen.findByRole('button', { name: 'Retry failed (1)' });

    checkChannelModel.mockClear();
    checkChannelModel.mockImplementation(async (_id, model) => warningResult(model));
    fireEvent.click(retry);

    await waitFor(() => expect(within(rowOf('broken')).getByText('Warning')).toBeTruthy());
    expect(checkChannelModel.mock.calls.map((c) => c[1])).toEqual(['broken']);
    expect(within(rowOf('gpt-4o-mini')).getByText('Warning')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /Retry failed/ })).toBeNull();
  });

  it('checks a single row and blocks a batch while it runs', async () => {
    const pending = deferred();
    checkChannelModel.mockImplementation(() => pending.promise);
    render(<ChannelCheckDialog open item={item} onClose={() => {}} />);

    fireEvent.click(within(rowOf('claude-x')).getByRole('button', { name: 'Check claude-x' }));
    expect(checkChannelModel).toHaveBeenCalledWith(9, 'claude-x');
    expect(within(rowOf('claude-x')).getByText('Checking')).toBeTruthy();
    expect(screen.getByRole('button', { name: /Start check/ }).disabled).toBe(true);
    expect(within(rowOf('claude-x')).getByRole('button', { name: 'Check claude-x' }).disabled).toBe(true);

    await act(async () => pending.resolve(failedResult('claude-x')));
    expect(within(rowOf('claude-x')).getByText('Failed')).toBeTruthy();
    expect(within(rowOf('claude-x')).getByRole('button', { name: 'Retry claude-x' })).toBeTruthy();
    expect(screen.getByRole('button', { name: /Start check/ }).disabled).toBe(false);
  });

  it('shows the request error and falls back to the code for unknown verdicts', async () => {
    checkChannelModel.mockImplementation(async (_id, model) => {
      if (model === 'broken') throw new Error('record not found');
      return { model, status: 1, items: [probe('base', 1, [{ name: 'future_check', status: 1, code: 'future_code', params: { n: 1 } }])] };
    });
    render(<ChannelCheckDialog open item={item} onClose={() => {}} />);
    await startAndConfirm();

    await waitFor(() => expect(within(rowOf('broken')).getByText('Failed')).toBeTruthy());
    fireEvent.click(within(rowOf('broken')).getByRole('button', { name: 'Details of broken' }));
    expect(screen.getByText('record not found')).toBeTruthy();

    await waitFor(() => expect(within(rowOf('claude-x')).getByText('Passed')).toBeTruthy());
    fireEvent.click(within(rowOf('claude-x')).getByRole('button', { name: 'Details of claude-x' }));
    expect(screen.getByText('future_check')).toBeTruthy();
    expect(screen.getByText('future_code {"n":1}')).toBeTruthy();
  });

  it('replaces the list with the upstream models', async () => {
    fetchProviderModels.mockResolvedValue(['up-a', 'up-b']);
    render(<ChannelCheckDialog open item={item} onClose={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: /Fetch upstream models/ }));
    await waitFor(() => expect(screen.getByText('up-a')).toBeTruthy());
    expect(screen.queryByText('gpt-4o-mini')).toBeNull();
    expect(screen.getByText('2 selected')).toBeTruthy();
  });

  it('renders the dialog in Japanese', async () => {
    await i18n.changeLanguage('ja_JP');
    render(<ChannelCheckDialog open item={item} onClose={() => {}} />);
    expect(screen.getByText('チャネル検査')).toBeTruthy();
    expect(screen.getByRole('button', { name: /検査を開始/ })).toBeTruthy();
  });
});
