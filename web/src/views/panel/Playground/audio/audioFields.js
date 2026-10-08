// ==============================|| PLAYGROUND — AUDIO PARAM SUMMARIES ||============================== //
// 语音页编辑卡上那两颗 chip 的摘要文案与示例清单。字段本身仍出自 shared/fieldSchema
// （TTS_FIELDS / STT_FIELDS），这里只把已求值的 values 折成一行短摘要。
// 音色单独一颗 chip（弹出音色库），所以参数 chip 的 schema 把 voice 去掉。

import { STT_FIELDS, TTS_FIELDS } from '../shared/fieldSchema';

// 示例 chip：只登记 id，标题与正文全部走 i18n。
export const SPEECH_EXAMPLES = ['support', 'sales', 'podcast', 'announcement', 'meditation'];

export const TTS_PARAM_FIELDS = TTS_FIELDS.filter((field) => field.key !== 'voice');

export const STT_PARAM_FIELDS = STT_FIELDS;

// 合成参数摘要：「1.0x · MP3」。语速缺省按上游默认的 1 显示。
export function speechSummary(values = {}) {
  const speed = Number(values.speed);
  const format = String(values.response_format || 'mp3');
  return `${(Number.isFinite(speed) ? speed : 1).toFixed(1)}x · ${format.toUpperCase()}`;
}

// 转写参数摘要：填了语言就带上，否则只报响应格式。
export function transcribeSummary(values = {}) {
  const language = String(values.language || '').trim();
  const format = String(values.response_format || 'json');
  return [language, format.toUpperCase()].filter(Boolean).join(' · ');
}

// 播放条时间：mm:ss，拿不到时长（还没有结果 / 元数据未就绪）时给 --:--。
export function clock(seconds) {
  const value = Number(seconds);
  if (!Number.isFinite(value) || value < 0) return '--:--';
  const total = Math.floor(value);
  return `${String(Math.floor(total / 60)).padStart(2, '0')}:${String(total % 60).padStart(2, '0')}`;
}
