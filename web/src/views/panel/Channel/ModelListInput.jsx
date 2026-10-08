import { useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Search, X, Plus } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';

// 渠道「模型」列表输入：替代裸逗号文本框。默认 chip 视图（可搜索、可逐个删除、
// 输入回车添加），保留「编辑为文本」逃生口用于批量粘贴。底层仍序列化成逗号串，
// 后端零改动。大小写不敏感去重（仅录入层）。
const split = (str) =>
  (str || '')
    .split(/[,\n]/)
    .map((m) => m.trim())
    .filter(Boolean);
const join = (arr) => Array.from(new Set(arr.filter(Boolean))).join(',');

export default function ModelListInput({ value, onChange, disabled, headerActions }) {
  const { t } = useTranslation();
  const [mode, setMode] = useState('chips'); // 'chips' | 'text'
  const [query, setQuery] = useState('');

  const models = useMemo(() => split(value), [value]);
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return q ? models.filter((m) => m.toLowerCase().includes(q)) : models;
  }, [models, query]);

  const remove = (m) => onChange(join(models.filter((x) => x !== m)));
  const add = (raw) => {
    const v = raw.trim();
    if (!v || models.some((x) => x.toLowerCase() === v.toLowerCase())) return;
    onChange(join([...models, v]));
    setQuery('');
  };
  const canAdd = query.trim() !== '' && !models.some((x) => x.toLowerCase() === query.trim().toLowerCase());

  if (mode === 'text') {
    return (
      <div className="space-y-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs text-muted-foreground">
              {t('channel_edit.modelCount', { count: models.length })}
            </span>
            {headerActions}
          </div>
          <button type="button" onClick={() => setMode('chips')} className="text-xs text-primary-text hover:underline">
            {t('channel_edit.modelChipMode')}
          </button>
        </div>
        <Textarea
          rows={4}
          placeholder="gpt-3.5-turbo, gpt-4"
          value={value || ''}
          onChange={(e) => onChange(e.target.value)}
          disabled={disabled}
        />
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="h-9 pl-8"
            placeholder={t('channel_edit.modelSearchAdd')}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && canAdd) {
                e.preventDefault();
                add(query);
              }
            }}
            disabled={disabled}
          />
        </div>
        {canAdd && (
          <button
            type="button"
            onClick={() => add(query)}
            className="inline-flex h-9 shrink-0 items-center gap-1 rounded-md border border-input px-2.5 text-sm hover:bg-muted"
          >
            <Plus className="size-4" />
            {t('channel_edit.modelAdd')}
          </button>
        )}
        <button
          type="button"
          onClick={() => setMode('text')}
          className="shrink-0 whitespace-nowrap text-xs text-muted-foreground hover:text-foreground"
        >
          {t('channel_edit.modelTextMode')}
        </button>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">
          {query.trim()
            ? t('channel_edit.modelCountFiltered', {
                shown: filtered.length,
                total: models.length
              })
            : t('channel_edit.modelCount', { count: models.length })}
        </p>
        {headerActions && <div className="flex flex-wrap items-center gap-2">{headerActions}</div>}
      </div>

      <div className="max-h-48 overflow-y-auto rounded-md border border-input bg-background p-2">
        {filtered.length === 0 ? (
          <p className="py-4 text-center text-xs text-muted-foreground">
            {models.length === 0
              ? t('channel_edit.modelEmpty')
              : t('channel_edit.modelNoMatch')}
          </p>
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {filtered.map((m) => (
              <span key={m} className="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-1 text-xs">
                <span className="max-w-[240px] truncate" title={m}>
                  {m}
                </span>
                {!disabled && (
                  <button
                    type="button"
                    onClick={() => remove(m)}
                    className="text-muted-foreground hover:text-destructive"
                    aria-label={`remove ${m}`}
                  >
                    <X className="size-3" />
                  </button>
                )}
              </span>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

ModelListInput.propTypes = {
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired,
  disabled: PropTypes.bool,
  headerActions: PropTypes.node
};
