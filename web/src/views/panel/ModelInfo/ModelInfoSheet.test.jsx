// @vitest-environment jsdom
import { createElement } from 'react';
import { MemoryRouter } from 'react-router';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import { toast } from '@/components/ui/sonner';
import ModelInfoSheet from './ModelInfoSheet';

// 模型抽屉:一处编辑目录信息与价格。保存时分别调用目录与价格接口,只写有改动的部分,
// 任一部分失败都要明确提示是哪一部分没保存。

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn(), put: vi.fn() } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() } }));

const ok = { data: { success: true, message: '' } };
const fail = (message) => ({ data: { success: false, message } });

const CATALOG = {
  id: 5,
  model: 'gpt-a',
  name: 'GPT A',
  context_length: 128000,
  max_tokens: 4096,
  input_modalities: '["text"]',
  output_modalities: '["text"]',
  tags: '[]',
  capabilities: '',
  endpoints: '',
  vendor_id: 0,
  hidden: false,
  source: 'models.dev'
};
const PRICE = { model: 'gpt-a', type: 'tokens', channel_type: 1, input: 1, output: 2, locked: false };
const ROW = { ...CATALOG, key: 'gpt-a', catalogId: 5, price: PRICE, bound_channels: [{ id: 9, name: 'openai-main', type: 1 }] };

beforeEach(() => {
  vi.clearAllMocks();
  API.get.mockImplementation((url) => {
    if (url === '/api/model_info/5') return Promise.resolve({ data: { success: true, data: { ...CATALOG } } });
    return Promise.resolve({ data: { success: true, data: [] } });
  });
  API.post.mockResolvedValue(ok);
  API.put.mockResolvedValue(ok);
});

afterEach(cleanup);

const open = (props = {}) => {
  const onSaved = vi.fn();
  const onOpenChange = vi.fn();
  render(
    createElement(
      MemoryRouter,
      null,
      createElement(ModelInfoSheet, {
        open: true,
        onOpenChange,
        onSaved,
        prices: [PRICE],
        ownedby: [{ value: 1, label: 'OpenAI' }],
        ...props
      })
    )
  );
  return { onSaved, onOpenChange };
};

const numberInputs = () => document.querySelectorAll('input[type="number"]');
// 数字输入依次为:上下文长度、最大输出、价格输入、价格输出。
const setPriceInput = (value) => fireEvent.change(numberInputs()[2], { target: { value } });
const setPriceOutput = (value) => fireEvent.change(numberInputs()[3], { target: { value } });
const clickChatEndpoint = () =>
  fireEvent.click(
    screen
      .getAllByText(i18n.t('modelInfoPage.endpointOptions.chat'))
      .map((el) => el.closest('button'))
      .find(Boolean)
  );
const submit = () => fireEvent.click(screen.getByRole('button', { name: i18n.t('common.submit') }));
const priceCalls = () => API.post.mock.calls.filter(([url]) => url === '/api/prices/multiple');

describe('ModelInfoSheet — 新建', () => {
  it('只填目录时只写目录,价格节留空不建价格', async () => {
    const { onSaved } = open();
    fireEvent.change(document.getElementById('modelinfo-model'), { target: { value: 'new-model' } });
    submit();
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(API.post).toHaveBeenCalledWith('/api/model_info/', expect.objectContaining({ model: 'new-model', name: 'new-model' }));
    expect(priceCalls()).toHaveLength(0);
  });

  it('同时填价格时目录与价格各调一次,无价格模型 original_models 为 []', async () => {
    const { onSaved } = open();
    fireEvent.change(document.getElementById('modelinfo-model'), { target: { value: 'new-model' } });
    setPriceInput('3');
    setPriceOutput('6');
    submit();
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(priceCalls()).toHaveLength(1);
    expect(priceCalls()[0][1]).toMatchObject({ original_models: [], models: ['new-model'] });
    expect(toast.success).toHaveBeenCalled();
  });

  it('目录无行的模型按新建处理,以模型名预填', async () => {
    const orphan = { key: 'orphan-x', catalogId: 0, id: 0, model: 'orphan-x', price: null, bound_channels: [] };
    open({ row: orphan });
    expect(screen.getByText(i18n.t('modelInfoPage.createTitle'))).toBeTruthy();
    expect(document.getElementById('modelinfo-model').value).toBe('orphan-x');
    expect(screen.getByText(i18n.t('modelInfoPage.priceEmptyTip'))).toBeTruthy();
  });
});

describe('ModelInfoSheet — 编辑', () => {
  it('改接口能力与价格,一次保存分别更新目录与价格(有价格传原模型名)', async () => {
    const { onSaved } = open({ row: ROW });
    await waitFor(() => expect(document.getElementById('modelinfo-name').value).toBe('GPT A'));
    expect(screen.getByText(i18n.t('modelInfoPage.editTitle'))).toBeTruthy();
    expect(screen.getByText(/openai-main/)).toBeTruthy();

    clickChatEndpoint();
    setPriceInput('5');
    submit();

    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(API.put).toHaveBeenCalledWith('/api/model_info/', expect.objectContaining({ id: 5, endpoints: '["chat"]', locked: true }));
    expect(priceCalls()[0][1]).toMatchObject({
      original_models: ['gpt-a'],
      models: ['gpt-a'],
      price: expect.objectContaining({ input: 5 })
    });
  });

  it('只改价格时不写目录', async () => {
    const { onSaved } = open({ row: ROW });
    await waitFor(() => expect(document.getElementById('modelinfo-name').value).toBe('GPT A'));
    setPriceInput('5');
    submit();
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(API.put).not.toHaveBeenCalled();
    expect(priceCalls()).toHaveLength(1);
  });

  it('什么都没改时直接关闭,不发请求', async () => {
    const { onOpenChange } = open({ row: ROW });
    await waitFor(() => expect(document.getElementById('modelinfo-name').value).toBe('GPT A'));
    submit();
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(API.put).not.toHaveBeenCalled();
    expect(priceCalls()).toHaveLength(0);
  });
});

describe('ModelInfoSheet — 部分保存失败', () => {
  it('价格失败:提示价格未保存,目录已保存仍刷新', async () => {
    API.post.mockImplementation((url) => Promise.resolve(url === '/api/prices/multiple' ? fail('boom') : ok));
    const { onSaved } = open({ row: ROW });
    await waitFor(() => expect(document.getElementById('modelinfo-name').value).toBe('GPT A'));
    clickChatEndpoint();
    setPriceInput('5');
    submit();
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(i18n.t('modelInfoPage.priceNotSaved', { message: 'boom' })));
    expect(onSaved).toHaveBeenCalled();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it('目录与价格都失败:分别提示,抽屉保持打开', async () => {
    API.put.mockRejectedValue(new Error('network down'));
    API.post.mockResolvedValue(fail('boom'));
    const { onSaved } = open({ row: ROW });
    await waitFor(() => expect(document.getElementById('modelinfo-name').value).toBe('GPT A'));
    clickChatEndpoint();
    setPriceInput('5');
    submit();
    await waitFor(() => expect(toast.error).toHaveBeenCalledTimes(2));
    expect(toast.error).toHaveBeenCalledWith(i18n.t('modelInfoPage.catalogNotSaved', { message: 'network down' }));
    expect(toast.error).toHaveBeenCalledWith(i18n.t('modelInfoPage.priceNotSaved', { message: 'boom' }));
    expect(onSaved).not.toHaveBeenCalled();
  });
});

describe('ModelInfoSheet — 绑定渠道', () => {
  const withChannels = (bound_channels) => ({ ...ROW, bound_channels });
  const badges = () => screen.queryAllByTestId('channel-disabled-badge');
  const noChannel = () => screen.queryByText(i18n.t('modelsPage.sheet.noChannelHint'));

  it('渠道全部禁用:显示无渠道警告,且每个禁用渠道带状态徽标', async () => {
    open({
      row: withChannels([
        { id: 9, name: 'openai-main', type: 1, status: 2 },
        { id: 10, name: 'openai-backup', type: 1, status: 3 }
      ])
    });
    await waitFor(() => expect(document.getElementById('modelinfo-name').value).toBe('GPT A'));
    expect(noChannel()).toBeTruthy();
    expect(screen.getByText(/openai-main/)).toBeTruthy();
    expect(badges().map((b) => b.textContent)).toEqual([i18n.t('channel_row.manual'), i18n.t('channel_row.auto')]);
  });

  it('启用与禁用混合:不显示警告,只有禁用渠道带徽标,每行链到渠道编辑', async () => {
    open({
      row: withChannels([
        { id: 9, name: 'openai-main', type: 1, status: 1 },
        { id: 10, name: 'openai-off', type: 1, status: 2 }
      ])
    });
    await waitFor(() => expect(document.getElementById('modelinfo-name').value).toBe('GPT A'));
    expect(noChannel()).toBeNull();
    expect(badges()).toHaveLength(1);
    expect(badges()[0].closest('tr').textContent).toContain('openai-off');
    const link = screen.getByRole('link', { name: i18n.t('modelsPage.sheet.openChannel', { name: 'openai-main' }), hidden: true });
    expect(link.getAttribute('href')).toBe('/panel/channel?name=openai-main&channel_id=9');
  });

  it('initialTab=channels 时默认打开渠道与分组 Tab', async () => {
    open({ row: ROW, initialTab: 'channels' });
    await waitFor(() => expect(document.getElementById('modelinfo-name').value).toBe('GPT A'));
    expect(screen.getByRole('tab', { selected: true }).textContent).toContain(i18n.t('modelsPage.channelsTab'));
    expect(screen.getByText(i18n.t('modelsPage.sheet.channelsReadonly'))).toBeTruthy();
  });

  it('没有状态的后端摘要渠道视为启用', async () => {
    open({ row: ROW });
    await waitFor(() => expect(document.getElementById('modelinfo-name').value).toBe('GPT A'));
    expect(noChannel()).toBeNull();
    expect(badges()).toHaveLength(0);
  });
});
