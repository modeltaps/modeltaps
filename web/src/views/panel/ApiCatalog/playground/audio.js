import { curlOpenAI, curlPost, shellQuote } from '../catalog/_helpers';

// ==============================|| PLAYGROUND — AUDIO HELPERS ||============================== //
// 音频面板的纯函数部分：请求体 / 表单字段的拼装、上传文件的前置校验、转写结果的归一化。
// 单测跑在 node 环境（无 DOM），所以这里不碰 React、不碰 FormData 之外的浏览器对象，
// 组件只负责把这些结果接到表单与渲染上。

export const TTS_PATH = '/v1/audio/speech';
export const STT_PATH = '/v1/audio/transcriptions';

export const TTS_RULE = { endpoints: ['audio.speech'] };
export const STT_RULE = { endpoints: ['audio.transcription'] };

export const TTS_VOICES = ['alloy', 'echo', 'fable', 'onyx', 'nova', 'shimmer'];
export const TTS_FORMATS = ['mp3', 'wav', 'opus', 'aac', 'flac', 'pcm'];
// 只给 json / verbose_json：text、srt、vtt 回的不是 JSON，面板拿不到结构化结果。
export const STT_FORMATS = ['json', 'verbose_json'];

// 上游对上传体积的硬限是 25 MB，本地先挡一次，省掉一次注定失败的往返。
export const MAX_AUDIO_BYTES = 25 * 1024 * 1024;
export const AUDIO_EXTENSIONS = ['.flac', '.m4a', '.mp3', '.mp4', '.mpeg', '.mpga', '.oga', '.ogg', '.wav', '.webm'];
// mp4 / webm 的 MIME 是 video/*，只靠 audio/* 会把合法容器挡在外面，故 accept 里连扩展名一起给。
export const AUDIO_ACCEPT = ['audio/*', ...AUDIO_EXTENSIONS].join(',');

// 等效 curl 里不回填真实文件路径（面板只拿得到文件名，拿不到磁盘路径），用占位符。
export const STT_FILE_PLACEHOLDER = '/path/to/audio.mp3';

export const speechFileName = (format) => `speech.${TTS_FORMATS.includes(format) ? format : 'mp3'}`;

// 语速：留空即不传（用上游默认 1.0）；超出 0.25–4.0 的值同样丢弃，避免拼出必然 400 的请求体。
export function parseSpeed(value) {
  const text = String(value ?? '').trim();
  if (text === '') return null;
  const speed = Number(text);
  if (!Number.isFinite(speed) || speed < 0.25 || speed > 4) return null;
  return speed;
}

export function buildSpeechBody({ model, input, voice, format, speed }) {
  const body = { model, input, voice, response_format: format };
  const parsed = parseSpeed(speed);
  if (parsed !== null && parsed !== 1) body.speed = parsed;
  return body;
}

// 转写表单里除文件之外的字段，顺序固定，便于 curl 与实际请求逐行对照。
export function transcriptionFields({ model, language, prompt, responseFormat }) {
  const fields = [
    ['model', model],
    ['response_format', responseFormat]
  ];
  if (String(language ?? '').trim()) fields.push(['language', String(language).trim()]);
  if (String(prompt ?? '').trim()) fields.push(['prompt', String(prompt).trim()]);
  return fields;
}

export function buildTranscriptionForm({ file, ...rest }) {
  const form = new FormData();
  form.append('file', file);
  for (const [name, value] of transcriptionFields(rest)) form.append(name, value);
  return form;
}

const extensionOf = (name) => {
  const text = String(name || '');
  const dot = text.lastIndexOf('.');
  return dot < 0 ? '' : text.slice(dot).toLowerCase();
};

// 选中文件的前置校验。返回 i18n key 而非文案：错误统一经面板的错误条呈现。
export function validateAudioFile(file) {
  if (!file) return { ok: false, i18nKey: 'playground.audio.stt.errors.missingFile' };
  const type = String(file.type || '');
  if (!type.startsWith('audio/') && !AUDIO_EXTENSIONS.includes(extensionOf(file.name))) {
    return { ok: false, i18nKey: 'playground.audio.stt.errors.unsupportedType' };
  }
  if (Number(file.size) > MAX_AUDIO_BYTES) return { ok: false, i18nKey: 'playground.audio.stt.errors.tooLarge' };
  return { ok: true };
}

// verbose_json 才有 segments；普通 json 只有 text。缺字段的响应按「没有分段」处理，不抛。
export function normalizeTranscription(payload) {
  if (typeof payload === 'string') return { text: payload, segments: [] };
  const text = typeof payload?.text === 'string' ? payload.text : '';
  const source = Array.isArray(payload?.segments) ? payload.segments : [];
  const segments = source.map((segment, index) => ({
    id: segment?.id ?? index,
    start: Number(segment?.start) || 0,
    end: Number(segment?.end) || 0,
    text: String(segment?.text ?? '').trim()
  }));
  return { text, segments };
}

// 分段时间戳：m:ss.s，超过一小时也只多累加分钟数，面板里够用。
export function formatTimestamp(seconds) {
  const total = Number.isFinite(Number(seconds)) && Number(seconds) > 0 ? Number(seconds) : 0;
  const minutes = Math.floor(total / 60);
  return `${minutes}:${(total % 60).toFixed(1).padStart(4, '0')}`;
}

// curl 里的密钥由调用方传占位符（CapabilitySection 的 API_KEY_PLACEHOLDER），
// 真实 key 只走 Authorization 头，不进面板文本。
export function speechCurl(baseUrl, apiKey, body, format) {
  return curlOpenAI(baseUrl, apiKey, TTS_PATH, body, [`--output ${speechFileName(format)}`]);
}

export function transcriptionCurl(baseUrl, apiKey, fields) {
  return curlPost(`${baseUrl}${STT_PATH}`, [
    `--header ${shellQuote(`Authorization: Bearer ${apiKey}`)}`,
    `--form ${shellQuote(`file=@${STT_FILE_PLACEHOLDER}`)}`,
    ...fields.map(([name, value]) => `--form ${shellQuote(`${name}=${value}`)}`)
  ]);
}
