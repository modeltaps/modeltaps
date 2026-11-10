import { useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Textarea } from '@/components/ui/textarea';
import ModelMultiSelect from '@/components/model/ModelMultiSelect';
import { mergeOptions } from '@/components/model/modelSelectIndex';

// Channel "models" field: the shared ModelMultiSelect dropdown (search, vendor groups, newest
// first, checkboxes) with an "edit as text" escape hatch for pasting long lists. The value stays
// a comma-separated string, so the backend is unchanged; ids are deduplicated case-insensitively.
//
// Options come from `upstream` (rows of the channel's upstream list, with release dates, loaded
// by the caller through `onOpen`) and `suggestions` (the type's default models and its
// price-table models), upstream first so its metadata wins.
const split = (str) =>
  (str || '')
    .split(/[,\n]/)
    .map((m) => m.trim())
    .filter(Boolean);
const join = (arr) => {
  const seen = new Set();
  return arr
    .filter((m) => {
      const k = m.toLowerCase();
      if (!m || seen.has(k)) return false;
      seen.add(k);
      return true;
    })
    .join(',');
};

export default function ModelListInput({
  value,
  onChange,
  disabled,
  headerActions,
  suggestions = [],
  upstream = [],
  onOpen,
  loading,
  notice,
  advanced
}) {
  const { t } = useTranslation();
  const [mode, setMode] = useState('select'); // 'select' | 'text'
  const models = useMemo(() => split(value), [value]);
  const options = useMemo(() => mergeOptions(upstream, suggestions), [upstream, suggestions]);

  const toolbar = (
    <div className="flex flex-wrap items-center justify-between gap-2">
      <div className="flex flex-wrap items-center gap-2">{headerActions}</div>
      <button
        type="button"
        onClick={() => setMode(mode === 'text' ? 'select' : 'text')}
        className="text-xs text-muted-foreground transition-colors hover:text-foreground"
      >
        {mode === 'text' ? t('channel_edit.modelChipMode') : t('channel_edit.modelTextMode')}
      </button>
    </div>
  );

  if (mode === 'text') {
    return (
      <div className="space-y-2">
        <Textarea
          rows={6}
          className="font-mono text-xs"
          placeholder="gpt-5.5, claude-sonnet-5-5"
          value={value || ''}
          onChange={(e) => onChange(e.target.value)}
          disabled={disabled}
        />
        {toolbar}
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <ModelMultiSelect
        id="channel-models"
        value={models}
        onChange={(ids) => onChange(join(ids))}
        options={options}
        onOpen={onOpen}
        loading={loading}
        notice={notice}
        advanced={advanced}
        disabled={disabled}
      />
      {toolbar}
    </div>
  );
}

ModelListInput.propTypes = {
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired,
  disabled: PropTypes.bool,
  headerActions: PropTypes.node,
  suggestions: PropTypes.arrayOf(PropTypes.string),
  upstream: PropTypes.arrayOf(PropTypes.object),
  onOpen: PropTypes.func,
  loading: PropTypes.bool,
  notice: PropTypes.node,
  advanced: PropTypes.object
};
