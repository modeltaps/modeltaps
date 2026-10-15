// @vitest-environment jsdom
import { useState } from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

import i18n from 'i18n/i18n';
import ModelListInput from './ModelListInput';

beforeAll(() => i18n.changeLanguage('en_US'));
afterAll(() => i18n.changeLanguage('zh_CN'));
afterEach(cleanup);

function Harness({ initial = '', suggestions }) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <ModelListInput value={value} onChange={setValue} suggestions={suggestions} />
      <output data-testid="value">{value}</output>
    </>
  );
}

const input = () => screen.getByRole('combobox');
const rows = () => screen.queryAllByRole('listbox').flatMap((l) => [...l.querySelectorAll('button')].map((b) => b.textContent));
const value = () => screen.getByTestId('value').textContent;

describe('ModelListInput suggestions', () => {
  it('offers candidates without the models already in the list', () => {
    render(<Harness initial="gpt-4o,Claude-X" suggestions={['gpt-4o', 'claude-x', 'gpt-6.1-sol', 'gpt-6.1-sol', 'o9']} />);
    fireEvent.focus(input());
    expect(rows()).toEqual(['gpt-6.1-sol', 'o9']);
  });

  it('adds a picked candidate and stops offering it', () => {
    render(<Harness initial="a" suggestions={['gpt-6.1-sol', 'gpt-6-astra']} />);
    fireEvent.change(input(), { target: { value: 'astra' } });
    expect(rows()).toEqual(['Add "astra"', 'gpt-6-astra']);
    fireEvent.mouseDown(screen.getByText('gpt-6-astra'));
    expect(value()).toBe('a,gpt-6-astra');
    expect(input().value).toBe('');
    fireEvent.focus(input());
    expect(rows()).toEqual(['gpt-6.1-sol']);
  });

  it('adds the typed text on Enter', () => {
    render(<Harness initial="a" suggestions={['gpt-6.1-sol']} />);
    fireEvent.change(input(), { target: { value: 'my-model' } });
    fireEvent.keyDown(input(), { key: 'Enter' });
    expect(value()).toBe('a,my-model');
  });

  it('adds the highlighted candidate on Enter after arrowing to it', () => {
    render(<Harness initial="a" suggestions={['gpt-6.1-sol', 'gpt-6-astra']} />);
    fireEvent.change(input(), { target: { value: 'gpt' } });
    fireEvent.keyDown(input(), { key: 'ArrowDown' });
    fireEvent.keyDown(input(), { key: 'Enter' });
    expect(value()).toBe('a,gpt-6.1-sol');
  });

  it('does not offer to add a model that is already listed', () => {
    render(<Harness initial="gpt-4o" suggestions={['gpt-4o-mini']} />);
    fireEvent.change(input(), { target: { value: 'GPT-4O' } });
    expect(rows()).toEqual(['gpt-4o-mini']);
    fireEvent.keyDown(input(), { key: 'Escape' });
    fireEvent.keyDown(input(), { key: 'Enter' });
    expect(value()).toBe('gpt-4o');
  });

  it('keeps the text editing mode', () => {
    render(<Harness initial="a,b" suggestions={['c']} />);
    fireEvent.click(screen.getByText('Edit as text'));
    expect(screen.getByRole('textbox').value).toBe('a,b');
  });
});
