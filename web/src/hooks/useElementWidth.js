import { useEffect, useState } from 'react';

// Width (px) of an element, kept current as it resizes. Returns [ref, width]: pass `ref` as the
// element's ref (a callback ref, so it also works when the element mounts after the first render).
// width is 0 until the element is measured, or when ResizeObserver is unavailable.
export default function useElementWidth() {
  const [el, setEl] = useState(null);
  const [width, setWidth] = useState(0);
  useEffect(() => {
    if (!el || typeof ResizeObserver === 'undefined') return undefined;
    setWidth(Math.round(el.getBoundingClientRect().width));
    const ro = new ResizeObserver(([entry]) => setWidth(Math.round(entry.contentRect.width)));
    ro.observe(el);
    return () => ro.disconnect();
  }, [el]);
  return [setEl, width];
}
