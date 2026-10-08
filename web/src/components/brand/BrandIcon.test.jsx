import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { afterEach, describe, expect, it } from 'vitest';

import BrandIcon from './BrandIcon';
import BrandIconPicker, { brandIconMode } from './BrandIconPicker';
import ModelIcon from './ModelIcon';
import { resolveBrandIconKey, setBrandIconManifest } from './brandIconManifest';

const manifest = {
  icons: {
    openai: { mono: true, color: false, scale: 1 },
    claude: { mono: true, color: true, scale: 1 },
    zhipu: { mono: true, color: true, scale: 1 },
    deepseek: { mono: true, color: true, scale: 1 },
    azure: { mono: true, color: true, scale: 1 },
    xai: { mono: true, color: false, scale: 0.82 },
    qwen: { mono: true, color: true, scale: 1 },
    alibabacloud: { mono: true, color: true, scale: 1 }
  },
  aliases: { glm: 'zhipu', microsoftazure: 'azure', x: 'xai', ghost: 'missing' },
  channelTypes: { 1: 'openai', 3: 'azure', 16: 'zhipu' },
  modelPrefixes: { gpt: 'openai', claude: 'claude', deepseek: 'deepseek', glm: 'zhipu', qwen: 'qwen', q: 'xai' },
  domains: { 'deepseek.com': 'deepseek', 'aliyuncs.com': 'alibabacloud', 'dashscope.aliyuncs.com': 'qwen' }
};

const resolve = (opts) => resolveBrandIconKey(manifest, opts);

describe('resolveBrandIconKey', () => {
  it('follows custom > alias > channel type > model prefix > base_url domain', () => {
    const all = {
      icon: 'brand:claude',
      brandKey: 'glm',
      channelType: 1,
      model: 'deepseek-chat',
      baseUrl: 'https://dashscope.aliyuncs.com/v1'
    };
    expect(resolve(all)).toBe('claude');
    expect(resolve({ ...all, icon: undefined })).toBe('zhipu');
    expect(resolve({ ...all, icon: undefined, brandKey: undefined })).toBe('openai');
    expect(resolve({ ...all, icon: undefined, brandKey: undefined, channelType: undefined })).toBe('deepseek');
    expect(resolve({ baseUrl: all.baseUrl })).toBe('qwen');
    expect(resolve({})).toBeNull();
  });

  it('accepts bare custom keys but ignores external URLs and uploads', () => {
    expect(resolve({ icon: 'azure' })).toBe('azure');
    expect(resolve({ icon: 'https://cdn.example.com/x.svg', brandKey: 'x' })).toBe('xai');
    expect(resolve({ icon: 'upload:12' })).toBeNull();
  });

  it('resolves keys and aliases case-insensitively and skips dangling aliases', () => {
    expect(resolve({ brandKey: 'MicrosoftAzure' })).toBe('azure');
    expect(resolve({ ownedBy: ' GLM ' })).toBe('zhipu');
    expect(resolve({ brandKey: 'ghost' })).toBeNull();
    expect(resolve({ brandKey: 'unknown', ownedBy: 'deepseek' })).toBe('deepseek');
  });

  it('maps channel types given as numbers or strings', () => {
    expect(resolve({ channelType: '16' })).toBe('zhipu');
    expect(resolve({ channelType: 999 })).toBeNull();
  });

  it('uses the longest model prefix and falls back to the last path segment', () => {
    expect(resolve({ model: 'qwen-max' })).toBe('qwen');
    expect(resolve({ model: 'GPT-4o' })).toBe('openai');
    expect(resolve({ model: 'deepseek-ai/DeepSeek-V3' })).toBe('deepseek');
    expect(resolve({ model: 'thudm/glm-4' })).toBe('zhipu');
    expect(resolve({ model: 'mystery-model' })).toBeNull();
  });

  it('matches the most specific domain, including bare hosts', () => {
    expect(resolve({ baseUrl: 'https://api.deepseek.com/v1' })).toBe('deepseek');
    expect(resolve({ baseUrl: 'oss.aliyuncs.com/path' })).toBe('alibabacloud');
    expect(resolve({ baseUrl: 'https://example.org' })).toBeNull();
    expect(resolve({ baseUrl: 'not a url' })).toBeNull();
  });
});

const render = (Component, props) => renderToStaticMarkup(createElement(Component, props));

describe('BrandIcon rendering', () => {
  afterEach(() => setBrandIconManifest(null));

  it('renders an empty same-size box while the manifest is not loaded', () => {
    const html = render(BrandIcon, { brandKey: 'openai', fallbackText: 'OpenAI' });
    expect(html).toContain('size-4');
    expect(html).not.toContain('>O</span>');
    expect(html).not.toContain('/api/brand-icon/');
  });

  it('renders color variants as a same-origin <img>', () => {
    setBrandIconManifest(manifest);
    const html = render(BrandIcon, { model: 'claude-3-5-sonnet' });
    expect(html).toMatch(/<img[^>]*src="\/api\/brand-icon\/claude\?variant=color"/);
  });

  it('renders mono-only marks with a CSS mask filled by currentColor for dark mode', () => {
    setBrandIconManifest(manifest);
    const html = render(BrandIcon, { channelType: 1 });
    expect(html).not.toContain('<img');
    expect(html).toContain('background-color:currentColor');
    expect(html).toContain('mask-image:url(&quot;/api/brand-icon/openai?variant=mono&quot;)');
  });

  it('applies the manifest optical scale to the mark', () => {
    setBrandIconManifest(manifest);
    expect(render(BrandIcon, { brandKey: 'x' })).toContain('transform:scale(0.82)');
    expect(render(BrandIcon, { brandKey: 'openai' })).not.toContain('transform');
  });

  it('falls back to a first-letter placeholder when nothing resolves', () => {
    setBrandIconManifest(manifest);
    const html = render(BrandIcon, { model: 'mystery', fallbackText: 'mystery' });
    expect(html).toContain('>M</span>');
    expect(html).not.toContain('/api/brand-icon/');
  });

  it('renders admin uploads from the upload endpoint, even before the manifest loads', () => {
    expect(render(BrandIcon, { icon: 'upload:42', ownedBy: 'openai' })).toMatch(/<img[^>]*src="\/api\/brand-icon\/upload\/42"/);
  });

  it('falls back to the cached domain favicon for unmapped hosts', () => {
    setBrandIconManifest(manifest);
    expect(render(BrandIcon, { baseUrl: 'https://api.example.org/v1', fallbackText: 'Acme' })).toMatch(
      /<img[^>]*src="\/api\/brand-icon\/domain\/api\.example\.org"/
    );
    expect(render(BrandIcon, { domain: 'cursor.com' })).toContain('/api/brand-icon/domain/cursor.com');
    expect(render(BrandIcon, { baseUrl: 'http://10.0.0.1:8080', fallbackText: 'Acme' })).toContain('>A</span>');
    expect(render(BrandIcon, { baseUrl: 'https://api.deepseek.com' })).toContain('/api/brand-icon/deepseek?variant=color');
  });

  it('never renders external icon URLs and honors a custom fallback node', () => {
    setBrandIconManifest(manifest);
    const html = render(BrandIcon, { icon: 'https://cdn.example.com/x.svg', fallbackText: 'Acme' });
    expect(html).not.toContain('cdn.example.com');
    expect(html).toContain('>A</span>');
    expect(render(BrandIcon, { fallback: createElement('i', { id: 'fb' }) })).toBe('<i id="fb"></i>');
  });

  it('resolves brandKey / ownedBy aliases and keeps ModelIcon props working', () => {
    setBrandIconManifest(manifest);
    expect(render(BrandIcon, { brandKey: 'microsoftazure', fallbackText: 'Azure' })).toContain(
      '/api/brand-icon/azure?variant=color'
    );
    expect(render(BrandIcon, { ownedBy: 'glm', fallbackText: 'glm' })).toContain('/api/brand-icon/zhipu?variant=color');
    expect(render(BrandIcon, { ownedBy: 'Acme', fallbackText: 'Acme' })).toContain('>A</span>');
    expect(render(ModelIcon, { model: 'gpt-4o', className: 'size-5' })).toContain('size-5');
    expect(render(ModelIcon, { model: 'gpt-4o' })).toContain('/api/brand-icon/openai?variant=mono');
  });
});

describe('BrandIconPicker', () => {
  afterEach(() => setBrandIconManifest(null));

  it('classifies icon values as auto, built-in or upload', () => {
    expect(brandIconMode('')).toBe('auto');
    expect(brandIconMode('  ')).toBe('auto');
    expect(brandIconMode(undefined)).toBe('auto');
    expect(brandIconMode('brand:openai')).toBe('builtin');
    expect(brandIconMode('upload:abc')).toBe('upload');
  });

  it('previews the value through BrandIcon on the same origin', () => {
    setBrandIconManifest(manifest);
    const onChange = () => {};
    expect(render(BrandIconPicker, { value: 'upload:abc', onChange })).toContain('/api/brand-icon/upload/abc');
    expect(render(BrandIconPicker, { value: 'brand:claude', onChange })).toContain('/api/brand-icon/claude?variant=color');
    expect(render(BrandIconPicker, { value: '', ownedBy: 'DeepSeek', onChange })).toContain('/api/brand-icon/deepseek?variant=color');
    const external = render(BrandIconPicker, { value: 'https://cdn.example.com/x.svg', fallbackText: 'Acme', onChange });
    expect(external).not.toContain('cdn.example.com');
  });
});
