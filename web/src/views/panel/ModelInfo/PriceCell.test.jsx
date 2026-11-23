// @vitest-environment jsdom
import { cleanup, render } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import 'i18n/i18n';
import { ValueFormatter } from 'utils/common';
import { PriceCell } from './ModelTable';

afterEach(cleanup);

const tokens = { type: 'tokens', input: 1.25, output: 2.5 };

describe('PriceCell side', () => {
  it('shows only the input or only the output of a per-token price', () => {
    const input = render(<PriceCell price={tokens} unit="M" side="input" />).container.textContent;
    cleanup();
    const output = render(<PriceCell price={tokens} unit="M" side="output" />).container.textContent;
    expect(input).toBe(ValueFormatter(tokens.input, true, true));
    expect(output).toBe(ValueFormatter(tokens.output, true, true));
    expect(input).not.toBe(output);
  });

  it('keeps a per-call price in the input column and leaves the output one empty', () => {
    const flat = { type: 'times', input: 0.04, output: 0 };
    expect(render(<PriceCell price={flat} unit="M" side="input" />).container.textContent).toContain(
      ValueFormatter(flat.input, true, false)
    );
    cleanup();
    expect(render(<PriceCell price={flat} unit="M" side="output" />).container.textContent).toBe('—');
  });
});
