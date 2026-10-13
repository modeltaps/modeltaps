// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';

const testChannelModel = vi.fn();
vi.mock('./channelApi', () => ({ testChannelModel: (...args) => testChannelModel(...args) }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

import ChannelTestDialog from './ChannelTestDialog';

const item = { id: 7, name: 'fake', models: 'gpt-4o-mini,dall-e-3,broken-model', test_model: 'gpt-4o-mini' };

const rowOf = (model) => screen.getByText(model).closest('tr');

describe('ChannelTestDialog', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('en_US');
    testChannelModel.mockReset();
    testChannelModel.mockImplementation(async (_id, model) =>
      model === 'broken-model'
        ? {
            success: false,
            message: 'speed test failed, reason: model does not exist',
            code: 'channel_test_failed',
            params: { reason: 'model does not exist' }
          }
        : { success: true, time: 0.42 }
    );
  });
  afterEach(cleanup);

  it('leaves costly models unselected and tests only the selected ones', async () => {
    const onFinished = vi.fn();
    render(<ChannelTestDialog open item={item} onClose={() => {}} onFinished={onFinished} />);

    expect(within(rowOf('gpt-4o-mini')).getByRole('checkbox').getAttribute('aria-checked')).toBe('true');
    expect(within(rowOf('dall-e-3')).getByRole('checkbox').getAttribute('aria-checked')).toBe('false');
    expect(screen.getByText('2 selected')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: /Start test/ }));
    await waitFor(() => expect(onFinished).toHaveBeenCalledTimes(1));

    expect(testChannelModel.mock.calls.map((c) => c[1]).sort()).toEqual(['broken-model', 'gpt-4o-mini']);
    expect(testChannelModel).toHaveBeenCalledWith(7, 'gpt-4o-mini', '');
    expect(within(rowOf('gpt-4o-mini')).getByText('Passed')).toBeTruthy();
    expect(within(rowOf('gpt-4o-mini')).getByText('0.42s')).toBeTruthy();
    expect(within(rowOf('broken-model')).getByText('model does not exist')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Endpoint' }).textContent).toContain('Endpoint: Auto detect');
    expect(within(rowOf('dall-e-3')).getByText('Not tested')).toBeTruthy();
  });

  it('select all includes costly models', () => {
    render(<ChannelTestDialog open item={item} onClose={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: 'Select all' }));
    expect(screen.getByText('3 selected')).toBeTruthy();
  });

  it('retries only the failed models and keeps the passed rows', async () => {
    const onFinished = vi.fn();
    render(<ChannelTestDialog open item={item} onClose={() => {}} onFinished={onFinished} />);
    fireEvent.click(screen.getByRole('button', { name: /Start test/ }));
    const retry = await screen.findByRole('button', { name: 'Retry failed (1)' });

    testChannelModel.mockClear();
    testChannelModel.mockResolvedValue({ success: true, time: 1.5 });
    fireEvent.click(retry);

    await waitFor(() => expect(within(rowOf('broken-model')).getByText('1.50s')).toBeTruthy());
    expect(testChannelModel.mock.calls.map((c) => c[1])).toEqual(['broken-model']);
    expect(within(rowOf('gpt-4o-mini')).getByText('0.42s')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /Retry failed/ })).toBeNull();
    expect(screen.getByText(/2 passed, 0 failed/)).toBeTruthy();
  });

  it('tests a single row and blocks a batch while it runs', async () => {
    let resolve;
    testChannelModel.mockImplementation(() => new Promise((r) => (resolve = r)));
    render(<ChannelTestDialog open item={item} onClose={() => {}} />);

    fireEvent.click(within(rowOf('dall-e-3')).getByRole('button', { name: 'Test dall-e-3' }));
    expect(testChannelModel).toHaveBeenCalledWith(7, 'dall-e-3', '');
    expect(within(rowOf('dall-e-3')).getByText('Testing')).toBeTruthy();
    expect(screen.getByRole('button', { name: /Start test/ }).disabled).toBe(true);
    expect(screen.getByRole('button', { name: 'Endpoint' }).disabled).toBe(true);

    await act(async () => resolve({ success: false, message: 'quota exceeded' }));
    expect(within(rowOf('dall-e-3')).getByText('quota exceeded')).toBeTruthy();
    expect(within(rowOf('dall-e-3')).getByRole('button', { name: 'Retry dall-e-3' })).toBeTruthy();
    expect(screen.getByRole('button', { name: /Start test/ }).disabled).toBe(false);
    expect(screen.getByRole('button', { name: 'Retry failed (1)' })).toBeTruthy();
  });
});
