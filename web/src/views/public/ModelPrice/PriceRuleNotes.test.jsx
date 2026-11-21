// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, render } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import i18n from 'i18n/i18n';
import PriceRuleNotes from './PriceRuleNotes';

// The pricing page lists what makes a request cost more or less than the price in its table.

afterEach(cleanup);

const offPeak = { ratio: 0.5, peak: [{ days: [1, 2, 3, 4, 5], from: '01:00', to: '04:00' }] };

describe('PriceRuleNotes', () => {
  it('states the long-context tier', () => {
    const price = { type: 'tokens', long_context: { threshold: 272000, input_ratio: 2, output_ratio: 1.5 } };
    const { container } = render(createElement(PriceRuleNotes, { price }));
    expect(container.textContent).toContain('272,000');
    expect(container.textContent).toBe(i18n.t('modelpricePage.longContextRule', { threshold: '272,000', input: 2, output: 1.5 }));
  });

  it('states the off-peak share and the peak hours', () => {
    const { container } = render(createElement(PriceRuleNotes, { price: { type: 'tokens', off_peak: offPeak } }));
    expect(container.textContent).toContain('50%');
    expect(container.textContent).toContain('01:00–04:00');
  });

  it('says nothing for a price without either, and for prices per call', () => {
    expect(render(createElement(PriceRuleNotes, { price: { type: 'tokens' } })).container.textContent).toBe('');
    // A tier whose ratios leave the price as listed is no tier.
    const flat = { type: 'tokens', long_context: { threshold: 200000, input_ratio: 1, output_ratio: 0 } };
    expect(render(createElement(PriceRuleNotes, { price: flat })).container.textContent).toBe('');
    const perCall = { type: 'times', long_context: { threshold: 1000, input_ratio: 2, output_ratio: 2 }, off_peak: offPeak };
    expect(render(createElement(PriceRuleNotes, { price: perCall })).container.textContent).toBe('');
  });
});
