import PropTypes from 'prop-types';
import { useMemo, useState, useEffect, useContext, useCallback, createContext } from 'react';

import { API } from 'utils/api';
import { NOTICE_STORAGE_KEY, resolveNotice } from './noticeState';

// The site announcement (the admin's `Notice` setting). It is fetched once per page load.
// An announcement the user has not read yet shows as a banner at the top of the panel
// (NoticeBanner); the dialog with the full text (NoticeDialogs) only opens on request, from the
// banner or from the account menu. Reading it there or dismissing the banner stores the text, so
// the banner stays away until the announcement changes.

export const NoticeContext = createContext(undefined);

const readStored = () => {
  try {
    return localStorage.getItem(NOTICE_STORAGE_KEY);
  } catch {
    return null; // storage disabled
  }
};

const writeStored = (value) => {
  try {
    if (value == null) localStorage.removeItem(NOTICE_STORAGE_KEY);
    else localStorage.setItem(NOTICE_STORAGE_KEY, value);
  } catch {
    // storage disabled: the banner comes back on the next page load
  }
};

export function NoticeProvider({ children }) {
  const [notice, setNotice] = useState(null);
  const [unread, setUnread] = useState(false);
  const [isOpen, setOpen] = useState(false);

  useEffect(() => {
    let cancelled = false;
    fetchNotice().then((fetched) => {
      if (cancelled) return;
      const next = resolveNotice(fetched, readStored());
      if (next.forgetRead) writeStored(null);
      setNotice(next.notice);
      setUnread(next.unread);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const dismissNotice = useCallback(() => {
    setUnread(false);
    if (notice) writeStored(notice);
  }, [notice]);

  const openNotice = useCallback(() => {
    setOpen(true);
    dismissNotice();
  }, [dismissNotice]);

  const closeNotice = useCallback(() => setOpen(false), []);

  const memoizedValue = useMemo(
    () => ({ notice, unread, isOpen, openNotice, closeNotice, dismissNotice }),
    [notice, unread, isOpen, openNotice, closeNotice, dismissNotice]
  );

  return <NoticeContext.Provider value={memoizedValue}>{children}</NoticeContext.Provider>;
}

NoticeProvider.propTypes = {
  children: PropTypes.node
};

export function useNotice() {
  const context = useContext(NoticeContext);

  if (context === undefined) {
    throw new Error('useNotice must be used within a NoticeProvider');
  }

  return context;
}

// The announcement text ('' when the site has none), or null when it could not be loaded.
async function fetchNotice() {
  try {
    const res = await API.get('/api/notice');
    const { success, data } = res.data;
    return success && typeof data === 'string' ? data : null;
  } catch (error) {
    console.error('Error fetching notice:', error);
    return null;
  }
}
