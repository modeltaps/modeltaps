import PropTypes from 'prop-types';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Plus, Trash2, ArrowDownToLine, ArrowUpFromLine } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { extraRatiosConfig, getReadableRatioName } from './pricingUtils';

export default function ExtraRatiosSelector({ value = {}, onChange }) {
  const { t } = useTranslation();
  const [selectedRatio, setSelectedRatio] = useState('');

  const availableRatios = useMemo(() => extraRatiosConfig.filter((c) => value[c.key] === undefined), [value]);
  const addedRatios = useMemo(() => extraRatiosConfig.filter((c) => value[c.key] !== undefined), [value]);

  const handleAddRatio = () => {
    if (selectedRatio && value[selectedRatio] === undefined) {
      onChange({ ...value, [selectedRatio]: 0 });
      setSelectedRatio('');
    }
  };

  const handleRemoveRatio = (key) => {
    const next = { ...value };
    delete next[key];
    onChange(next);
  };

  const handleChangeRatioValue = (key, newValue) => {
    onChange({ ...value, [key]: Number(newValue) || 0 });
  };

  return (
    <div className="space-y-3 rounded-md border border-border p-3">
      <p className="text-sm font-medium">{t('pricing_edit.extraRatios')}</p>

      <div className="flex flex-col gap-2 sm:flex-row">
        <Select value={selectedRatio} onValueChange={setSelectedRatio} disabled={availableRatios.length === 0}>
          <SelectTrigger className="flex-1">
            <SelectValue placeholder={t('pricing_edit.selectExtraRatio')} />
          </SelectTrigger>
          <SelectContent>
            {availableRatios.length === 0 ? (
              <SelectItem value="__none" disabled>
                {t('pricing_edit.noAvailableRatios')}
              </SelectItem>
            ) : (
              availableRatios.map((option) => (
                <SelectItem key={option.key} value={option.key}>
                  <span className="flex items-center gap-2">
                    {option.isPrompt ? <ArrowDownToLine className="size-3.5" /> : <ArrowUpFromLine className="size-3.5" />}
                    {getReadableRatioName(option.key, t)}
                  </span>
                </SelectItem>
              ))
            )}
          </SelectContent>
        </Select>
        <Button type="button" variant="outline" size="icon" onClick={handleAddRatio} disabled={!selectedRatio}>
          <Plus className="size-4" />
        </Button>
      </div>

      {addedRatios.length > 0 ? (
        <div className="space-y-2">
          {addedRatios.map((config) => (
            <div key={config.key} className="flex items-center gap-2 rounded-md border border-border p-2">
              {config.isPrompt ? (
                <ArrowDownToLine className="size-4 text-muted-foreground" />
              ) : (
                <ArrowUpFromLine className="size-4 text-emerald-500" />
              )}
              <span className="flex-1 text-sm">{getReadableRatioName(config.key, t)}</span>
              <Input
                type="number"
                step="0.01"
                min="0"
                className="h-8 w-32"
                value={value[config.key]}
                onChange={(e) => handleChangeRatioValue(config.key, parseFloat(e.target.value))}
              />
              <Button type="button" variant="ghost" size="icon" className="size-8 text-destructive" onClick={() => handleRemoveRatio(config.key)}>
                <Trash2 className="size-4" />
              </Button>
            </div>
          ))}
        </div>
      ) : (
        <p className="rounded-md border border-dashed border-border p-3 text-center text-sm text-muted-foreground">
          {t('pricing_edit.noExtraRatios')}
        </p>
      )}
    </div>
  );
}

ExtraRatiosSelector.propTypes = {
  value: PropTypes.object,
  onChange: PropTypes.func.isRequired
};
