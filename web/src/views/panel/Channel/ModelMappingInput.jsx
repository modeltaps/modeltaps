import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Plus, Trash2, ArrowRight, HelpCircle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { ModelCombobox } from '@/components/ui/model-combobox';
import { Checkbox } from '@/components/ui/checkbox';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

// 行式编辑器，替代裸 JSON 文本框。底层仍序列化为 model_mapping 的 JSON 字符串
// {请求模型: 上游模型}，上游模型前缀 "+" 表示按传入模型计费（用复选框暴露，不再手写 +）。

// JSON 字符串 -> 行数组
function parseMapping(str) {
  if (!str || !str.trim()) return [];
  try {
    const obj = JSON.parse(str);
    if (!obj || typeof obj !== 'object' || Array.isArray(obj)) return [];
    return Object.entries(obj).map(([from, val]) => {
      const v = String(val ?? '');
      const bill = v.startsWith('+');
      return { from, to: bill ? v.slice(1) : v, bill };
    });
  } catch (e) {
    return [];
  }
}

// 行数组 -> JSON 字符串（丢弃空行；全空则空串，保持与旧数据一致）
function serializeMapping(rows) {
  const obj = {};
  rows.forEach(({ from, to, bill }) => {
    const k = (from || '').trim();
    const v = (to || '').trim();
    if (k && v) obj[k] = (bill ? '+' : '') + v;
  });
  return Object.keys(obj).length ? JSON.stringify(obj) : '';
}

export default function ModelMappingInput({ value, onChange, models = [] }) {
  const { t } = useTranslation();
  const [rows, setRows] = useState(() => parseMapping(value));

  // 候选来源（路线 A）：右边「上游真实模型」用渠道 models 列表；左边「请求模型/别名」
  // 再叠加已存在的映射 key。combobox 仍允许手敲列表外的值（上游有但未加进 models 时）。
  const toOptions = models;
  const fromOptions = useMemo(
    () => Array.from(new Set([...models, ...rows.map((r) => r.from).filter(Boolean)])),
    [models, rows]
  );
  const createLabel = (v) => t('channel_edit.mappingCreate', { value: v });

  // 大小写不敏感去重：仅 UI 提示。运行时仍精确匹配，但请求模型名若只差大小写
  // （如 GPT-4 与 gpt-4）几乎肯定是误配，标红提醒。返回重复的小写 key 集合。
  const dupKeys = useMemo(() => {
    const seen = new Map();
    rows.forEach((r) => {
      const k = (r.from || '').trim().toLowerCase();
      if (k) seen.set(k, (seen.get(k) || 0) + 1);
    });
    return new Set([...seen.entries()].filter(([, n]) => n > 1).map(([k]) => k));
  }, [rows]);
  const isDup = (from) => dupKeys.has((from || '').trim().toLowerCase());

  // 外部 value 变化（加载/重置）且与当前序列化不一致时，重新同步本地行
  useEffect(() => {
    if (serializeMapping(rows) !== (value || '')) {
      setRows(parseMapping(value));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  const emit = (next) => {
    setRows(next);
    onChange(serializeMapping(next));
  };
  const updateRow = (i, field, v) => emit(rows.map((r, idx) => (idx === i ? { ...r, [field]: v } : r)));
  const removeRow = (i) => emit(rows.filter((_, idx) => idx !== i));
  // 新增空行不产生 JSON 变化，故只更新本地状态、不 emit
  const addRow = () => setRows([...rows, { from: '', to: '', bill: false }]);

  return (
    <TooltipProvider delayDuration={150}>
      <div className="space-y-2">
        {rows.map((row, index) => (
          <div key={index} className="flex items-center gap-2">
            <ModelCombobox
              className={`flex-1${isDup(row.from) ? ' [&>input]:border-destructive [&>input]:focus:ring-destructive' : ''}`}
              value={row.from}
              options={fromOptions}
              createLabel={createLabel}
              placeholder={t('channel_edit.mappingFrom')}
              onChange={(v) => updateRow(index, 'from', v)}
            />
            <ArrowRight className="size-4 shrink-0 text-muted-foreground" />
            <ModelCombobox
              className="flex-1"
              value={row.to}
              options={toOptions}
              createLabel={createLabel}
              placeholder={t('channel_edit.mappingTo')}
              onChange={(v) => updateRow(index, 'to', v)}
            />
            <div className="flex shrink-0 items-center gap-1 text-xs text-muted-foreground">
              <label className="flex items-center gap-1.5">
                <Checkbox checked={row.bill} onCheckedChange={(c) => updateRow(index, 'bill', !!c)} />
                {t('channel_edit.mappingBillIncoming')}
              </label>
              <Tooltip>
                <TooltipTrigger asChild>
                  <button type="button" tabIndex={-1} className="text-muted-foreground hover:text-foreground" aria-label="help">
                    <HelpCircle className="size-3.5" />
                  </button>
                </TooltipTrigger>
                <TooltipContent className="max-w-xs whitespace-normal text-xs">
                  {t('channel_edit.mappingBillIncomingTip')}
                </TooltipContent>
              </Tooltip>
            </div>
            <Button type="button" variant="ghost" size="icon" className="shrink-0" onClick={() => removeRow(index)}>
              <Trash2 className="size-4" />
            </Button>
          </div>
        ))}
        {dupKeys.size > 0 && (
          <p className="text-xs text-destructive">
            {t('channel_edit.mappingDupKey')}
          </p>
        )}
        <Button type="button" variant="outline" size="sm" onClick={addRow}>
          <Plus className="size-4" />
          {t('channel_edit.add')}
        </Button>
      </div>
    </TooltipProvider>
  );
}

ModelMappingInput.propTypes = {
  value: PropTypes.string,
  onChange: PropTypes.func,
  models: PropTypes.arrayOf(PropTypes.string)
};
