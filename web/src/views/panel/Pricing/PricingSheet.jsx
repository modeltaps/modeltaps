import { useEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useForm, Controller } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { Loader2, X } from 'lucide-react';

import { API } from 'utils/api';
import { trims } from 'utils/common';
import { toast } from '@/components/ui/sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetBody, SheetFooter } from '@/components/ui/sheet';
import ExtraRatiosSelector from './ExtraRatiosSelector';
import LongContextSelector from './LongContextSelector';
import { valueToRate, convertUnit, getPriceTypeOptions, formatRatePreview } from './pricingUtils';

const EMPTY = {
  model: '',
  type: 'tokens',
  channel_type: 1,
  input: 0,
  output: 0,
  locked: false,
  models: [],
  extra_ratios: {},
  long_context: {}
};

// 价格节表单的初值:已有价格按其回填,无价格为空表单(输入 / 输出留空)。
export const priceFormValues = (price) =>
  price
    ? { ...EMPTY, ...price, models: price.models || [], extra_ratios: price.extra_ratios || {}, long_context: price.long_context || {} }
    : { ...EMPTY, input: '', output: '' };

// threshold <= 0 视为未启用分档,提交时省略字段(后端 LongContext 为可空 JSON,同语义)。
const normalizeLongContext = (lc) => (lc && Number(lc.threshold) > 0 ? lc : undefined);

// 表单值 → 价格载荷(倍率);校验不通过返回 { error }(i18n key)。
export function buildPricePayload(values, unitType, localUnit) {
  const inputRate = valueToRate(values.input, unitType, localUnit, values.type);
  const outputRate = values.type === 'times' ? inputRate : valueToRate(values.output, unitType, localUnit, values.type);
  if (values.channel_type < 1) return { error: 'pricing_edit.channelTypeErr' };
  if (inputRate === '' || inputRate < 0) return { error: 'pricing_edit.inputVal' };
  if (values.type === 'tokens' && (outputRate === '' || outputRate < 0)) return { error: 'pricing_edit.outputVal' };
  return {
    price: {
      type: values.type,
      channel_type: values.channel_type,
      input: inputRate,
      output: outputRate,
      locked: values.locked,
      extra_ratios: values.extra_ratios,
      long_context: normalizeLongContext(values.long_context)
    }
  };
}

// 价格输入单位(倍率 / USD / RMB × K / M):打开时新建默认 USD/M,编辑默认倍率 + 外部单位;切换时换算已填数值。
export function usePriceUnits({ open, isEdit, unit, getValues, setValue }) {
  const [unitType, setUnitType] = useState('rate');
  const [localUnit, setLocalUnit] = useState(unit);

  const initRef = useRef({ isEdit, unit });
  initRef.current = { isEdit, unit };
  useEffect(() => {
    if (!open) return;
    const { isEdit: e, unit: u } = initRef.current;
    if (e) {
      setUnitType('rate');
      setLocalUnit(u);
    } else {
      setUnitType('USD');
      setLocalUnit('M');
    }
  }, [open]);

  const reconvert = (fromType, fromUnit, toType, toUnit) => {
    if (fromType === toType && fromUnit === toUnit) return;
    const cur = getValues();
    if (cur.input !== '') setValue('input', convertUnit(cur.input ?? 0, fromType, fromUnit, toType, toUnit, cur.type));
    if (cur.output !== '') setValue('output', convertUnit(cur.output ?? 0, fromType, fromUnit, toType, toUnit, cur.type));
  };

  const onUnitTypeChange = (next) => {
    if (!next || next === unitType) return;
    reconvert(unitType, localUnit, next, localUnit);
    setUnitType(next);
  };

  const onLocalUnitChange = (next) => {
    if (!next || next === localUnit) return;
    if (unitType !== 'rate') reconvert(unitType, localUnit, unitType, next);
    setLocalUnit(next);
  };

  return { unitType, localUnit, onUnitTypeChange, onLocalUnitChange };
}

function UnitToggle({ value, options, onChange }) {
  return (
    <div className="inline-flex overflow-hidden rounded-md border border-border">
      {options.map((o, i) => (
        <button
          key={String(o.value)}
          type="button"
          onClick={() => onChange(o.value)}
          className={`px-3 py-1.5 text-sm font-medium ${i > 0 ? 'border-l border-border' : ''} ${
            value === o.value ? 'bg-muted text-foreground' : 'text-muted-foreground hover:bg-muted'
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

UnitToggle.propTypes = { value: PropTypes.any, options: PropTypes.array, onChange: PropTypes.func };

// 价格字段(类型、渠道类型、单位、输入 / 输出、锁定、额外倍率、长上下文);children 插在价格输入与锁定之间。
export function PriceFields({ control, type, ownedby = [], units, children }) {
  const { t } = useTranslation();
  const { unitType, localUnit, onUnitTypeChange, onLocalUnitChange } = units;

  const unitTypeOptions = [
    { value: 'rate', label: t('modelpricePage.rate') },
    { value: 'USD', label: 'USD' },
    { value: 'RMB', label: 'RMB' }
  ];
  const kmOptions = [
    { value: 'K', label: 'K' },
    { value: 'M', label: 'M' }
  ];

  return (
    <>
      <FormField id="pricing-type" label={t('pricing_edit.type')}>
        <Controller
          control={control}
          name="type"
          render={({ field }) => (
            <Select value={field.value} onValueChange={field.onChange}>
              <SelectTrigger id="pricing-type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {getPriceTypeOptions(t).map((o) => (
                  <SelectItem key={o.value} value={o.value}>
                    {o.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
      </FormField>

      <FormField id="pricing-channel-type" label={t('pricing_edit.channelType')} required>
        <Controller
          control={control}
          name="channel_type"
          render={({ field }) => (
            <Select value={String(field.value)} onValueChange={(v) => field.onChange(Number(v))}>
              <SelectTrigger id="pricing-channel-type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ownedby.map((o) => (
                  <SelectItem key={o.value} value={String(o.value)}>
                    {o.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
      </FormField>

      <div className="flex flex-wrap items-center gap-3">
        <UnitToggle value={unitType} options={unitTypeOptions} onChange={onUnitTypeChange} />
        {type !== 'times' && <UnitToggle value={localUnit} options={kmOptions} onChange={onLocalUnitChange} />}
      </div>

      <div className="flex flex-col gap-3 sm:flex-row">
        <FormField
          className="flex-1"
          label={type === 'times' ? t('modelpricePage.timesPrice') : t('modelpricePage.inputMultiplier')}
          required
        >
          <Controller
            control={control}
            name="input"
            render={({ field }) => (
              <div className="relative">
                <Input
                  type="number"
                  value={field.value}
                  onChange={(e) => field.onChange(e.target.value === '' ? '' : Number(e.target.value))}
                />
                <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">
                  {formatRatePreview(field.value, unitType, localUnit, type)}
                </span>
              </div>
            )}
          />
        </FormField>
        {type === 'tokens' && (
          <FormField className="flex-1" label={t('modelpricePage.outputMultiplier')} required>
            <Controller
              control={control}
              name="output"
              render={({ field }) => (
                <div className="relative">
                  <Input
                    type="number"
                    value={field.value}
                    onChange={(e) => field.onChange(e.target.value === '' ? '' : Number(e.target.value))}
                  />
                  <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">
                    {formatRatePreview(field.value, unitType, localUnit, type)}
                  </span>
                </div>
              )}
            />
          </FormField>
        )}
      </div>

      {children}

      <FormField row label={t('pricing_edit.locked_title')} help={t('pricing_edit.lockedTip')} controlClassName="w-auto">
        <Controller
          control={control}
          name="locked"
          render={({ field }) => <Switch checked={!!field.value} onCheckedChange={field.onChange} />}
        />
      </FormField>

      <Controller
        control={control}
        name="extra_ratios"
        render={({ field }) => <ExtraRatiosSelector value={field.value || {}} onChange={field.onChange} />}
      />

      {type === 'tokens' && (
        <Controller
          control={control}
          name="long_context"
          render={({ field }) => <LongContextSelector value={field.value || {}} onChange={field.onChange} />}
        />
      )}
    </>
  );
}

PriceFields.propTypes = {
  control: PropTypes.object.isRequired,
  type: PropTypes.string,
  ownedby: PropTypes.array,
  units: PropTypes.object.isRequired,
  children: PropTypes.node
};

// 批量定价:选中模型统一设价;original_models 为其中已有价格的模型(走更新),其余新增。
export default function PricingSheet({ open, onClose, onSaved, ownedby = [], priceItem = null, modelOptions = [], unit = 'K' }) {
  const { t } = useTranslation();
  const [submitting, setSubmitting] = useState(false);
  const [modelInput, setModelInput] = useState('');
  const { handleSubmit, control, reset, watch, setValue, getValues } = useForm({ defaultValues: EMPTY });

  const type = watch('type');
  const models = watch('models');
  const units = usePriceUnits({ open, isEdit: !!priceItem, unit, getValues, setValue });

  useEffect(() => {
    if (!open) return;
    reset(priceItem ? priceFormValues(priceItem) : EMPTY);
    setModelInput('');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, priceItem]);

  const addModel = (name) => {
    const v = (name || '').trim();
    if (!v) return;
    const current = getValues('models') || [];
    if (!current.includes(v)) setValue('models', [...current, v]);
    setModelInput('');
  };

  const removeModel = (name) => {
    setValue(
      'models',
      (getValues('models') || []).filter((m) => m !== name)
    );
  };

  const onSubmit = async (values) => {
    const modelList = trims(values.models || []);
    const { error, price } = buildPricePayload(values, units.unitType, units.localUnit);
    if (error) return toast.error(t(error));
    if (!modelList.length) return toast.error(t('pricing_edit.requiredModels'));
    setSubmitting(true);
    try {
      const res = await API.post('/api/prices/multiple', {
        original_models: priceItem?.original_models ?? (priceItem?.models || []),
        models: modelList,
        price: { model: 'batch', ...price }
      });
      const { success, message } = res.data;
      if (success) {
        toast.success(t('common.saveSuccess', { defaultValue: 'Saved' }));
        onSaved();
      } else toast.error(message);
    } catch (error) {
      toast.error(error.message);
    }
    setSubmitting(false);
  };

  return (
    <Sheet open={open} onOpenChange={(v) => !v && onClose()}>
      <SheetContent onClose={onClose}>
        <SheetHeader>
          <SheetTitle>{priceItem ? t('common.edit') : t('common.create')}</SheetTitle>
        </SheetHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="flex min-h-0 flex-1 flex-col">
          <SheetBody className="space-y-4">
            <PriceFields control={control} type={type} ownedby={ownedby} units={units}>
              <FormField label={t('pricing_edit.model')} required help={t('pricing_edit.modelTip')}>
                <div className="space-y-2">
                  {(models || []).length > 0 && (
                    <div className="flex flex-wrap gap-1.5">
                      {(models || []).map((m) => (
                        <Badge key={m} variant="secondary" className="gap-1">
                          {m}
                          <button type="button" onClick={() => removeModel(m)}>
                            <X className="size-3" />
                          </button>
                        </Badge>
                      ))}
                    </div>
                  )}
                  <Input
                    list="pricing-model-options"
                    value={modelInput}
                    onChange={(e) => setModelInput(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ',') {
                        e.preventDefault();
                        addModel(modelInput);
                      }
                    }}
                    onBlur={() => addModel(modelInput)}
                  />
                  <datalist id="pricing-model-options">
                    {modelOptions
                      .filter((m) => !(models || []).includes(m))
                      .map((m) => (
                        <option key={m} value={m} />
                      ))}
                  </datalist>
                </div>
              </FormField>
            </PriceFields>
          </SheetBody>
          <SheetFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting && <Loader2 className="size-4 animate-spin" />}
              {t('common.submit')}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}

PricingSheet.propTypes = {
  open: PropTypes.bool,
  onClose: PropTypes.func,
  onSaved: PropTypes.func,
  ownedby: PropTypes.array,
  priceItem: PropTypes.object,
  modelOptions: PropTypes.array,
  unit: PropTypes.string
};
