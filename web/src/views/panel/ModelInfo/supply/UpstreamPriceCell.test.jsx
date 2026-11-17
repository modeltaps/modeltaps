// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import i18n from 'i18n/i18n';
import { UpstreamName, UpstreamPriceCell } from './supplyParts';

const supply = (fields) => ({ model: 'm', upstream_priced: true, sell_input_price: 3, sell_output_price: 15, ...fields });
const show = (fields) => render(createElement(UpstreamPriceCell, { supply: supply(fields) }));

describe('UpstreamPriceCell', () => {
  beforeEach(() => i18n.changeLanguage('en'));
  afterEach(cleanup);

  it('shows the token price without a mark when the model sells at or above it', () => {
    show({ upstream_price: { model: 'v/m', input: 3, output: 15 } });
    expect(screen.getByText(/\$3/).textContent).toBe('$3 / $15');
    expect(screen.queryByTestId('supply-price-loss')).toBeNull();
  });

  it('marks an upstream price above a free sell price', () => {
    show({ upstream_price: { model: 'v/m', input: 1, output: 2 }, sell_input_price: 0, sell_output_price: 0 });
    expect(screen.getByTestId('supply-price-loss')).toBeTruthy();
  });

  it('marks a supply whose channel cost with its fee is above the quote, even at the listed price', () => {
    show({
      upstream_price: { model: 'v/m', input: 3, output: 15 },
      cost: { known: true, source: 'channel_price', fee_rate: 0.05, input: 3.15, output: 15.75, above_quote: true },
      traffic: 'balanced'
    });
    expect(screen.getByTestId('supply-price-loss')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /v\/m|m/ }));
    expect(screen.getByText(i18n.t('upstreamPrice.costWithFee', { rate: 5 }))).toBeTruthy();
    expect(screen.getByText('$3.15 / $15.75')).toBeTruthy();
    expect(screen.getByText(i18n.t('modelSupply.aboveQuote'))).toBeTruthy();
  });

  it('trusts the channel cost over the listed price when it is not above the quote', () => {
    show({
      upstream_price: { model: 'v/m', input: 1, output: 2 },
      sell_input_price: 0,
      sell_output_price: 0,
      cost: { known: true, source: 'cost_ratio', input: 0, output: 0, above_quote: false }
    });
    expect(screen.queryByTestId('supply-price-loss')).toBeNull();
  });

  it('says free, and tells a missing entry from a channel without a price list', () => {
    show({ upstream_price: { model: 'v/m', input: 0, output: 0 } });
    expect(screen.getByText('Free')).toBeTruthy();
    cleanup();
    show({ upstream_price: null });
    expect(screen.getByText('—').getAttribute('title')).toBe(i18n.t('channels.models.priceMissing'));
    cleanup();
    show({ upstream_price: null, upstream_priced: false });
    expect(screen.getByText('—').getAttribute('title')).toBe(i18n.t('modelSupply.noPriceList'));
  });

  it('opens the details with every price and a link to check it on', () => {
    show({
      upstream_model: 'deepseek/deepseek-v3.2',
      upstream_price: {
        model: 'deepseek/deepseek-v3.2',
        input: 0.62,
        output: 1.85,
        cache_read: 0.06,
        source: 'Vercel AI Gateway',
        page_url: 'https://vercel.com/ai-gateway/models/deepseek-v3.2',
        page_exact: true
      },
      sell_input_price: 0.58,
      sell_output_price: 1.68
    });
    fireEvent.click(screen.getByRole('button', { name: /deepseek\/deepseek-v3.2/ }));
    expect(screen.getByText('Cache read')).toBeTruthy();
    const link = screen.getByRole('link', { name: /Vercel AI Gateway/ });
    expect(link.getAttribute('href')).toBe('https://vercel.com/ai-gateway/models/deepseek-v3.2');
    expect(screen.getAllByText(/below the upstream|lower than|below/i).length).toBeGreaterThan(0);
  });
});

describe('UpstreamName', () => {
  afterEach(cleanup);

  it('puts the namespace on its own line and keeps the full name in the title', () => {
    render(createElement(UpstreamName, { upstream: 'accounts/fireworks/models/llama-v3p3-70b-instruct', model: 'llama' }));
    expect(screen.getByText('accounts/fireworks/models/')).toBeTruthy();
    expect(screen.getByText('llama-v3p3-70b-instruct')).toBeTruthy();
    expect(screen.getByRole('button').getAttribute('title')).toContain('accounts/fireworks/models/llama-v3p3-70b-instruct');
  });
});
