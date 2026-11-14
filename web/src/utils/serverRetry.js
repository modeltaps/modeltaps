// A request that failed because the server could not answer (no response at all, or a 5xx such
// as the dev proxy's 502 while the backend restarts), as opposed to an answer like 401.
export function isServerUnavailable(err) {
  if (!err) return false;
  if (!err.response) return true;
  return err.response.status >= 500;
}

// Waits between tries while the server is unavailable: about 20 seconds in all, enough for a
// backend restart.
export const SERVER_RETRY_DELAYS = [500, 1000, 1500, 2000, 3000, 3000, 3000, 3000, 3000];

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

// Runs `fn` and, while it fails with the server unavailable, runs it again after each delay.
// Any other failure, or the last one, is thrown.
export async function retryWhileUnavailable(fn, delays = SERVER_RETRY_DELAYS, wait = sleep) {
  for (let attempt = 0; ; attempt += 1) {
    try {
      return await fn();
    } catch (err) {
      if (!isServerUnavailable(err) || attempt >= delays.length) throw err;
      await wait(delays[attempt]);
    }
  }
}
