import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { fieldOptionLabelKey, fieldOptions, visibleFields } from '../shared/fieldSchema';

// ==============================|| PLAYGROUND — FIELD RENDERER ||============================== //
// 按 shared/fieldSchema 的字段描述渲染运行设置栏的一组控件：select / range / number / text /
// switch 五种。只吃 props（fields + model + values + onChange），文案走字段自带的 i18n key，
// 各模态因此不必再各写一套表单。不可见字段由 visibleFields 过滤掉，这里不再判断。

const labelClass = 'text-xs font-medium text-muted-foreground';

function FieldControl({ field, model, value, disabled, onChange }) {
  const { t } = useTranslation();
  const id = `playground-field-${field.key}`;

  if (field.type === 'select') {
    const options = fieldOptions(field, model);
    // 取值本身就是 API 字面量时直接显示；声明了 optionLabelKey 的字段走 i18n。
    const optionLabel = (option) => {
      const labelKey = fieldOptionLabelKey(field, option);
      return labelKey ? t(labelKey) : option;
    };
    // 空串取值（「默认 = 不发送」）显示的是 placeholder，得把它的文案一并给过去。
    return (
      <Select value={String(value ?? '')} onValueChange={onChange} disabled={disabled}>
        <SelectTrigger id={id} className="h-8">
          <SelectValue placeholder={options.includes('') ? optionLabel('') : undefined} />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option} value={option}>
              {optionLabel(option)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    );
  }

  if (field.type === 'switch') {
    return <Switch id={id} checked={Boolean(value)} disabled={disabled} onCheckedChange={onChange} />;
  }

  if (field.type === 'range') {
    // 滑块与数字框是同一个值的两个入口：滑块给粗调，数字框给精确值。
    return (
      <div className="flex items-center gap-2">
        <input
          id={id}
          type="range"
          className="h-1.5 flex-1 accent-primary"
          min={field.min}
          max={field.max}
          step={field.step}
          value={Number(value ?? field.min ?? 0)}
          disabled={disabled}
          aria-label={t(field.labelKey)}
          onChange={(event) => onChange(Number(event.target.value))}
        />
        <Input
          className="h-8 w-16 text-center"
          type="number"
          min={field.min}
          max={field.max}
          step={field.step}
          value={value ?? ''}
          disabled={disabled}
          aria-label={t(field.labelKey)}
          onChange={(event) => onChange(event.target.value === '' ? '' : Number(event.target.value))}
        />
      </div>
    );
  }

  return (
    <Input
      id={id}
      className="h-8"
      type={field.type === 'number' ? 'number' : 'text'}
      min={field.min}
      max={field.max}
      step={field.step}
      value={value ?? ''}
      disabled={disabled}
      placeholder={field.placeholderKey ? t(field.placeholderKey) : undefined}
      onChange={(event) => onChange(event.target.value)}
    />
  );
}

const fieldShape = PropTypes.shape({
  key: PropTypes.string.isRequired,
  type: PropTypes.string.isRequired,
  labelKey: PropTypes.string,
  optionLabelKey: PropTypes.string,
  hintKey: PropTypes.string,
  placeholderKey: PropTypes.string,
  min: PropTypes.number,
  max: PropTypes.number,
  step: PropTypes.number
});

FieldControl.propTypes = {
  field: fieldShape.isRequired,
  model: PropTypes.object,
  value: PropTypes.any,
  disabled: PropTypes.bool,
  onChange: PropTypes.func.isRequired
};

export default function FieldRenderer({ fields, model, values, onChange, disabled = false }) {
  const { t } = useTranslation();

  return (
    <>
      {visibleFields(fields, model, values).map((field) => (
        <div key={field.key} className="space-y-1.5">
          {field.type === 'switch' ? (
            <div className="flex items-center justify-between gap-2">
              <span className={labelClass}>{t(field.labelKey)}</span>
              <FieldControl
                field={field}
                model={model}
                value={values[field.key]}
                disabled={disabled}
                onChange={(v) => onChange(field.key, v)}
              />
            </div>
          ) : (
            <>
              <label className={labelClass} htmlFor={`playground-field-${field.key}`}>
                {t(field.labelKey)}
              </label>
              <FieldControl
                field={field}
                model={model}
                value={values[field.key]}
                disabled={disabled}
                onChange={(v) => onChange(field.key, v)}
              />
            </>
          )}
          {field.hintKey && <p className="text-[11px] text-muted-foreground">{t(field.hintKey)}</p>}
        </div>
      ))}
    </>
  );
}

FieldRenderer.propTypes = {
  fields: PropTypes.arrayOf(fieldShape).isRequired,
  model: PropTypes.object,
  values: PropTypes.object.isRequired,
  onChange: PropTypes.func.isRequired,
  disabled: PropTypes.bool
};
