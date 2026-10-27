// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import PaymentSheet from './PaymentSheet';

// 支付编辑抽屉:后端只下发脱敏后的密钥,表单把回显放在空字段旁边,留空提交表示沿用库里的值。

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn(), put: vi.fn() } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() } }));

const STORED = {
  id: 7,
  type: 'stripe',
  uuid: 'u7',
  name: 'Stripe',
  icon: 'https://example.com/i.png',
  notify_domain: '',
  fixed_fee: 0,
  percent_fee: 0,
  currency: 'USD',
  currency_rate: 1,
  sort: 1,
  enable: true,
  config: JSON.stringify({ secret_key: 'sk_l****mnop', webhook_secret: 'whse****wxyz' })
};

beforeAll(() => i18n.changeLanguage('en_US'));
afterAll(() => i18n.changeLanguage('zh_CN'));
beforeEach(() => {
  vi.clearAllMocks();
  API.get.mockResolvedValue({ data: { success: true, data: { ...STORED } } });
  API.put.mockResolvedValue({ data: { success: true, message: '' } });
});
afterEach(cleanup);

describe('PaymentSheet secrets', () => {
  it('shows the masked secrets beside empty fields when editing', async () => {
    render(<PaymentSheet open paymentId={7} onClose={() => {}} onSaved={() => {}} />);
    await screen.findByText('Configured sk_l****mnop');
    expect(screen.getByText('Configured whse****wxyz')).toBeTruthy();
    expect(document.getElementById('p-config-secret_key').value).toBe('');
    expect(document.getElementById('p-config-webhook_secret').value).toBe('');
    expect(screen.getAllByText('Leave blank to keep the existing value')).toHaveLength(2);
  });

  it('submits blank secrets so the backend keeps the stored values', async () => {
    render(<PaymentSheet open paymentId={7} onClose={() => {}} onSaved={() => {}} />);
    await screen.findByText('Configured sk_l****mnop');
    fireEvent.change(document.getElementById('p-config-webhook_secret'), { target: { value: 'whsec_new' } });
    fireEvent.submit(document.querySelector('form'));
    await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
    const [, body] = API.put.mock.calls[0];
    expect(JSON.parse(body.config)).toEqual({ secret_key: '', webhook_secret: 'whsec_new' });
  });

  it('does not show a mask line for a new payment', async () => {
    render(<PaymentSheet open paymentId={null} onClose={() => {}} onSaved={() => {}} />);
    expect(screen.queryByText(/^Configured /)).toBeNull();
    expect(screen.queryByText('Leave blank to keep the existing value')).toBeNull();
  });
});
