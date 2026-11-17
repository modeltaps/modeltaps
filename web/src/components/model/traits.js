// What a model is and what it can do, as the model lists mark it next to a model's name.
// One hue per medium across the app, so a kind and a trait about the same medium match: image
// amber, audio teal, video rose. The hues are the chart palette, tuned for both themes.
// Pure functions only (no React), so the Playground index and tests can use them directly.

const TONES = {
  image: 'var(--chart-3)',
  audio: 'var(--chart-6)',
  video: 'var(--chart-5)',
  file: 'var(--chart-8)',
  retrieval: 'var(--chart-7)',
  tools: 'var(--chart-1)',
  reasoning: 'var(--chart-4)',
  structured: 'var(--chart-2)'
};

// Kinds tagged next to a name; a chat model is the default and carries no tag.
export const MODEL_KINDS = ['image', 'video', 'speech', 'transcription', 'audio', 'realtime', 'embedding', 'rerank', 'decision'];

const KIND_TONES = {
  chat: 'var(--muted-foreground)',
  image: TONES.image,
  video: TONES.video,
  speech: TONES.audio,
  transcription: TONES.audio,
  audio: TONES.audio,
  realtime: TONES.audio,
  embedding: TONES.retrieval,
  rerank: TONES.retrieval,
  decision: TONES.retrieval
};

export const kindTone = (kind) => KIND_TONES[kind];

// Traits in display order: what a chat model takes in beyond text, then what it can do.
// `label` is the i18n key of the name shown on hover.
export const TRAITS = [
  { key: 'vision', tone: TONES.image, label: 'modelInfoPage.inputOptions.image' },
  { key: 'audio_input', tone: TONES.audio, label: 'modelInfoPage.inputOptions.audio' },
  { key: 'video_input', tone: TONES.video, label: 'modelInfoPage.inputOptions.video' },
  { key: 'pdf_input', tone: TONES.file, label: 'modelInfoPage.inputOptions.pdf' },
  { key: 'tool_call', tone: TONES.tools, label: 'modelpricePage.capability.tool_call' },
  { key: 'reasoning', tone: TONES.reasoning, label: 'modelpricePage.capability.reasoning' },
  { key: 'structured_output', tone: TONES.structured, label: 'modelpricePage.capability.structured_output' }
];

export const TRAIT_KEYS = TRAITS.map((trait) => trait.key);
export const traitOf = (key) => TRAITS.find((trait) => trait.key === key);

const INPUT_TRAITS = { image: 'vision', audio: 'audio_input', video: 'video_input', pdf: 'pdf_input', file: 'pdf_input' };

const asList = (value) => {
  if (Array.isArray(value)) return value;
  if (typeof value !== 'string' || !value) return [];
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
};

// The traits a model shows, in display order. Inputs count only for a chat model: an audio input
// says nothing new about a transcription model. Values outside the vocabulary are dropped.
export function modelTraits({ kind = '', inputModalities, capabilities } = {}) {
  const found = new Set(asList(capabilities));
  if (!kind || kind === 'chat') {
    asList(inputModalities).forEach((m) => {
      const trait = INPUT_TRAITS[String(m).toLowerCase()];
      if (trait) found.add(trait);
    });
  }
  return TRAIT_KEYS.filter((key) => found.has(key));
}

// The catalog's kind rules (model.CatalogModelKind) for a model the catalog API sends without
// one: endpoints without chat name the kind, then the output modalities, then the name.
const ENDPOINT_KINDS = [
  ['images', 'image'],
  ['audio.speech', 'speech'],
  ['audio.transcription', 'transcription'],
  ['embeddings', 'embedding'],
  ['rerank', 'rerank'],
  ['decisions', 'decision'],
  ['realtime', 'realtime']
];

const OUTPUT_KINDS = [
  ['video', 'video'],
  ['image', 'image'],
  ['transcription', 'transcription'],
  ['speech', 'speech'],
  ['embeddings', 'embedding'],
  ['embedding', 'embedding'],
  ['audio', 'audio']
];

const NAME_KINDS = [
  ['decision', /(^|[/_-])(decisions?|decider)([_.-]|$)/],
  ['rerank', /rerank/],
  ['embedding', /embed/],
  ['transcription', /transcri|whisper|scribe|(^|[/_-])stt([_.-]|$)|(^|[/_-])asr([_.-]|$)/],
  ['audio', /music|sound-effect|lyria/],
  ['speech', /(^|[/_-])tts([_.-]|$)|speech|(^|\/)eleven-|voice|^fish-audio\//],
  ['video', /video|(^|[/_-])veo-|(^|[/_-])kling|seedance|(^|[/_-])wan-|(^|[/_-])sora|hailuo|[it2r]2v\b/],
  ['image', /image|imagen|flux|dall-e|seedream|recraft|ideogram|stable-diffusion|sdxl|nano-banana|midjourney/]
];

const lastTwoSegments = (name) => {
  const parts = String(name || '')
    .trim()
    .toLowerCase()
    .split('/');
  return parts.slice(-2).join('/');
};

// "" for a chat model, as the backend.
export function modelKindOf({ name = '', endpoints, outputModalities } = {}) {
  const list = asList(endpoints);
  if (list.length > 0 && !list.includes('chat') && !list.includes('responses')) {
    const hit = ENDPOINT_KINDS.find(([endpoint]) => list.includes(endpoint));
    if (hit) return hit[1];
  }
  const output = new Set(asList(outputModalities).map((m) => String(m).toLowerCase()));
  const byOutput = OUTPUT_KINDS.find(([modality]) => output.has(modality));
  if (byOutput) return byOutput[1];
  const n = lastTwoSegments(name);
  const byName = NAME_KINDS.find(([, re]) => re.test(n));
  return byName ? byName[0] : '';
}
