// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';

vi.mock('utils/api', () => ({ API: { get: vi.fn() } }));

import { PricingRiskPreview, ratioText } from './PricingCard';

const risk = {
  group_ratio: 1.12,
  payment_cost: 4,
  net_factor: 1.0752,
  channels: [
    {
      channel_id: 1,
      name: 'Gateway',
      type: 1,
      fee_rate: 0.05,
      fee_breakeven: 1.09375,
      fee_margin: 2.4,
      worst_breakeven: 1.2,
      supplies: 3,
      serving_above: 1,
      backup_above: 0,
      cost_unknown: 0,
      pending: 0
    }
  ],
  serving_above: 1,
  backup_above: 0,
  cost_unknown: 0,
  models_at_loss: [],
  worst: [{ model: 'gpt-x', channel_name: 'Gateway', traffic: 'primary', cost_to_catalog: 1.15, breakeven: 1.2 }],
  groups: [
    { symbol: 'default', name: 'Default', ratio: 1.12, net_factor: 1.0752, losing_channels: 0 },
    { symbol: 'vip', name: 'VIP', ratio: 1, net_factor: 0.96, losing_channels: 1 }
  ],
  supply_routing: true
};

describe('PricingRiskPreview', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('en_US');
    API.get.mockReset();
  });
  afterEach(cleanup);

  it('writes group ratios with at most three decimals', () => {
    expect(ratioText(1.09375)).toBe('×1.094');
    expect(ratioText(1.12)).toBe('×1.12');
    expect(ratioText(1)).toBe('×1');
  });

  it('previews the saved ratio, shows break-evens as ratios and lists the groups', async () => {
    API.get.mockResolvedValue({ data: { success: true, data: risk } });
    const onSavedRatio = vi.fn();
    render(<PricingRiskPreview ratio={null} payment={4} onSavedRatio={onSavedRatio} />);
    const preview = await screen.findByTestId('pricing-risk', {}, { timeout: 2000 });
    expect(API.get).toHaveBeenCalledWith('/api/model_supply/pricing_risk', { params: { payment: 4 } });
    expect(preview.textContent).toContain('×1.094');
    expect(preview.textContent).toContain('needs ×1.2');
    const groups = screen.getByTestId('pricing-risk-groups');
    expect(groups.textContent).toContain('VIP');
    expect(groups.textContent).toContain('$0.9600');
    await waitFor(() => expect(onSavedRatio).toHaveBeenCalledWith(1.12));
  });

  it('sends the ratio being tried', async () => {
    API.get.mockResolvedValue({ data: { success: true, data: { ...risk, group_ratio: 1.2 } } });
    render(<PricingRiskPreview ratio={1.2} payment={4} />);
    await screen.findByTestId('pricing-risk', {}, { timeout: 2000 });
    expect(API.get).toHaveBeenCalledWith('/api/model_supply/pricing_risk', { params: { ratio: 1.2, payment: 4 } });
  });
});
