// Helpers for the multi-model test dialog (ChannelTestDialog.jsx).

export const ENDPOINT_AUTO = 'auto';

export const ENDPOINT_OPTIONS = [
  { value: 'chat', label: 'chat (/v1/chat/completions)' },
  { value: 'response', label: 'response (/v1/responses)' },
  { value: 'embeddings', label: 'embeddings (/v1/embeddings)' },
  { value: 'image', label: 'image (/v1/images/generations)' }
];

// Requests to one channel in flight at once; more tends to trip upstream rate limits.
export const TEST_CONCURRENCY = 4;

// Models that can cost real money per test request (image / video generation, deep research).
// They stay unselected by default so "start" on a large channel cannot run up a bill.
const COSTLY_MODEL = /(deep-research|dall-e|gpt-image|imagen|flux|midjourney|^mj_|sora|veo|kling|suno|seedance)/i;

export const isCostlyTestModel = (model) => COSTLY_MODEL.test(model);

// The channel's models in list order, with the test model first when it is not listed,
// so the model the scheduled test uses can be checked here too. Duplicates and blanks dropped.
export function channelTestModels(channel) {
  const list = (channel?.models || '')
    .split(',')
    .map((m) => m.trim())
    .filter(Boolean);
  const testModel = (channel?.test_model || '').trim();
  if (testModel && !list.includes(testModel)) list.unshift(testModel);
  return Array.from(new Set(list));
}

export const defaultTestSelection = (models) => models.filter((m) => !isCostlyTestModel(m));

// Runs worker over items with at most `limit` in flight. Once isStopped() returns true no
// new item is started; requests already in flight finish on their own.
export async function runWithConcurrency(items, limit, worker, isStopped) {
  let cursor = 0;
  const runners = Array.from({ length: Math.min(limit, items.length) }, async () => {
    for (;;) {
      if (isStopped()) return;
      const index = cursor;
      cursor += 1;
      if (index >= items.length) return;
      await worker(items[index]);
    }
  });
  await Promise.all(runners);
}
