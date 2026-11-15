import { useCallback, useEffect, useState } from 'react';

import { API } from 'utils/api';

// GET /api/option/mail: whether mail is set in the deployment (smtp.url / SMTP_URL) and how,
// whether the platform can send mail, and the user email templates with their defaults. Shared by
// the mail cards; one request per page load, refreshed after a save.
let cache = null;
let pending = null;
const listeners = new Set();

async function fetchStatus() {
  if (!pending) {
    pending = API.get('/api/option/mail')
      .then((res) => (res.data?.success ? res.data.data : null))
      .catch(() => null)
      .then((data) => {
        pending = null;
        if (data) {
          cache = data;
          listeners.forEach((fn) => fn(data));
        }
        return data;
      });
  }
  return pending;
}

export default function useMailStatus() {
  const [status, setStatus] = useState(cache);
  useEffect(() => {
    listeners.add(setStatus);
    if (!cache) fetchStatus();
    return () => listeners.delete(setStatus);
  }, []);
  const refresh = useCallback(() => fetchStatus(), []);
  return { status, refresh };
}
