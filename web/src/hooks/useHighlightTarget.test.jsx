// @vitest-environment jsdom
import { createElement } from 'react';
import { act, cleanup, render } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import useHighlightTarget from './useHighlightTarget';

// jsdom lays nothing out: report a box for every element unless the test hides it.
const hidden = new Set();
beforeEach(() => {
  vi.useFakeTimers();
  Element.prototype.scrollIntoView = vi.fn();
  vi.spyOn(Element.prototype, 'getClientRects').mockImplementation(function rects() {
    return hidden.has(this) ? [] : [{}];
  });
});
afterEach(() => {
  cleanup();
  hidden.clear();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

function Probe() {
  useHighlightTarget();
  return null;
}

const mount = (search, html, prepare) => {
  document.body.innerHTML = html;
  prepare?.(document.body);
  const container = document.createElement('div');
  document.body.appendChild(container);
  return render(createElement(MemoryRouter, { initialEntries: [`/x${search}`] }, createElement(Probe)), { container });
};

describe('useHighlightTarget', () => {
  it('flashes the shown copy of a list row', () => {
    const html =
      '<section data-highlight="telegram"><ul><li id="phone" data-highlight="telegram:4" data-highlight-row></li></ul>' +
      '<table><tbody><tr id="desk" data-highlight="telegram:4" data-highlight-row></tr></tbody></table></section>';
    // The phone list comes first in the DOM but is hidden by CSS on a desktop.
    mount('?highlight=telegram%3A4', html, (body) => hidden.add(body.querySelector('#phone')));
    expect(document.getElementById('desk').classList.contains('highlight-row-active')).toBe(true);
    expect(document.getElementById('phone').classList.contains('highlight-row-active')).toBe(false);
  });

  it('falls back to the card when the row never shows up', () => {
    mount('?highlight=telegram%3A99', '<section id="card" data-highlight="telegram"></section>');
    expect(document.getElementById('card').classList.contains('highlight-target-active')).toBe(false);
    act(() => {
      vi.advanceTimersByTime(3100);
    });
    expect(document.getElementById('card').classList.contains('highlight-target-active')).toBe(true);
  });
});
