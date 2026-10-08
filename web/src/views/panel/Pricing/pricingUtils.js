import Decimal from 'decimal.js';
import { ValueFormatter } from 'utils/common';
import { safeJsonArray } from '../ModelInfo/modelInfoHelpers';

// rate is the backend storage unit; USD/RMB x K/M is converted through rate.
// Base: 1 rate = $0.002 / 1K tokens (also 1 rate = ￥0.014 / 1K, implied FX 7).
// priceType='times' is per-call billing, K/M is meaningless and ignored.
export const valueToRate = (value, unitType, localUnit, priceType) => {
  if (value === '' || value == null) return '';
  const v = new Decimal(value);
  if (unitType === 'rate') return Number(v.toFixed(4));
  let r = v;
  if (priceType !== 'times' && localUnit === 'M') r = r.div(1000);
  if (unitType === 'USD') r = r.div(0.002);
  if (unitType === 'RMB') r = r.div(0.014);
  return Number(r.toFixed(4));
};

export const rateToValue = (rate, unitType, localUnit, priceType) => {
  if (rate === '' || rate == null) return '';
  const r = new Decimal(rate);
  if (unitType === 'rate') return Number(r.toFixed(4));
  let v = r;
  if (unitType === 'USD') v = v.mul(0.002);
  if (unitType === 'RMB') v = v.mul(0.014);
  if (priceType !== 'times' && localUnit === 'M') v = v.mul(1000);
  return Number(v.toFixed(6));
};

export const convertUnit = (value, fromType, fromUnit, toType, toUnit, priceType) => {
  if (fromType === toType && fromUnit === toUnit) return value;
  const rate = valueToRate(value, fromType, fromUnit, priceType);
  return rateToValue(rate, toType, toUnit, priceType);
};

export const getPriceTypeOptions = (t) => [
  { value: 'tokens', label: t('modelpricePage.tokens') },
  { value: 'times', label: t('modelpricePage.times') }
];

// Format a stored rate value for display in the table (USD only, with unit suffix).
export const formatPrice = (value, type, unit) => {
  if (value === 0) return 'Free';
  let isM = unit === 'M';
  let unitLabel = '';
  if (type === 'tokens') {
    unitLabel = ` / 1${unit}`;
  } else {
    isM = false;
  }
  return ValueFormatter(value, true, isM) + unitLabel;
};

// End-adornment helper for the editor price inputs.
export const formatRatePreview = (value, unitType, localUnit, priceType) => {
  if (value === '' || value == null) return '';
  if (unitType === 'rate') return ValueFormatter(value);
  if (value === 0) return 'Free';
  return `${valueToRate(value, unitType, localUnit, priceType)} Rate`;
};

// 行（单模型 / 多模型分组）对应的模态并集。dir = 'input' | 'output'。
// 元信息来自 /api/model_info/，模态字段为 JSON 字符串；缺元信息的模型按无模态处理。
export const rowModalities = (row, modelInfoMap = {}, dir = 'input') => {
  const field = dir === 'output' ? 'output_modalities' : 'input_modalities';
  const models = row?.models || (row?.model ? [row.model] : []);
  const out = [];
  models.forEach((m) => {
    safeJsonArray(modelInfoMap[m]?.[field]).forEach((v) => {
      if (!out.includes(v)) out.push(v);
    });
  });
  return out;
};

export const extraRatiosConfig = [
  { name: 'cached_tokens', key: 'cached_tokens', isPrompt: true },
  { name: 'cached_write_tokens', key: 'cached_write_tokens', isPrompt: true },
  { name: 'cached_write_1h_tokens', key: 'cached_write_1h_tokens', isPrompt: true },
  { name: 'cached_read_tokens', key: 'cached_read_tokens', isPrompt: true },
  { name: 'openai_cache_write_tokens', key: 'openai_cache_write_tokens', isPrompt: true },
  { name: 'input_audio_tokens', key: 'input_audio_tokens', isPrompt: true },
  { name: 'output_audio_tokens', key: 'output_audio_tokens', isPrompt: false },
  { name: 'reasoning_tokens', key: 'reasoning_tokens', isPrompt: false },
  { name: 'input_text_tokens', key: 'input_text_tokens', isPrompt: true },
  { name: 'output_text_tokens', key: 'output_text_tokens', isPrompt: false },
  { name: 'input_image_tokens', key: 'input_image_tokens', isPrompt: true },
  { name: 'output_image_tokens', key: 'output_image_tokens', isPrompt: false }
];

const RATIO_I18N_KEYS = extraRatiosConfig.map((c) => c.key);

export const getReadableRatioName = (key, t) => {
  if (RATIO_I18N_KEYS.includes(key)) {
    return t(`modelpricePage.${key}`);
  }
  return key;
};
