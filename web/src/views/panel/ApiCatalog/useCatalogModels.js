import { useCallback, useEffect, useState } from 'react';

import { API } from 'utils/api';
import { compareCatalogModels, resolveVendorSlug } from './availability';

// ==============================|| API CATALOG — MODELS ||============================== //
// 目录页统一取数：/api/available_model 已只返回「已发布 + 可路由」的主名，
// 因此「站点是否提供某能力」完全由这份数据决定（endpoints + price.model_info）。

// 后端把 model_info 挂在 price 下（ModelInfoResponse，数组字段已解析好）。
const normalizeInfo = (info) => ({
  inputModalities: info?.input_modalities || [],
  outputModalities: info?.output_modalities || [],
  capabilities: info?.capabilities || [],
  contextLength: info?.context_length || 0,
  maxTokens: info?.max_tokens || 0,
  description: info?.description || '',
  name: info?.name || ''
});

const normalize = (id, raw) => ({
  id,
  ownedBy: raw?.owned_by || '',
  // 厂商统一存 slug：后端的 vendor 可能是对象（{ slug, name }）、字符串或干脆没有，
  // 可用性规则只比 slug，这里就地归一，避免下游拿对象去比较。
  vendor: resolveVendorSlug(raw),
  endpoints: raw?.endpoints || [],
  aliases: raw?.aliases || [],
  groups: raw?.groups || [],
  price: raw?.price || null,
  info: normalizeInfo(raw?.price?.model_info),
  // 文字转语音模型的音色表（{ id, language, gender }），非 TTS 模型为空数组。
  ttsVoices: Array.isArray(raw?.tts_voices) ? raw.tts_voices : [],
  // 文字转语音模型可用的输出格式（response_format），后端没给时为空数组，由消费方回落完整列表。
  ttsFormats: Array.isArray(raw?.tts_formats) ? raw.tts_formats : []
});

export default function useCatalogModels() {
  const [models, setModels] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const fetchModels = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const res = await API.get('/api/available_model');
      const { success, data } = res.data;
      if (!success || !data) throw new Error('available_model request failed');
      setModels(
        Object.entries(data)
          .map(([id, raw]) => normalize(id, raw))
          .sort(compareCatalogModels)
      );
    } catch (err) {
      // 取数失败必须显式暴露：这份数据就是「站点支持什么」的全部依据，
      // 悄悄渲染成「暂无可用模型」等于给用户一个错误答案。
      console.error(err);
      setModels([]);
      setError(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchModels();
  }, [fetchModels]);

  return { models, loading, error, reload: fetchModels };
}
