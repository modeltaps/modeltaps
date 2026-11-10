// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { ProviderGrid } from './ProviderPicker';

beforeAll(() => {
  i18n.changeLanguage('en_US');
  Element.prototype.scrollIntoView ??= () => {};
});
afterAll(() => i18n.changeLanguage('zh_CN'));
afterEach(cleanup);

const sectionTitles = () => [...document.querySelectorAll('[data-section] h3')].map((h) => h.textContent);

describe('ProviderGrid', () => {
  it('lists providers by category, common first', () => {
    render(<ProviderGrid value={1} onPick={() => {}} />);
    expect(sectionTitles().slice(0, 4)).toEqual(['Common', 'Model vendors', 'Cloud platforms', 'Gateways']);
    expect(sectionTitles().at(-1)).toBe('Legacy');
  });

  it('shows how each provider connects', () => {
    render(<ProviderGrid value={1} onPick={() => {}} />);
    const card = document.querySelector('[data-section="subscription"] button');
    expect(card.textContent).toContain('Claude Code');
    expect(card.textContent).toContain('Account login');
  });

  it('searches across categories and hides the common duplicates', () => {
    render(<ProviderGrid value={1} onPick={() => {}} />);
    fireEvent.change(screen.getByPlaceholderText('Search providers'), { target: { value: 'claude' } });
    expect(sectionTitles()).toEqual(['Subscription accounts']);
    fireEvent.change(screen.getByPlaceholderText('Search providers'), { target: { value: 'zzz' } });
    expect(screen.getByText('No matching providers')).toBeTruthy();
  });

  it('reports the picked type as a number', () => {
    const onPick = vi.fn();
    render(<ProviderGrid value={1} onPick={onPick} />);
    fireEvent.click(document.querySelector('[data-section="gateway"] button'));
    expect(onPick).toHaveBeenCalledWith(20);
  });
});
