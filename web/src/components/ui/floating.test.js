import { describe, expect, it } from 'vitest';

import { computeFloatingStyle } from './floating';

const viewport = { innerWidth: 1000, innerHeight: 600 };
const anchorAt = (top, left = 100, width = 200, height = 36) => ({ top, bottom: top + height, left, right: left + width, width, height });
const place = (over) =>
  computeFloatingStyle({ width: 200, height: 200, side: 'bottom', align: 'start', offset: 4, matchWidth: false, clamp: true, viewport, ...over });

describe('computeFloatingStyle', () => {
  it('opens below the trigger when it fits', () => {
    const s = place({ anchor: anchorAt(100) });
    expect(s.top).toBe(140);
    expect(s.left).toBe(100);
  });

  it('moves above the trigger when there is more room there', () => {
    const s = place({ anchor: anchorAt(480) });
    expect(s.top).toBe(480 - 4 - 200);
  });

  it('caps the height to the room on the chosen side so it never leaves the window', () => {
    const s = place({ anchor: anchorAt(250), height: 900 });
    // 250 above vs 310 below: stays below, capped to the room there.
    expect(s.top).toBe(290);
    expect(s.maxHeight).toBe(600 - 286 - 4 - 8);
  });

  it('honours a smaller height cap and matches the trigger width', () => {
    const s = place({ anchor: anchorAt(100, 100, 320), height: 900, maxHeight: 288, matchWidth: true });
    expect(s.maxHeight).toBe(288);
    expect(s.minWidth).toBe(320);
  });

  it('keeps the panel inside the left and right edges', () => {
    expect(place({ anchor: anchorAt(100, 900) }).left).toBe(1000 - 200 - 8);
    expect(place({ anchor: anchorAt(100, 150), align: 'end' }).left).toBe(150 + 200 - 200);
  });
});
