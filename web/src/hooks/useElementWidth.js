import { useEffect, useRef, useState } from 'react';

// Width (px) of an element, kept current as it resizes. Returns [ref, width]; width is 0 until
// the element is measured (or when ResizeObserver is unavailable).
export default function useElementWidth() {
  const ref = useRef(null);
  const [width, setWidth] = useState(0);
  useEffect(() => {
    const el = ref.current;
    if (!el || typeof ResizeObserver === 'undefined') return undefined;
    const ro = new ResizeObserver(([entry]) => setWidth(Math.round(entry.contentRect.width)));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  return [ref, width];
}
