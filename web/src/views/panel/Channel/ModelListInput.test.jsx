// @vitest-environment jsdom
import { useState } from 'react';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import ModelListInput from './ModelListInput';

beforeAll(() => {
  i18n.changeLanguage('en_US');
  Element.prototype.scrollIntoView ??= () => {};
});
afterAll(() => i18n.changeLanguage('zh_CN'));
afterEach(cleanup);

function Harness({ initial = '', ...props }) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <ModelListInput value={value} onChange={setValue} {...props} />
      <output data-testid="value">{value}</output>
    </>
  );
}

const value = () => screen.getByTestId('value').textContent;
const open = () => fireEvent.click(document.getElementById('channel-models'));
const search = () => screen.getByRole('searchbox');
describe('ModelListInput', () => {
  it('shows the selection as a count and opens a grouped, newest-first list', () => {
    render(
      <Harness
        initial="openai/gpt-5.5"
        upstream={[
          { id: 'openai/gpt-5.5', releasedAt: 100 },
          { id: 'openai/gpt-6.1-sol', releasedAt: 200 }
        ]}
        suggestions={['claude-opus-5-5']}
      />
    );
    expect(screen.getByRole('combobox').textContent).toContain('1 models selected');
    open();
    const options = within(screen.getByRole('listbox')).getAllByRole('option');
    const ids = options.map((o) => o.textContent);
    expect(ids.findIndex((t) => t.startsWith('openai/gpt-6.1-sol'))).toBeLessThan(ids.findIndex((t) => t.startsWith('openai/gpt-5.5')));
    expect(ids.some((t) => t.startsWith('Anthropic'))).toBe(true);
  });

  it('checks rows without closing and writes a comma-separated value', () => {
    render(<Harness initial="a" suggestions={['gpt-6.1-sol', 'gpt-6-astra']} />);
    open();
    fireEvent.click(screen.getByText('gpt-6-astra'));
    fireEvent.click(screen.getByText('gpt-6.1-sol'));
    expect(value()).toBe('a,gpt-6-astra,gpt-6.1-sol');
    expect(screen.getByRole('listbox')).toBeTruthy();
  });

  it('adds a typed model on Enter and never duplicates one already listed', () => {
    render(<Harness initial="gpt-4o" suggestions={['gpt-4o-mini']} />);
    open();
    fireEvent.change(search(), { target: { value: 'my-model' } });
    fireEvent.keyDown(search(), { key: 'Enter' });
    expect(value()).toBe('gpt-4o,my-model');
    fireEvent.change(search(), { target: { value: 'GPT-4O' } });
    expect(screen.queryByText('Add "GPT-4O"')).toBeNull();
  });

  it('selects a whole vendor group from its header', () => {
    render(<Harness suggestions={['claude-opus-5-5', 'claude-sonnet-5-5', 'gpt-5.5']} />);
    open();
    fireEvent.click(screen.getByText('Anthropic'));
    expect(value()).toBe('claude-sonnet-5-5,claude-opus-5-5');
  });

  it('closes on Escape without letting the event reach the surrounding sheet', () => {
    const onDocKey = vi.fn((e) => e.defaultPrevented);
    document.addEventListener('keydown', onDocKey);
    render(<Harness suggestions={['gpt-5.5']} />);
    open();
    fireEvent.keyDown(search(), { key: 'Escape' });
    expect(screen.queryByRole('listbox')).toBeNull();
    expect(onDocKey.mock.results[0].value).toBe(true);
    document.removeEventListener('keydown', onDocKey);
  });

  it('opens the advanced picker from the footer and calls onOpen when opening', () => {
    const onAdvanced = vi.fn();
    const onOpen = vi.fn();
    render(<Harness suggestions={['gpt-5.5']} advanced={{ onClick: onAdvanced }} onOpen={onOpen} />);
    open();
    expect(onOpen).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByText('Advanced filters'));
    expect(onAdvanced).toHaveBeenCalledTimes(1);
  });

  it('keeps the text editing mode', () => {
    render(<Harness initial="a,b" suggestions={['c']} />);
    fireEvent.click(screen.getByText('Edit as text'));
    expect(screen.getByRole('textbox').value).toBe('a,b');
  });
});
