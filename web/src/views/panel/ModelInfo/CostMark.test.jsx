// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { CostMark } from './ModelTable';
import { filterCatalogRows } from './index';

describe('CostMark', () => {
  beforeEach(() => i18n.changeLanguage('en'));
  afterEach(cleanup);

  it('shows nothing when no supply is above the quote or of unknown cost', () => {
    render(createElement(CostMark, { summary: null, unit: 'M' }));
    expect(screen.queryByTestId('cost-mark')).toBeNull();
    render(createElement(CostMark, { summary: { above_quote: 0, cost_unknown: 0 }, unit: 'M' }));
    expect(screen.queryByTestId('cost-mark')).toBeNull();
  });

  it('marks supplies above the quote and opens the supplies', () => {
    const onOpen = vi.fn();
    render(createElement(CostMark, { summary: { above_quote: 2, max_input: 5.775, max_output: 34.65 }, unit: 'M', onOpen }));
    const mark = screen.getByTestId('cost-mark');
    expect(mark.className).toContain('text-warning');
    expect(mark.getAttribute('aria-label')).toContain('2 supplies');
    // The costs come in USD per million tokens and are shown as they are in that unit.
    expect(mark.getAttribute('aria-label')).toContain('$5.775 / $34.65');
    fireEvent.click(mark);
    expect(onOpen).toHaveBeenCalled();
  });

  it('shows the costs per thousand tokens when the list does', () => {
    render(createElement(CostMark, { summary: { above_quote: 1, max_input: 5.775, max_output: 34.65 }, unit: 'K' }));
    expect(screen.getByTestId('cost-mark').getAttribute('aria-label')).toContain('$0.005775 / $0.03465');
  });

  it('marks supplies of unknown cost in a muted tone', () => {
    render(createElement(CostMark, { summary: { above_quote: 0, cost_unknown: 1 }, unit: 'M' }));
    expect(screen.getByTestId('cost-mark').className).toContain('text-muted-foreground');
  });
});

describe('cost filter', () => {
  const rows = [
    { key: 'a', model: 'a', catalogId: 1, cost_summary: { above_quote: 1 } },
    { key: 'b', model: 'b', catalogId: 2, cost_summary: { cost_unknown: 2 } },
    { key: 'c', model: 'c', catalogId: 3 }
  ];
  it('keeps the models with supplies above the quote or of unknown cost', () => {
    expect(filterCatalogRows(rows, { params: { cost: 'above_quote' } }).map((r) => r.model)).toEqual(['a']);
    expect(filterCatalogRows(rows, { params: { cost: 'cost_unknown' } }).map((r) => r.model)).toEqual(['b']);
  });
});
