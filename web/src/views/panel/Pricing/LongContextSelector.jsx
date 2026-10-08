import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

// 长上下文分档计价编辑(对应 model.LongContextTier):输入 token 超过 threshold 时,
// 整次请求按 input_ratio/output_ratio 计费。threshold 留空或 0 即未启用(后端同语义,
// 见 GetLongContextMultiplier)。上游 LongContextSelector(MUI)的 shadcn 翻译版。
const FIELDS = [
  { key: 'threshold', labelKey: 'pricing_edit.longContextThreshold', step: '1', integer: true },
  { key: 'input_ratio', labelKey: 'pricing_edit.longContextInputRatio', step: '0.01' },
  { key: 'output_ratio', labelKey: 'pricing_edit.longContextOutputRatio', step: '0.01' }
];

export default function LongContextSelector({ value = {}, onChange }) {
  const { t } = useTranslation();

  const handleChangeField = (field, raw) => {
    const parsed = field.integer ? parseInt(raw, 10) : parseFloat(raw);
    onChange({ ...value, [field.key]: Number.isNaN(parsed) ? 0 : parsed });
  };

  return (
    <div className="space-y-3 rounded-md border border-border p-3">
      <p className="text-sm font-medium">{t('pricing_edit.longContext')}</p>
      <div className="flex flex-col gap-2 sm:flex-row">
        {FIELDS.map((field) => (
          <div key={field.key} className="flex-1 space-y-1.5">
            <Label className="text-xs text-muted-foreground">{t(field.labelKey)}</Label>
            <Input
              type="number"
              step={field.step}
              min="0"
              value={value?.[field.key] ?? ''}
              onChange={(e) => handleChangeField(field, e.target.value)}
            />
          </div>
        ))}
      </div>
      <p className="text-xs text-muted-foreground">{t('pricing_edit.longContextHelp')}</p>
    </div>
  );
}

LongContextSelector.propTypes = {
  value: PropTypes.object,
  onChange: PropTypes.func.isRequired
};
