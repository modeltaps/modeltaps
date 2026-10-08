import { useCallback, useMemo } from 'react';

import { normalizeTranscription, speechFileName, validateAudioFile } from 'views/panel/ApiCatalog/playground/audio';
import { useConsole } from '../shared/ConsoleContext';
import { buildSnippets } from '../shared/codegen';
import { STT_FIELDS, STT_RULE, TTS_FIELDS, TTS_RULE, fieldParams, resolveValues } from '../shared/fieldSchema';
import { createSessionStore, playgroundAuthHeaders, relaySend, useSessionState } from '../shared/sessionStore';
import useModalityModel from '../shared/useModalityModel';

// ==============================|| PLAYGROUND — AUDIO SESSION ||============================== //
// 语音模态的会话：两个子方向（文本转语音 / 语音转文本）各自一套模型候选、参数与结果，
// 外加 TTS 的两步流（选音色 → 生成）。状态放模块级 store，「表单 | 代码」切 tab 不丢。
// 合成回的是二进制：object URL 换新与重置时都显式回收，否则整页停留期间一直攒着。

export const TTS_PATH = '/v1/audio/speech';
export const STT_PATH = '/v1/audio/transcriptions';

export const DEFAULT_SPEECH_TEXT = 'Welcome to the Modeltaps gateway. Point base_url at the gateway to connect.';

export const buildSpeechBody = ({ model, input, params = {} }) => ({ model, input, ...params });

// 转写表单里除文件之外的字段，顺序固定，便于等效代码与实际请求逐行对照。
// timestamp_granularities 在 OpenAI 侧是数组，multipart 里要写成带方括号的字段名。
export function transcriptionFields({ model, params = {} }) {
  const fields = [['model', model]];
  for (const [key, value] of Object.entries(params)) {
    fields.push([key === 'timestamp_granularities' ? 'timestamp_granularities[]' : key, String(value)]);
  }
  return fields;
}

// 等效代码里 timestamp_granularities 要写成数组：Python / JavaScript 的 SDK 收数组，
// 只有 multipart 才需要 `[]` 后缀的字段名（见 transcriptionFields）。
export const transcriptionParams = (params = {}) =>
  params.timestamp_granularities ? { ...params, timestamp_granularities: [params.timestamp_granularities] } : params;

export function buildTranscriptionForm({ file, model, params }) {
  const form = new FormData();
  form.append('file', file);
  for (const [name, value] of transcriptionFields({ model, params })) form.append(name, value);
  return form;
}

// 结果区元信息：耗时 · 格式（合成）/ 耗时 · 分段数 · 语言（转写）。缺的项不占位。
export function speechResultMeta({ elapsedMs = 0, values = {} }) {
  const facts = [];
  if (elapsedMs > 0) facts.push(`${(elapsedMs / 1000).toFixed(2)}s`);
  if (values.voice) facts.push(values.voice);
  if (values.response_format) facts.push(values.response_format);
  return facts;
}

export function transcriptionResultMeta({ elapsedMs = 0, result }) {
  const facts = [];
  if (elapsedMs > 0) facts.push(`${(elapsedMs / 1000).toFixed(2)}s`);
  if (result?.segments?.length) facts.push(`${result.segments.length}`);
  return facts;
}

// 上游不一定按 response_format 回（有的只出 mp3 / wav），下载扩展名以响应的 Content-Type 为准；
// 认不出的类型返回空串，由调用方回落所选格式。
const FORMAT_BY_CONTENT_TYPE = {
  'audio/mpeg': 'mp3',
  'audio/mp3': 'mp3',
  'audio/wav': 'wav',
  'audio/wave': 'wav',
  'audio/x-wav': 'wav',
  'audio/vnd.wave': 'wav',
  'audio/opus': 'opus',
  'audio/ogg': 'opus',
  'audio/aac': 'aac',
  'audio/x-aac': 'aac',
  'audio/flac': 'flac',
  'audio/x-flac': 'flac',
  'audio/pcm': 'pcm',
  'audio/l16': 'pcm'
};

// 裸 PCM 没有容器头，浏览器的 <audio> 放不了，只能下载。
export const isPlayableSpeechFormat = (format) => format !== 'pcm';

export const formatFromContentType = (type) =>
  FORMAT_BY_CONTENT_TYPE[
    String(type || '')
      .split(';')[0]
      .trim()
      .toLowerCase()
  ] || '';

// 选过的音色记在 localStorage，刷新后仍是它；不属于当前模型时由 resolveValues 回落该模型默认音色。
const VOICE_STORAGE_KEY = 'playground.ttsVoice';

export function readStoredVoice() {
  try {
    return (typeof localStorage !== 'undefined' && localStorage.getItem(VOICE_STORAGE_KEY)) || '';
  } catch {
    return '';
  }
}

function storeVoice(voice) {
  try {
    if (typeof localStorage !== 'undefined') localStorage.setItem(VOICE_STORAGE_KEY, voice);
  } catch {
    /* 存不下就只在本次会话里生效 */
  }
}

const storedVoice = readStoredVoice();

const INITIAL = {
  subtab: 'tts',
  step: 'voice',
  input: DEFAULT_SPEECH_TEXT,
  ttsValues: storedVoice ? { voice: storedVoice } : {},
  ttsStatus: 'idle',
  ttsError: null,
  ttsUrl: '',
  ttsFormat: 'mp3',
  ttsElapsedMs: 0,
  ttsTab: 'preview',
  sttValues: {},
  source: 'upload',
  file: null,
  fileError: null,
  recording: false,
  sttStatus: 'idle',
  sttError: null,
  sttResult: null,
  sttElapsedMs: 0
};

const store = createSessionStore(INITIAL);

let ttsController = null;
let sttController = null;
let objectUrl = '';

const swapObjectUrl = (next) => {
  if (objectUrl) URL.revokeObjectURL(objectUrl);
  objectUrl = next;
  return next;
};

export async function runSpeech({ model, input, params }) {
  ttsController?.abort();
  ttsController = new AbortController();
  const startedAt = Date.now();
  store.setState({ ttsStatus: 'running', ttsError: null, ttsUrl: swapObjectUrl(''), ttsElapsedMs: 0 });

  try {
    const headers = await playgroundAuthHeaders();
    const blob = await relaySend({
      path: TTS_PATH,
      headers,
      json: buildSpeechBody({ model, input, params }),
      parse: 'blob',
      signal: ttsController.signal
    });
    store.setState({
      ttsStatus: 'done',
      ttsUrl: swapObjectUrl(URL.createObjectURL(blob)),
      ttsFormat: formatFromContentType(blob.type) || params?.response_format || 'mp3',
      ttsElapsedMs: Date.now() - startedAt
    });
  } catch (err) {
    if (err?.aborted) store.setState({ ttsStatus: 'idle' });
    else store.setState({ ttsStatus: 'error', ttsError: err, ttsElapsedMs: Date.now() - startedAt });
  }
}

export async function runTranscription({ model, file, params }) {
  sttController?.abort();
  sttController = new AbortController();
  const startedAt = Date.now();
  store.setState({ sttStatus: 'running', sttError: null, sttResult: null, sttElapsedMs: 0 });

  try {
    const headers = await playgroundAuthHeaders();
    const payload = await relaySend({
      path: STT_PATH,
      headers,
      form: buildTranscriptionForm({ file, model, params }),
      signal: sttController.signal
    });
    store.setState({
      sttStatus: 'done',
      sttResult: normalizeTranscription(payload),
      sttElapsedMs: Date.now() - startedAt
    });
  } catch (err) {
    if (err?.aborted) store.setState({ sttStatus: 'idle' });
    else store.setState({ sttStatus: 'error', sttError: err, sttElapsedMs: Date.now() - startedAt });
  }
}

export { speechFileName };

// 麦克风录音：浏览器不支持 MediaRecorder / getUserMedia 时整个「录音」输入源不渲染。
export const canRecordAudio = () =>
  typeof window !== 'undefined' && typeof window.MediaRecorder !== 'undefined' && Boolean(navigator?.mediaDevices?.getUserMedia);

let recorder = null;

export async function startRecording() {
  if (!canRecordAudio() || recorder) return;
  const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
  const chunks = [];
  recorder = new MediaRecorder(stream);
  recorder.ondataavailable = (event) => event.data?.size && chunks.push(event.data);
  recorder.onstop = () => {
    stream.getTracks().forEach((track) => track.stop());
    const blob = new Blob(chunks, { type: recorder?.mimeType || 'audio/webm' });
    recorder = null;
    store.setState({ recording: false, fileError: null, file: new File([blob], 'recording.webm', { type: blob.type }) });
  };
  recorder.start();
  store.setState({ recording: true, fileError: null });
}

export function stopRecording() {
  recorder?.stop();
}

export default function useAudioSession() {
  const state = useSessionState(store);
  const { baseUrl, setSelectedModel, loading, error: catalogError, reload } = useConsole();

  const tts = state.subtab === 'tts';
  const fields = tts ? TTS_FIELDS : STT_FIELDS;

  // 两个子方向各有一套候选，规则随 tab 切换。
  const { options, model } = useModalityModel(tts ? TTS_RULE : STT_RULE);

  const stored = tts ? state.ttsValues : state.sttValues;
  const values = useMemo(() => resolveValues(fields, model, stored), [fields, model, stored]);
  const params = useMemo(() => fieldParams(fields, model, values), [fields, model, values]);

  const snippets = useMemo(
    () =>
      buildSnippets({
        modality: tts ? 'audio.speech' : 'audio.transcription',
        baseUrl,
        model: model?.id || '',
        prompt: tts ? state.input : '',
        params: tts ? params : transcriptionParams(params),
        audio: state.file ? { name: state.file.name } : undefined
      }),
    [tts, baseUrl, model, state.input, state.file, params]
  );

  const setValue = useCallback((key, value) => {
    if (store.getState().subtab === 'tts' && key === 'voice') storeVoice(value);
    store.setState((prev) =>
      prev.subtab === 'tts' ? { ttsValues: { ...prev.ttsValues, [key]: value } } : { sttValues: { ...prev.sttValues, [key]: value } }
    );
  }, []);

  const setFile = useCallback((file) => {
    if (!file) return store.setState({ file: null, fileError: null });
    const checked = validateAudioFile(file);
    return store.setState(checked.ok ? { file, fileError: null } : { file: null, fileError: checked.i18nKey });
  }, []);

  const run = useCallback(() => {
    if (!model) return undefined;
    return tts
      ? runSpeech({ model: model.id, input: state.input, params })
      : runTranscription({ model: model.id, file: state.file, params });
  }, [tts, model, state.input, state.file, params]);

  const abort = useCallback(() => {
    ttsController?.abort();
    sttController?.abort();
  }, []);

  return {
    ...state,
    tts,
    fields,
    model,
    options,
    values,
    params,
    snippets,
    endpoint: tts ? TTS_PATH : STT_PATH,
    status: tts ? state.ttsStatus : state.sttStatus,
    error: tts ? state.ttsError : state.sttError,
    meta: tts
      ? speechResultMeta({ elapsedMs: state.ttsElapsedMs, values })
      : transcriptionResultMeta({ elapsedMs: state.sttElapsedMs, result: state.sttResult }),
    loading,
    catalogError,
    reload,
    setSelectedModel,
    setSubtab: useCallback((subtab) => store.setState({ subtab }), []),
    setStep: useCallback((step) => store.setState({ step }), []),
    setInput: useCallback((input) => store.setState({ input }), []),
    setSource: useCallback((source) => store.setState({ source }), []),
    setTtsTab: useCallback((ttsTab) => store.setState({ ttsTab }), []),
    setValue,
    setFile,
    startRecording,
    stopRecording,
    run,
    abort
  };
}
