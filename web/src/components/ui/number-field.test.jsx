// @vitest-environment jsdom
import { createElement, useState } from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { NumberField } from './number-field';

afterEach(cleanup);

function Harness({ initial, ...props }) {
  const [value, setValue] = useState(initial);
  return createElement(NumberField, { 'aria-label': 'n', value, onChange: setValue, ...props });
}

describe('NumberField', () => {
  it('steps within its range with the buttons and the arrow keys', () => {
    const { container } = render(createElement(Harness, { initial: '98', min: 1, max: 99 }));
    const input = screen.getByLabelText('n');
    const [minus, plus] = container.querySelectorAll('button');
    fireEvent.click(plus);
    expect(input.value).toBe('99');
    expect(plus.disabled).toBe(true);
    fireEvent.keyDown(input, { key: 'ArrowDown' });
    fireEvent.click(minus);
    expect(input.value).toBe('97');
  });

  it('starts stepping from the placeholder when empty', () => {
    const { container } = render(createElement(Harness, { initial: '', placeholder: '3600', step: 60 }));
    fireEvent.click(container.querySelectorAll('button')[1]);
    expect(screen.getByLabelText('n').value).toBe('3660');
  });
});
