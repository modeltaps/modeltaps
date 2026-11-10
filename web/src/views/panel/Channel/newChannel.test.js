import { describe, expect, it } from 'vitest';

import {
  applyBlockedModels,
  applyMappingToModels,
  autoChannelName,
  buildPartsKey,
  countKeys,
  isValidWildcard,
  maskInput,
  matchesEntry,
  normalizeKeyInput,
  resolveAllowedModels,
  slugify,
  toWildcard
} from './newChannel';

describe('keys', () => {
  it('counts one channel per non-empty key line', () => {
    expect(countKeys('')).toBe(0);
    expect(countKeys('sk-a')).toBe(1);
    expect(countKeys('sk-a\n\n  sk-b  \nsk-c\n')).toBe(3);
  });

  it('keeps a JSON credential as one key', () => {
    const json = '{\n  "type": "service_account",\n  "project_id": "p"\n}';
    expect(normalizeKeyInput(json)).toBe('{"type":"service_account","project_id":"p"}');
    expect(countKeys(json)).toBe(1);
  });

  it('masks keys after editing', () => {
    expect(maskInput('sk-proj-4f8a2c9e1b7d0x9Qk')).toBe('sk-proj-…x9Qk');
    expect(maskInput('abc')).toBe('abc…');
  });
});

describe('autoChannelName', () => {
  it('slugs the provider name and stays unique', () => {
    expect(slugify('My Cloud AI')).toBe('my-cloud-ai');
    expect(slugify('!!!')).toBe('channel');
    expect(autoChannelName('Acme', [])).toBe('acme');
    expect(autoChannelName('Acme', ['acme', 'acme-2'])).toBe('acme-3');
  });
});

describe('wildcards', () => {
  it('accepts only a trailing prefix wildcard', () => {
    expect(isValidWildcard('gpt-4.1*')).toBe(true);
    expect(isValidWildcard('*')).toBe(false);
    expect(isValidWildcard('gpt-*-mini')).toBe(false);
    expect(toWildcard('gpt-4.1')).toBe('gpt-4.1*');
    expect(toWildcard('gpt-4.1*')).toBe('gpt-4.1*');
  });

  it('matches exact names and globs', () => {
    expect(matchesEntry('gpt-4.1-mini', 'gpt-4.1*')).toBe(true);
    expect(matchesEntry('gpt-4.1', 'gpt-4.1')).toBe(true);
    expect(matchesEntry('gpt-4o', 'gpt-4.1*')).toBe(false);
    expect(matchesEntry('o3.mini', 'o3.*')).toBe(true);
    expect(matchesEntry('o3xmini', 'o3.mini')).toBe(false);
  });
});

describe('applyBlockedModels', () => {
  it('removes blocked models and adds them to the drift ignore rules', () => {
    const { models, driftIgnoreRules } = applyBlockedModels(['a', 'gpt-4o', 'gpt-4o-mini', 'b'], ['gpt-4o*', 'b'], '*-preview\nb');
    expect(models).toEqual(['a']);
    expect(driftIgnoreRules).toBe('*-preview\nb\ngpt-4o*');
  });

  it('leaves everything alone without blocked models', () => {
    expect(applyBlockedModels(['a'], [], '')).toEqual({ models: ['a'], driftIgnoreRules: '' });
  });
});

describe('resolveAllowedModels', () => {
  it('stores the upstream list for all models and turns on auto-add', () => {
    expect(resolveAllowedModels({ allow: ['*'], upstream: ['x', 'y'], defaults: ['d'] })).toEqual({
      models: ['x', 'y'],
      driftAutoAdd: true
    });
    expect(resolveAllowedModels({ allow: ['*'], upstream: [], defaults: ['d'] }).models).toEqual(['d']);
  });

  it('keeps explicit entries and adds mapping names', () => {
    expect(resolveAllowedModels({ allow: ['a', 'gpt-4.1*'], mappingKeys: ['alias', 'a'] })).toEqual({
      models: ['a', 'gpt-4.1*', 'alias'],
      driftAutoAdd: false
    });
  });
});

describe('buildPartsKey', () => {
  it('builds every supported format', () => {
    expect(buildPartsKey('apiKey', { region: 'us-east-1', token: 'tok' })).toBe('us-east-1|tok');
    expect(buildPartsKey('access', { region: 'r', accessKey: 'AK', secretKey: 'SK' })).toBe('r|AK|SK');
    expect(buildPartsKey('access', { region: 'r', accessKey: 'AK', secretKey: 'SK', sessionToken: 'S' })).toBe('r|AK|SK|S');
    expect(buildPartsKey('role', { region: 'r', accessKey: 'AK', secretKey: 'SK', roleArn: 'arn' })).toBe('r|AK|SK|arn');
    expect(buildPartsKey('role', { region: 'r', accessKey: 'AK', secretKey: 'SK', sessionToken: 'S', roleArn: 'arn' })).toBe(
      'r|AK|SK|S|arn'
    );
  });

  it('is empty while a required part is missing', () => {
    expect(buildPartsKey('access', { region: 'r', accessKey: 'AK' })).toBe('');
    expect(buildPartsKey('nope', {})).toBe('');
  });
});

describe('applyMappingToModels', () => {
  it('replaces mapped upstream names by their request names, adds the rest', () => {
    expect(applyMappingToModels(['gpt-4.1-2025', 'o3'], { 'gpt-4.1': '+gpt-4.1-2025', alias: 'deploy-x', o3: 'o3' })).toEqual([
      'gpt-4.1',
      'o3',
      'alias'
    ]);
    expect(applyMappingToModels(['a'], {})).toEqual(['a']);
  });
});
