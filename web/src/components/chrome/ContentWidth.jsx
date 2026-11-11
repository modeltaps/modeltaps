import { createContext, useContext, useEffect } from 'react';

// A page can tell the shell how wide its content needs to be (for example a table's natural
// width). The shell then collapses the sidebar to its icon rail as soon as the expanded sidebar
// would leave less room than that, before the page has to fall back to a smaller layout.
export const ContentMinWidthContext = createContext(() => {});

// Registers `width` (px, or null for none) while the calling page is mounted.
export function useContentMinWidth(width) {
  const setWidth = useContext(ContentMinWidthContext);
  useEffect(() => {
    setWidth(width || null);
  }, [setWidth, width]);
  useEffect(() => () => setWidth(null), [setWidth]);
}
