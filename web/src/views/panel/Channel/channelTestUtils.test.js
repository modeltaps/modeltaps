import { describe, expect, it } from 'vitest';

import { channelTestModels, defaultTestSelection, isCostlyTestModel, runWithConcurrency } from './channelTestUtils';

describe('channelTestModels', () => {
  it('keeps list order, drops blanks and duplicates', () => {
    expect(channelTestModels({ models: 'a, b,,a , c' })).toEqual(['a', 'b', 'c']);
  });

  it('puts an unlisted test model first', () => {
    expect(channelTestModels({ models: 'a,b', test_model: 'x' })).toEqual(['x', 'a', 'b']);
    expect(channelTestModels({ models: 'a,b', test_model: 'b' })).toEqual(['a', 'b']);
  });

  it('handles a missing channel', () => {
    expect(channelTestModels(null)).toEqual([]);
  });
});

describe('defaultTestSelection', () => {
  it('leaves costly models unselected', () => {
    const models = ['gpt-4o-mini', 'o3-deep-research', 'dall-e-3', 'gpt-image-1', 'veo-3', 'mj_imagine', 'o3-pro'];
    expect(defaultTestSelection(models)).toEqual(['gpt-4o-mini', 'o3-pro']);
    expect(isCostlyTestModel('Imagen-4')).toBe(true);
  });
});

describe('runWithConcurrency', () => {
  it('runs every item with at most `limit` in flight', async () => {
    let inFlight = 0;
    let peak = 0;
    const done = [];
    await runWithConcurrency(
      [1, 2, 3, 4, 5, 6],
      2,
      async (item) => {
        inFlight += 1;
        peak = Math.max(peak, inFlight);
        await new Promise((r) => setTimeout(r, 1));
        inFlight -= 1;
        done.push(item);
      },
      () => false
    );
    expect(done.sort()).toEqual([1, 2, 3, 4, 5, 6]);
    expect(peak).toBe(2);
  });

  it('starts no new item after stop', async () => {
    let stopped = false;
    const started = [];
    await runWithConcurrency(
      [1, 2, 3, 4],
      1,
      async (item) => {
        started.push(item);
        if (item === 2) stopped = true;
      },
      () => stopped
    );
    expect(started).toEqual([1, 2]);
  });
});
