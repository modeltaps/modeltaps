import { useCallback, useMemo } from 'react';

import { useConsole } from '../shared/ConsoleContext';
import { buildSnippets, chatImageBody } from '../shared/codegen';
import { IMAGE_FIELDS, IMAGE_RULES, fieldParams, isChatImageModel, resolveValues, supportsImageToImage } from '../shared/fieldSchema';
import { createSessionStore, playgroundAuthHeaders, relaySend, useSessionState } from '../shared/sessionStore';
import useModalityModel from '../shared/useModalityModel';

// ==============================|| PLAYGROUND — IMAGE SESSION ||============================== //
// 图像模态的会话：提示词 / 参考图 / schema 参数 / 一次运行的结果，全部放模块级 store，
// 「表单 | 代码」切 tab 不丢。参数只由 fieldSchema 决定，等效代码与真实请求同一份来源。
// 无参考图走 /v1/images/generations（JSON）；带参考图（仅 image-to-image 模型可选）走
// /v1/images/edits（multipart）。对话出图模型（chat 接口 + 输出模态含 image）一律走
// /v1/chat/completions，参考图作为 image_url 内容块放进用户消息。

export const IMAGE_GENERATIONS_PATH = '/v1/images/generations';
export const IMAGE_EDITS_PATH = '/v1/images/edits';
export const CHAT_COMPLETIONS_PATH = '/v1/chat/completions';

export const DEFAULT_IMAGE_PROMPT = 'a red panda coding at night, warm desk lamp, cinematic lighting';

export const imagePath = (reference, chat = false) => {
  if (chat) return CHAT_COMPLETIONS_PATH;
  return reference ? IMAGE_EDITS_PATH : IMAGE_GENERATIONS_PATH;
};

export const buildImageBody = ({ model, prompt, params = {} }) => ({ model, prompt, ...params });

// edits 走 multipart：图片是文件，其余字段与 generations 同名同值。
export function buildImageForm({ file, model, prompt, params = {} }) {
  const form = new FormData();
  form.append('image', file);
  form.append('model', model);
  form.append('prompt', prompt);
  for (const [key, value] of Object.entries(params)) form.append(key, String(value));
  return form;
}

// 回包归一化：data[] 里 url 与 b64_json 两种形态都收，b64 渲染成 data: URL。
// revised_prompt 是上游改写后的提示词，seed 只有部分供应商回，缺了就不画。
export function normalizeImageResult(payload) {
  const data = Array.isArray(payload?.data) ? payload.data : [];
  return data
    .map((item, index) => {
      const b64 = typeof item?.b64_json === 'string' ? item.b64_json.trim() : '';
      const url = typeof item?.url === 'string' ? item.url.trim() : '';
      const src = b64 ? `data:image/png;base64,${b64}` : url;
      if (!src) return null;
      return {
        src,
        fileName: `image-${index + 1}.png`,
        seed: item?.seed ?? null,
        revisedPrompt: typeof item?.revised_prompt === 'string' ? item.revised_prompt : ''
      };
    })
    .filter(Boolean);
}

// 对话出图回包：图片可能在 message.images[]（OpenRouter 风格），也可能是 content 数组里的
// image_url 块，或是文本里的 markdown 图片（data URL / http）。文字说明原样留给结果区。
const MARKDOWN_IMAGE = /!\[[^\]]*\]\(((?:data:image\/[^)\s]+)|(?:https?:\/\/[^)\s]+))\)/g;

const partUrl = (part) => {
  const url = typeof part?.image_url === 'string' ? part.image_url : part?.image_url?.url;
  return typeof url === 'string' ? url.trim() : '';
};

export function normalizeChatImageResult(payload) {
  const urls = [];
  const texts = [];
  const choices = Array.isArray(payload?.choices) ? payload.choices : [];
  for (const choice of choices) {
    const message = choice?.message || {};
    for (const part of Array.isArray(message.images) ? message.images : []) urls.push(partUrl(part));
    const content = message.content;
    const parts = Array.isArray(content) ? content : [{ type: 'text', text: content }];
    for (const part of parts) {
      if (part?.type === 'image_url') urls.push(partUrl(part));
      else if (typeof part?.text === 'string') {
        const text = part.text.replace(MARKDOWN_IMAGE, (_, url) => {
          urls.push(url);
          return '';
        });
        if (text.trim()) texts.push(text.trim());
      }
    }
  }
  const images = urls.filter(Boolean).map((src, index) => {
    const ext = (/^data:image\/(\w+)/.exec(src)?.[1] || 'png').replace('jpeg', 'jpg');
    return { src, fileName: `image-${index + 1}.${ext}`, seed: null, revisedPrompt: '' };
  });
  return { images, text: texts.join('\n\n') };
}

export const readAsDataUrl = (file) =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(file);
  });

// 参考图的前置校验：类型与体积注定 400 的上传不该先推上去。返回 i18n key，文案由工作区渲染。
export const MAX_REFERENCE_BYTES = 8 * 1024 * 1024;
export const REFERENCE_TYPES = ['image/png', 'image/jpeg', 'image/webp'];
export const REFERENCE_ACCEPT = REFERENCE_TYPES.join(',');

export function validateReferenceImage(file) {
  if (!file) return { ok: false, i18nKey: 'playgroundConsole.image.errors.missingFile' };
  if (!REFERENCE_TYPES.includes(String(file.type || '').toLowerCase())) {
    return { ok: false, i18nKey: 'playgroundConsole.image.errors.unsupportedType' };
  }
  if (Number(file.size) > MAX_REFERENCE_BYTES) return { ok: false, i18nKey: 'playgroundConsole.image.errors.tooLarge' };
  return { ok: true };
}

// 结果区顶部一行元信息：耗时 · 张数 · 尺寸 · 质量。缺的项不占位。
export function imageResultMeta({ images = [], elapsedMs = 0, values = {} }) {
  const facts = [];
  if (elapsedMs > 0) facts.push(`${(elapsedMs / 1000).toFixed(2)}s`);
  if (images.length > 0) facts.push(`${images.length}`);
  if (values.size) facts.push(values.size);
  if (values.quality) facts.push(values.quality);
  return facts;
}

const INITIAL = {
  prompt: DEFAULT_IMAGE_PROMPT,
  values: {},
  reference: null,
  fileError: null,
  resultTab: 'preview',
  status: 'idle',
  error: null,
  images: [],
  text: '',
  raw: '',
  elapsedMs: 0
};

const store = createSessionStore(INITIAL);

let controller = null;

async function imageRequest({ model, prompt, params, reference, chat }) {
  if (chat) {
    const imageUrl = reference ? await readAsDataUrl(reference) : undefined;
    return { json: chatImageBody({ model, prompt, imageUrl }) };
  }
  return reference
    ? { form: buildImageForm({ file: reference, model, prompt, params }) }
    : { json: buildImageBody({ model, prompt, params }) };
}

export async function runImage({ model, prompt, params, reference, chat = false }) {
  controller?.abort();
  controller = new AbortController();
  const startedAt = Date.now();
  store.setState({ status: 'running', error: null, images: [], text: '', raw: '', elapsedMs: 0 });

  try {
    const headers = await playgroundAuthHeaders();
    const request = await imageRequest({ model, prompt, params, reference, chat });
    const payload = await relaySend({ path: imagePath(reference, chat), headers, signal: controller.signal, ...request });
    const result = chat ? normalizeChatImageResult(payload) : { images: normalizeImageResult(payload), text: '' };
    store.setState({
      status: 'done',
      images: result.images,
      text: result.text,
      raw: JSON.stringify(payload, null, 2),
      elapsedMs: Date.now() - startedAt
    });
  } catch (err) {
    // 中止是用户主动停的，不算失败，回到空闲。
    if (err?.aborted) store.setState({ status: 'idle' });
    else store.setState({ status: 'error', error: err, elapsedMs: Date.now() - startedAt });
  }
}

export function abortImage() {
  controller?.abort();
  controller = null;
}

export default function useImageSession() {
  const state = useSessionState(store);
  const { baseUrl, setSelectedModel, loading, error: catalogError, reload } = useConsole();

  // 本模态认声明了 images 接口的模型，以及对话出图模型（chat 接口 + 输出模态含 image）。
  const { options, model } = useModalityModel(IMAGE_RULES);
  const chat = useMemo(() => isChatImageModel(model), [model]);

  const values = useMemo(() => resolveValues(IMAGE_FIELDS, model, state.values), [model, state.values]);
  const params = useMemo(() => fieldParams(IMAGE_FIELDS, model, values), [model, values]);
  const canUseReference = useMemo(() => supportsImageToImage(model), [model]);
  const reference = canUseReference ? state.reference : null;

  const snippets = useMemo(
    () =>
      buildSnippets({
        modality: chat ? 'image.chat' : reference ? 'image.edit' : 'image',
        baseUrl,
        model: model?.id || '',
        prompt: state.prompt,
        params,
        image: reference ? { name: reference.name } : undefined
      }),
    [baseUrl, model, state.prompt, params, reference, chat]
  );

  const setPrompt = useCallback((prompt) => store.setState({ prompt }), []);
  const setValue = useCallback((key, value) => store.setState((prev) => ({ values: { ...prev.values, [key]: value } })), []);
  const setReference = useCallback((file) => {
    if (!file) return store.setState({ reference: null, fileError: null });
    const checked = validateReferenceImage(file);
    return store.setState(checked.ok ? { reference: file, fileError: null } : { reference: null, fileError: checked.i18nKey });
  }, []);
  const setResultTab = useCallback((resultTab) => store.setState({ resultTab }), []);
  // 「清空」只清这一次运行的结果：提示词、参数与参考图留着，方便改一版再跑。
  const clear = useCallback(() => {
    abortImage();
    store.setState({ status: 'idle', error: null, images: [], text: '', raw: '', elapsedMs: 0 });
  }, []);
  const run = useCallback(
    () => runImage({ model: model?.id, prompt: state.prompt, params, reference, chat }),
    [model, state.prompt, params, reference, chat]
  );

  return {
    ...state,
    reference,
    canUseReference,
    chat,
    model,
    options,
    values,
    params,
    snippets,
    endpoint: imagePath(reference, chat),
    meta: imageResultMeta({ images: state.images, elapsedMs: state.elapsedMs, values: chat ? {} : values }),
    loading,
    catalogError,
    reload,
    setSelectedModel,
    setPrompt,
    setValue,
    setReference,
    setResultTab,
    clear,
    run,
    abort: abortImage
  };
}
