// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react';
import { Provider } from 'react-redux';
import { createStore } from 'redux';
import { afterEach, describe, expect, it } from 'vitest';
import { KeyRound } from 'lucide-react';

import 'i18n/i18n';
import PageEmptyState, { docsUrl } from './PageEmptyState';

afterEach(cleanup);

const withDocs = (docs_link) => createStore(() => ({ siteInfo: { docs_link } }));

describe('docsUrl', () => {
  it('puts the locale folder between the site and the guide path', () => {
    expect(docsUrl('https://docs.example.com/', 'zh_CN', '/guide/tokens')).toBe('https://docs.example.com/zh/guide/tokens');
    expect(docsUrl('https://docs.example.com', 'zh_HK', '/guide/tokens')).toBe('https://docs.example.com/hk/guide/tokens');
    expect(docsUrl('https://docs.example.com', 'ja_JP', '/guide/tokens')).toBe('https://docs.example.com/ja/guide/tokens');
    expect(docsUrl('https://docs.example.com', 'en_US', '/guide/tokens')).toBe('https://docs.example.com/guide/tokens');
  });

  it('is empty without a docs site or a guide path', () => {
    expect(docsUrl('', 'en_US', '/guide/tokens')).toBe('');
    expect(docsUrl('https://docs.example.com', 'en_US', undefined)).toBe('');
  });
});

describe('PageEmptyState', () => {
  it('shows the title, description and action', () => {
    render(
      <PageEmptyState icon={KeyRound} title="No keys" description="Keys call the gateway." action={<button type="button">Create</button>} />
    );
    expect(screen.getByRole('heading', { name: 'No keys' })).toBeTruthy();
    expect(screen.getByText('Keys call the gateway.')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Create' })).toBeTruthy();
  });

  it('links the guide when the site has a docs link', () => {
    render(
      <Provider store={withDocs('https://docs.example.com')}>
        <PageEmptyState title="No keys" docsPath="/guide/tokens" />
      </Provider>
    );
    expect(screen.getByRole('link').getAttribute('href')).toMatch(/^https:\/\/docs\.example\.com(\/\w+)?\/guide\/tokens$/);
  });

  it('hides the docs button when no docs link is configured', () => {
    render(
      <Provider store={withDocs('')}>
        <PageEmptyState title="No keys" docsPath="/guide/tokens" />
      </Provider>
    );
    expect(screen.queryByRole('link')).toBeNull();
  });
});
