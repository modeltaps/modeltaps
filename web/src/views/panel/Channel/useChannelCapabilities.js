import { useEffect, useState } from 'react';

import { fetchChannelCapabilities } from './channelApi';

// GET /api/channel/capabilities for the monitoring tab: null until it arrives (or when it fails).
export default function useChannelCapabilities() {
  const [caps, setCaps] = useState(null);
  useEffect(() => {
    let alive = true;
    fetchChannelCapabilities()
      .then((data) => alive && setCaps(data))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);
  return caps;
}
