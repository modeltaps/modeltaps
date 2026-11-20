import { useEffect } from 'react';
import { useSearchParams } from 'react-router';

// ==============================|| DEEP-LINK HIGHLIGHT HOOK ||============================== //
// Universal "guide to setting" mechanism: reads `?highlight=<anchor>` from the URL,
// finds the element tagged with a matching `data-highlight` attribute, scrolls it into
// view (block: center) and draws a temporary ring that fades out (~3s). Tolerates
// asynchronously rendered targets (data fetch / tab switch) via a MutationObserver with
// a bounded wait. No parameter → zero side effects.
// Only a target that is shown counts: a settings page keeps its inactive groups mounted but hidden,
// two groups can use the same anchor (the personal and the organization `log-io`), and a list draws
// its rows twice (phone and desktop) with one of them hidden by CSS.
// A row of a list is `<card>:<row key>` (cards.jsx rowAnchor) and flashes its background; when the
// row does not show up (deleted, not on the first page), the card is highlighted instead.

const HIGHLIGHT_CLASS = 'highlight-target-active';
const ROW_CLASS = 'highlight-row-active';
const ROW_FALLBACK_MS = 3000;
const FADE_MS = 3000; // keep in sync with the highlight-target-fade animation in tailwind.css
const WAIT_TIMEOUT_MS = 8000;

export default function useHighlightTarget() {
  const [searchParams] = useSearchParams();
  const anchor = searchParams.get('highlight');

  useEffect(() => {
    if (!anchor) return undefined;

    let cancelled = false;
    let observer;
    let waitTimer;
    let removeTimer;

    const shown = (node) => !node.closest('[hidden]') && node.getClientRects().length > 0;
    const find = (value) =>
      Array.from(document.querySelectorAll('[data-highlight]')).find(
        (node) => node.getAttribute('data-highlight') === value && shown(node)
      );
    const cardAnchor = anchor.includes(':') ? anchor.slice(0, anchor.indexOf(':')) : null;
    let startedAt = Date.now();
    // The row itself, or its card once the row had time to load and did not.
    const findEl = () => find(anchor) || (cardAnchor && Date.now() - startedAt > ROW_FALLBACK_MS ? find(cardAnchor) : null);

    const activate = (el) => {
      if (cancelled) return;
      const cls = el.hasAttribute('data-highlight-row') ? ROW_CLASS : HIGHLIGHT_CLASS;
      el.scrollIntoView({ block: 'center', behavior: 'smooth' });
      el.classList.add(cls);
      removeTimer = setTimeout(() => el.classList.remove(cls), FADE_MS + 200);
    };

    startedAt = Date.now();
    const existing = find(anchor);
    if (existing) {
      activate(existing);
      return () => {
        cancelled = true;
        if (removeTimer) clearTimeout(removeTimer);
      };
    }

    // Target not mounted yet (async data / tab switch): watch the DOM until it appears.
    observer = new MutationObserver(() => {
      const el = findEl();
      if (!el) return;
      observer.disconnect();
      if (waitTimer) clearTimeout(waitTimer);
      activate(el);
    });
    // `hidden` toggles too: a mounted target in a hidden group shows up without a DOM insert.
    observer.observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['hidden'] });
    waitTimer = setTimeout(() => observer && observer.disconnect(), WAIT_TIMEOUT_MS);
    // A row that never renders leaves no DOM change to wake the observer: look for the card then.
    let fallbackTimer;
    if (cardAnchor) {
      fallbackTimer = setTimeout(() => {
        const el = findEl();
        if (!el || cancelled) return;
        observer.disconnect();
        activate(el);
      }, ROW_FALLBACK_MS + 50);
    }

    return () => {
      cancelled = true;
      if (observer) observer.disconnect();
      if (waitTimer) clearTimeout(waitTimer);
      if (fallbackTimer) clearTimeout(fallbackTimer);
      if (removeTimer) clearTimeout(removeTimer);
    };
  }, [anchor]);
}
