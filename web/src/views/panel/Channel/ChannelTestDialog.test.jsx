// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
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
      model === 'broken-model' ? { success: false, message: 'model does not exist' } : { success: true, time: 0.42 }
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
    expect(within(rowOf('dall-e-3')).getByText('Not tested')).toBeTruthy();
  });

  it('select all includes costly models', () => {
    render(<ChannelTestDialog open item={item} onClose={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: 'Select all' }));
    expect(screen.getByText('3 selected')).toBeTruthy();
  });
});
