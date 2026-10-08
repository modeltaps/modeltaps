import { useEffect } from 'react';
import { useSearchParams } from 'react-router';

// ==============================|| DEEP-LINK HIGHLIGHT HOOK ||============================== //
// Universal "guide to setting" mechanism: reads `?highlight=<anchor>` from the URL,
// finds the element tagged with a matching `data-highlight` attribute, scrolls it into
// view (block: center) and draws a temporary ring that fades out (~3s). Tolerates
// asynchronously rendered targets (data fetch / tab switch) via a MutationObserver with
// a bounded wait. No parameter → zero side effects.

const HIGHLIGHT_CLASS = 'highlight-target-active';
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

    const findEl = () =>
      Array.from(document.querySelectorAll('[data-highlight]')).find((node) => node.getAttribute('data-highlight') === anchor);

    const activate = (el) => {
      if (cancelled) return;
      el.scrollIntoView({ block: 'center', behavior: 'smooth' });
      el.classList.add(HIGHLIGHT_CLASS);
      removeTimer = setTimeout(() => el.classList.remove(HIGHLIGHT_CLASS), FADE_MS + 200);
    };

    const existing = findEl();
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
    observer.observe(document.body, { childList: true, subtree: true });
    waitTimer = setTimeout(() => observer && observer.disconnect(), WAIT_TIMEOUT_MS);

    return () => {
      cancelled = true;
      if (observer) observer.disconnect();
      if (waitTimer) clearTimeout(waitTimer);
      if (removeTimer) clearTimeout(removeTimer);
    };
  }, [anchor]);
}
