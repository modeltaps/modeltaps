// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import i18n from 'i18n/i18n';
import { UpstreamPriceCell } from './supplyParts';

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
});
