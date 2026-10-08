import { useCallback, useContext, useEffect, useState } from 'react';
import { ChevronRight, Copy, Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { API } from 'utils/api';
import { copy, showError, showSuccess } from 'utils/common';
import { LoadStatusContext } from 'contexts/StatusContext';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Switch } from '@/components/ui/switch';
import { Button } from '@/components/ui/button';
import { FormField } from '@/components/ui/form-field';

// Shared data layer + presentational helpers for the system settings tabs.
// All option keys map 1:1 to the v1 `/api/option/` API for functional parity.

export function useOptionSettings() {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState({});
  const [origin, setOrigin] = useState({});
  const [loading, setLoading] = useState(false);
  const loadStatus = useContext(LoadStatusContext);

  const refresh = useCallback(async () => {
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
        setInputs(next);
        setOrigin(next);
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

  const putOption = useCallback(async (key, value) => {
    const res = await API.put('/api/option/', { key, value });
    const { success, message } = res.data;
    if (!success) throw new Error(message);
    return true;
  }, []);

  const toggle = useCallback(
    async (key) => {
      const value = inputs[key] === 'true' ? 'false' : 'true';
      setLoading(true);
      try {
        await putOption(key, value);
        setInputs((p) => ({ ...p, [key]: value }));
        setOrigin((p) => ({ ...p, [key]: value }));
        if (loadStatus) await loadStatus();
        showSuccess(t('common.setSuccess'));
      } catch (e) {
        showError(e.message);
      } finally {
        setLoading(false);
      }
    },
    [inputs, putOption, loadStatus, t]
  );

  const saveKeys = useCallback(
    async (keys, options = {}) => {
      const { validate, transform } = options;
      if (validate) {
        const err = validate(inputs);
        if (err) {
          showError(err);
          return false;
        }
      }
      setLoading(true);
      try {
        for (const key of keys) {
          let value = inputs[key];
          if (transform && transform[key]) value = transform[key](value);
          if (origin[key] !== value) {
            await putOption(key, value ?? '');
          }
        }
        if (loadStatus) await loadStatus();
        await refresh();
        showSuccess(t('common.saveSuccess'));
        return true;
      } catch (e) {
        showError(e.message);
        return false;
      } finally {
        setLoading(false);
      }
    },
    [inputs, origin, putOption, loadStatus, refresh, t]
  );

  // Dirty check for a card's own option keys, so its save button stays disabled
  // until something actually changed. Both sides normalise to string because
  // number inputs turn origin values like 10 into '10'.
  const isDirty = useCallback((keys) => keys.some((key) => String(inputs[key] ?? '') !== String(origin[key] ?? '')), [inputs, origin]);

  return { inputs, setField, toggle, saveKeys, loading, refresh, isDirty };
}

export function SettingsCard({ title, description, headerAction, children, className, highlight }) {
  return (
    <Card className={className} data-highlight={highlight}>
      <CardHeader>
        {headerAction ? (
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0 space-y-2.5">
              <CardTitle className="text-lg">{title}</CardTitle>
              {description && <CardDescription>{description}</CardDescription>}
            </div>
            <div className="shrink-0">{headerAction}</div>
          </div>
        ) : (
          <div className="space-y-2.5">
            <CardTitle className="text-lg">{title}</CardTitle>
            {description && <CardDescription>{description}</CardDescription>}
          </div>
        )}
      </CardHeader>
      <CardContent className="space-y-4">{children}</CardContent>
    </Card>
  );
}

export function SettingsSection({ title, description, children, className }) {
  return (
    <section className={className ? `space-y-3 ${className}` : 'space-y-3'}>
      <div className="space-y-2.5">
        <h4 className="text-sm font-medium leading-none">{title}</h4>
        {description && <p className="text-xs text-muted-foreground">{description}</p>}
      </div>
      {children}
    </section>
  );
}

export function TextRow({
  id,
  label,
  value,
  onChange,
  placeholder,
  type = 'text',
  disabled,
  readOnly,
  multiline,
  rows = 6,
  description,
  error,
  endAdornment,
  copyable
}) {
  const { t } = useTranslation();
  const adornment =
    endAdornment ??
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
    <FormField id={id} label={label} hint={description} error={error}>
      <div className="relative">
        {multiline ? (
          <Textarea
            id={id}
            value={value ?? ''}
            onChange={(e) => onChange(e.target.value)}
            placeholder={placeholder}
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
            placeholder={placeholder}
            disabled={disabled}
            readOnly={readOnly}
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

export function ToggleRow({ label, description, checked, onCheckedChange, disabled }) {
  return (
    <div className={`flex ${description ? 'items-start' : 'items-center'} justify-between gap-4 rounded-md border border-border p-3`}>
      <div className="min-w-0 space-y-2.5">
        <p className="text-sm font-medium leading-none">{label}</p>
        {description && <p className="text-xs text-muted-foreground">{description}</p>}
      </div>
      <Switch checked={checked} onCheckedChange={onCheckedChange} disabled={disabled} />
    </div>
  );
}

// Imperative enable switch for SettingsCard.headerAction: saves instantly via toggle().
export function HeaderSwitch({ id, label, checked, onCheckedChange, disabled }) {
  const { t } = useTranslation();
  return (
    <div className="flex items-center gap-2">
      <Label htmlFor={id} className="font-normal text-muted-foreground">
        {label ?? t('common.enableToggle')}
      </Label>
      <Switch id={id} checked={checked} onCheckedChange={onCheckedChange} disabled={disabled} />
    </div>
  );
}

export function ConfirmAction({ title, description, confirmLabel, onConfirm, disabled, variant = 'destructive', children }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  const handleConfirm = async () => {
    setBusy(true);
    try {
      await onConfirm?.();
      setOpen(false);
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <Button variant={variant} disabled={disabled} onClick={() => setOpen(true)}>
        {children}
      </Button>
      <Dialog open={open} onOpenChange={(v) => !busy && setOpen(v)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            {description && <DialogDescription>{description}</DialogDescription>}
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" disabled={busy} onClick={() => setOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button variant={variant} disabled={busy} onClick={handleConfirm}>
              {busy && <Loader2 className="size-4 animate-spin" />}
              {confirmLabel ?? t('common.confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

export function SaveButton({ onClick, loading, disabled, children }) {
  return (
    <div className="flex justify-end pt-1">
      <Button onClick={onClick} disabled={loading || disabled}>
        {loading && <Loader2 className="size-4 animate-spin" />}
        {children}
      </Button>
    </div>
  );
}
