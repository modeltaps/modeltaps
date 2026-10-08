import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Plus, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

export default function ModelHeadersInput({ value, onChange }) {
  const { t } = useTranslation();
  const rows = Array.isArray(value) ? value : [];

  const updateRow = (index, field, v) => {
    onChange(rows.map((row, i) => (i === index ? { ...row, [field]: v } : row)));
  };

  const addRow = () => onChange([...rows, { key: '', value: '' }]);
  const removeRow = (index) => onChange(rows.filter((_, i) => i !== index));

  return (
    <div className="space-y-2">
      {rows.map((row, index) => (
        <div key={index} className="flex items-center gap-2">
          <Input value={row.key ?? ''} placeholder="Header" onChange={(e) => updateRow(index, 'key', e.target.value)} />
          <Input value={row.value ?? ''} placeholder="Value" onChange={(e) => updateRow(index, 'value', e.target.value)} />
          <Button type="button" variant="ghost" size="icon" className="shrink-0" onClick={() => removeRow(index)}>
            <Trash2 className="size-4" />
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" size="sm" onClick={addRow}>
        <Plus className="size-4" />
        {t('channel_edit.add')}
      </Button>
    </div>
  );
}

ModelHeadersInput.propTypes = {
  value: PropTypes.array,
  onChange: PropTypes.func
};
