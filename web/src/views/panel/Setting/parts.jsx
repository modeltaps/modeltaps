import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react';
import { ChevronRight, Copy, KeyRound } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { API } from 'utils/api';
import { copy, showError, showSuccess } from 'utils/common';
import { LoadStatusContext } from 'contexts/StatusContext';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { FormField } from '@/components/ui/form-field';
import { OBJECT_PAGES, catalogEntry, settingDescription, settingLabel, splitFirstSentence } from '@/components/settings/catalog';
import { Link } from 'react-router';
import { ModifiedDot, SettingHelpBody, SettingHelpToggle, hasMoreHelp } from '@/components/settings/SettingHelp';

// Shared data layer + field helpers for the platform settings cards.
// All option keys map 1:1 to the `/api/option/` API.

export function useOptionSettings() {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState({});
  const [origin, setOrigin] = useState({});
  // Secret options ({ key: mask }, '' when unset): the backend never returns their values,
  // only a mask. Their inputs start empty, and saving an empty one keeps the stored value.
  const [secrets, setSecrets] = useState({});
  // Whether stored credentials are encrypted at rest (ENCRYPTION_KEY set); null until loaded.
  const [encryptionEnabled, setEncryptionEnabled] = useState(null);
  // Each option's value right after registration ({ key: value }, secrets left out): the default
  // the "modified" marks compare the stored value with.
  const [defaults, setDefaults] = useState({});
  const [loading, setLoading] = useState(false);
  const loadStatus = useContext(LoadStatusContext);
  // Latest values for the async save path, which must not depend on a stale render.
  const latest = useRef({ inputs, origin });
  latest.current = { inputs, origin };

  // Reloads every option. With `keepUnsaved`, unsaved edits survive the reload: after one card
  // saves, the other cards keep what the user typed in them.
  const refresh = useCallback(async ({ keepUnsaved = false } = {}) => {
    try {
      const res = await API.get('/api/option/');
      const { success, message, data } = res.data;
      if (success) {
        const next = {};
        data.forEach((item) => {
          let v = item.value;
          if (typeof v === 'boolean') v = v.toString();
          next[item.key] = v;
        });
        const { inputs: prevInputs, origin: prevOrigin } = latest.current;
        const merged = { ...next };
        if (keepUnsaved) {
          for (const key of Object.keys(prevInputs)) {
            if (!sameOption(prevInputs[key], prevOrigin[key])) merged[key] = prevInputs[key];
          }
        }
        setInputs(merged);
        setOrigin(next);
        setSecrets(res.data.secrets || {});
        setDefaults(res.data.defaults || {});
        setEncryptionEnabled(typeof res.data.encryption_enabled === 'boolean' ? res.data.encryption_enabled : null);
      } else {
        showError(message);
      }
    } catch (e) {
      /* network errors surfaced globally by the API interceptor */
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const setField = useCallback((name, value) => setInputs((p) => ({ ...p, [name]: value })), []);
  // Boolean options are stored as 'true' / 'false' strings.
  const setBool = useCallback((name, value) => setInputs((p) => ({ ...p, [name]: value ? 'true' : 'false' })), []);

  const putOption = useCallback(async (key, value) => {
    const res = await API.put('/api/option/', { key, value });
    const { success, message } = res.data;
    if (!success) throw new Error(message);
    return true;
  }, []);

  // Saves the changed keys of one card. `values` overrides inputs (used by collection cards that
  // save one item of a JSON option right away).
  const saveKeys = useCallback(
    async (keys, options = {}) => {
      const { validate, transform, values } = options;
      const current = { ...latest.current.inputs, ...(values || {}) };
      if (validate) {
        const err = validate(current);
        if (err) {
          showError(err);
          return false;
        }
      }
      setLoading(true);
      try {
        for (const key of keys) {
          let value = current[key];
          if (transform && transform[key]) value = transform[key](value);
          if (key in secrets && !value) continue;
          if (!sameOption(latest.current.origin[key], value)) {
            await putOption(key, value ?? '');
          }
        }
        if (loadStatus) await loadStatus();
        await refresh({ keepUnsaved: true });
        // Keys saved from `values` replace any pending edit of them. A saved secret leaves its
        // input: the server now holds it and shows only the mask, so the card is no longer dirty.
        setInputs((p) => {
          const next = { ...p, ...(values || {}) };
          for (const key of keys) if (key in secrets) next[key] = '';
          return next;
        });
        showSuccess(t('common.saveSuccess'));
        return true;
      } catch (e) {
        showError(e.message);
        return false;
      } finally {
        setLoading(false);
      }
    },
    [secrets, putOption, loadStatus, refresh, t]
  );

  // Undo: puts a card's keys back to the stored values.
  const resetKeys = useCallback(
    (keys) =>
      setInputs((p) => {
        const next = { ...p };
        keys.forEach((key) => {
          if (key in origin) next[key] = origin[key];
          else delete next[key];
        });
        return next;
      }),
    [origin]
  );

  // Dirty check for a card's own option keys, so its save button stays disabled
  // until something actually changed. Both sides normalise to string because
  // number inputs turn origin values like 10 into '10'.
  const isDirty = useCallback((keys) => keys.some((key) => !sameOption(inputs[key], origin[key])), [inputs, origin]);

  return { inputs, origin, secrets, defaults, encryptionEnabled, setField, setBool, saveKeys, resetKeys, loading, refresh, isDirty };
}

const sameOption = (a, b) => String(a ?? '') === String(b ?? '');

// Whether a stored option differs from its default ({ defaults, origin } of useOptionSettings).
export const isModified = (meta, key) =>
  Boolean(meta?.defaults) && key in meta.defaults && key in (meta.origin || {}) && !sameOption(meta.origin[key], meta.defaults[key]);

// The stored and default values of the settings page, for the "modified" marks of its fields.
// Null outside the platform settings page.
export const SettingsMetaContext = createContext(null);

// A field's text from the settings catalog when the card does not pass its own: the label, the
// first sentence of the description under the field, and the rest in the help its `?` opens.
// `id` is the option key. With a description of the card's own, the help holds the whole
// catalog description. `modified` holds the default when the stored value differs from it.
export function useSettingCopy(id, label, description) {
  const { t } = useTranslation();
  const meta = useContext(SettingsMetaContext);
  const entry = catalogEntry(id);
  if (!entry) return { label, description, help: null, modified: null };
  const full = description !== undefined;
  return {
    label: label ?? settingLabel(t, id),
    description: description ?? splitFirstSentence(settingDescription(t, id)).first,
    help: hasMoreHelp(t, id, full) ? <SettingHelpBody settingKey={id} full={full} /> : null,
    helpFull: full,
    modified: isModified(meta, id) ? { defaultValue: String(meta.defaults[id] ?? '') } : null,
    overridden: meta?.overrides?.[id]?.count > 0 ? meta.overrides[id] : null
  };
}

// Objects that set their own value instead of following this site setting, linked to their list.
const OVERRIDE_PAGES = { channel: OBJECT_PAGES.channels, model: OBJECT_PAGES.models };

function OverriddenNote({ overridden }) {
  const { t } = useTranslation();
  if (!overridden) return null;
  const object = t(`settingsUi.catalog.objects.${overridden.scope}`, { defaultValue: overridden.scope });
  const text = t('settingsUi.catalog.overridden', { count: overridden.count, object });
  const href = OVERRIDE_PAGES[overridden.scope];
  return (
    <span className="block text-xs text-muted-foreground" data-testid="setting-overridden">
      {href ? (
        <Link to={href} className="underline underline-offset-2 hover:text-foreground">
          {text} ↗
        </Link>
      ) : (
        text
      )}
    </span>
  );
}

// The description under a field, with the note of objects overriding it.
const withOverrides = (description, overridden) =>
  overridden ? (
    <>
      {description}
      <OverriddenNote overridden={overridden} />
    </>
  ) : (
    description
  );

const withDot = (label, modified) =>
  modified && label ? (
    <>
      {label}
      <ModifiedDot defaultValue={modified.defaultValue} />
    </>
  ) : (
    label
  );

// FormCard props for a card saved through the option API: dirty state, Save and Undo bound to
// the card's own keys. `options` are passed to saveKeys (validate / transform); `beforeSave`
// may return false (or a promise of false) to stop, e.g. to ask for confirmation first;
// `afterSave` runs after a successful save.
export function useOptionCard(ctx, keys, options = {}) {
  const [saving, setSaving] = useState(false);
  const dirty = ctx.isDirty(keys);
  const save = async () => {
    setSaving(true);
    try {
      const ok = await ctx.saveKeys(keys, options);
      if (ok && options.afterSave) await options.afterSave();
      return ok;
    } finally {
      setSaving(false);
    }
  };
  return {
    dirty,
    saving,
    save,
    cardProps: {
      dirty,
      saving,
      disabled: ctx.loading && !saving,
      onSave: async () => {
        if (options.beforeSave && (await options.beforeSave()) === false) return;
        await save();
      },
      onReset: () => ctx.resetKeys(keys)
    }
  };
}

// A unit written into a label ("重试间隔(秒)", "Cooldown (s)", "测试间隔（分钟）") moves into the input
// as a suffix, so the label reads as a name. A bare "s" needs a space before the bracket so plural
// forms like "model(s)" are left alone.
const UNIT_RE = /^(.*?)\s*[(（]\s*(秒|分钟|分鐘|小时|小時|天|毫秒|分|時間|日|%|s|ms|sec|seconds|minutes|hours|days|MB|KB)\s*[)）]\s*$/;

export function splitLabelUnit(label) {
  if (typeof label !== 'string') return { name: label, unit: null };
  const m = label.match(UNIT_RE);
  if (!m || !m[1]) return { name: label, unit: null };
  if (m[2] === 's' && !/\s[(（]\s*s\s*[)）]\s*$/.test(label)) return { name: label, unit: null };
  return { name: m[1], unit: m[2] };
}

export function TextRow({
  id,
  label: labelProp,
  // The card title already names the field (a one-field card): keep the label for screen readers only.
  labelHidden = false,
  unit: unitProp,
  value,
  onChange,
  placeholder,
  type = 'text',
  disabled,
  readOnly,
  multiline,
  rows = 6,
  description: descriptionProp,
  error,
  endAdornment,
  copyable,
  secret
}) {
  const { t } = useTranslation();
  const copyText = useSettingCopy(id, labelProp, descriptionProp);
  const description = copyText.description;
  const { name: label, unit: labelUnit } = splitLabelUnit(copyText.label);
  const catalogUnit = catalogEntry(id)?.unit;
  const unit = multiline ? null : (unitProp ?? labelUnit ?? catalogUnit ?? null);
  // `secret` is the backend mask of a secret option ('' when unset, undefined for ordinary
  // fields): show it above the empty input so the stored value can be checked.
  const secretSet = Boolean(secret);
  const adornment =
    endAdornment ??
    (unit ? (
      <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">{unit}</span>
    ) : null) ??
    (copyable && !multiline ? (
      <Button
        type="button"
        variant="ghost"
        className="absolute right-1 top-1/2 h-8 w-8 -translate-y-1/2 p-0 text-muted-foreground hover:text-foreground"
        aria-label={t('common.copy')}
        onClick={() => copy(value ?? '', label ?? '')}
      >
        <Copy />
      </Button>
    ) : null);
  return (
    <FormField
      id={id}
      data-setting-key={catalogEntry(id) ? id : undefined}
      data-highlight={catalogEntry(id) ? id : undefined}
      label={labelHidden ? undefined : withDot(label, copyText.modified)}
      hint={secretSet ? t('common.secretKeepHint') : withOverrides(description, copyText.overridden)}
      // A hidden label leaves no place for the `?`: the card title and the hint stand for it.
      help={labelHidden ? null : copyText.help}
      error={error}
    >
      {secretSet && (
        <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <KeyRound className="size-3.5" />
          {t('common.secretConfigured', { masked: secret })}
        </p>
      )}
      <div className="relative">
        {multiline ? (
          <Textarea
            id={id}
            value={value ?? ''}
            onChange={(e) => onChange(e.target.value)}
            placeholder={secretSet ? '••••••••' : placeholder}
            disabled={disabled}
            readOnly={readOnly}
            rows={rows}
          />
        ) : (
          <Input
            id={id}
            type={type}
            value={value ?? ''}
            onChange={(e) => onChange(e.target.value)}
            placeholder={secretSet ? '••••••••' : placeholder}
            disabled={disabled}
            readOnly={readOnly}
            aria-label={labelHidden ? label : undefined}
            className={adornment ? 'pr-12' : undefined}
          />
        )}
        {adornment}
      </div>
    </FormField>
  );
}

// Disclosure for long help text replacing wall-of-text info Alerts (#8): a small
// trigger that expands plain-text lines, keeping card body scannable.
export function CollapsibleHelp({ trigger, lines, children, className }) {
  const [open, setOpen] = useState(false);
  return (
    <div className={className ? `space-y-2 ${className}` : 'space-y-2'}>
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
      >
        <ChevronRight className={`size-3.5 transition-transform ${open ? 'rotate-90' : ''}`} />
        {trigger}
      </button>
      {open && (
        <div className="space-y-1.5 rounded-md border border-border bg-muted/30 p-3 text-xs leading-relaxed text-muted-foreground">
          {lines ? lines.map((line, i) => <p key={i}>{line}</p>) : children}
        </div>
      )}
    </div>
  );
}

// One switch row of an OptionList (the list draws the frame and the lines between rows).
export function ToggleRow({ id, label: labelProp, description: descriptionProp, checked, onCheckedChange, disabled, children }) {
  const copyText = useSettingCopy(id, labelProp, descriptionProp);
  const { label, description } = copyText;
  const known = Boolean(catalogEntry(id));
  return (
    <div className="space-y-3 px-4 py-3" data-setting-key={known ? id : undefined} data-highlight={known ? id : undefined}>
      <div className={`flex ${description ? 'items-start' : 'items-center'} justify-between gap-4`}>
        <div className="flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1">
          <Label htmlFor={id} className="text-sm font-medium leading-5">
            {withDot(label, copyText.modified)}
          </Label>
          {copyText.help && <SettingHelpToggle settingKey={id} full={copyText.helpFull} />}
          {(description || copyText.overridden) && (
            <div className="basis-full text-xs text-muted-foreground">{withOverrides(description, copyText.overridden)}</div>
          )}
        </div>
        <Switch id={id} checked={!!checked} onCheckedChange={onCheckedChange} disabled={disabled} />
      </div>
      {children}
    </div>
  );
}

// One row of an OptionList with the label on the left and any control on the right, for a page
// that keeps every explanation behind the `?` instead of a line under the label.
export function InlineRow({ id, label: labelProp, children }) {
  const copyText = useSettingCopy(id, labelProp, '');
  const known = Boolean(catalogEntry(id));
  // The control on the right shows the unit, so the label drops its "(unit)".
  const { name } = splitLabelUnit(copyText.label);
  return (
    <div className="space-y-3 px-4 py-3" data-setting-key={known ? id : undefined} data-highlight={known ? id : undefined}>
      <div className="flex items-center justify-between gap-4">
        <div className="flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1">
          <Label htmlFor={id} className="text-sm font-medium leading-5">
            {withDot(name, copyText.modified)}
          </Label>
          {copyText.help && <SettingHelpToggle settingKey={id} full={copyText.helpFull} />}
          {copyText.overridden && <div className="basis-full text-xs text-muted-foreground">{withOverrides('', copyText.overridden)}</div>}
        </div>
        <div className="flex shrink-0 items-center gap-2">{children}</div>
      </div>
    </div>
  );
}

// Labelled select bound to one option; the label sits above the control. An enum setting of the
// catalog may leave out `options` labels: they come from the catalog.
export function SelectRow({ id, label: labelProp, description: descriptionProp, value, onChange, options, disabled, className }) {
  const copyText = useSettingCopy(id, labelProp, descriptionProp);
  const known = Boolean(catalogEntry(id));
  return (
    <FormField
      id={id}
      data-setting-key={known ? id : undefined}
      data-highlight={known ? id : undefined}
      label={withDot(copyText.label, copyText.modified)}
      hint={withOverrides(copyText.description, copyText.overridden)}
      help={copyText.help}
      className={className}
    >
      <Select value={value} onValueChange={onChange} disabled={disabled}>
        <SelectTrigger id={id}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((o) => (
            <SelectItem key={o.value} value={o.value}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </FormField>
  );
}
