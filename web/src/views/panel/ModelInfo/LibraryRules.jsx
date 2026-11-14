import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ArrowDown, ArrowUp, Loader2, Play, Plus, RotateCcw, Save, Search, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from '@/components/ui/sonner';
import { API } from 'utils/api';

// The built-in rules carry English names; show them in the UI language until an admin renames one.
const DEFAULT_RULE_NAME_KEYS = {
  'Retire deprecated models': 'libraryRules.defaultNames.retireDeprecated',
  'Previews as drafts': 'libraryRules.defaultNames.previews',
  'Publish new models from the main labs': 'libraryRules.defaultNames.publishMain',
  'Other supplied models as drafts': 'libraryRules.defaultNames.otherSupplied'
};

export const RULE_ACTIONS = ['publish', 'draft', 'ignore', 'retire'];
const TRISTATE = ['any', 'yes', 'no'];

const splitList = (text) =>
  String(text || '')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);

const triFrom = (v) => (v === true ? 'yes' : v === false ? 'no' : 'any');
const triTo = (v) => (v === 'yes' ? true : v === 'no' ? false : undefined);

// Rules as the form edits them (lists as comma text, booleans as any / yes / no) and back.
export function ruleToForm(rule) {
  return {
    name: rule.name || '',
    labs: (rule.labs || []).join(', '),
    patterns: (rule.patterns || []).join(', '),
    days: rule.released_within_days ? String(rule.released_within_days) : '',
    supplied: triFrom(rule.supplied),
    deprecated: triFrom(rule.deprecated),
    openWeights: triFrom(rule.open_weights),
    endpoints: (rule.endpoints || []).join(', '),
    action: rule.action || 'draft'
  };
}

export function formToRule(form) {
  const rule = { name: form.name.trim(), action: form.action };
  const labs = splitList(form.labs);
  const patterns = splitList(form.patterns);
  const endpoints = splitList(form.endpoints);
  if (labs.length) rule.labs = labs;
  if (patterns.length) rule.patterns = patterns;
  if (endpoints.length) rule.endpoints = endpoints;
  const days = parseInt(form.days, 10);
  if (days > 0) rule.released_within_days = days;
  const supplied = triTo(form.supplied);
  if (supplied !== undefined) rule.supplied = supplied;
  const deprecated = triTo(form.deprecated);
  if (deprecated !== undefined) rule.deprecated = deprecated;
  const openWeights = triTo(form.openWeights);
  if (openWeights !== undefined) rule.open_weights = openWeights;
  return rule;
}

const EMPTY_RULE = {
  name: '',
  labs: '',
  patterns: '',
  days: '',
  supplied: 'yes',
  deprecated: 'any',
  openWeights: 'any',
  endpoints: '',
  action: 'draft'
};

function TriSelect({ value, onChange, label }) {
  const { t } = useTranslation();
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger className="h-8 w-24" aria-label={label}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {TRISTATE.map((v) => (
          <SelectItem key={v} value={v}>
            {t(`libraryRules.tri.${v}`)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

TriSelect.propTypes = { value: PropTypes.string, onChange: PropTypes.func, label: PropTypes.string };

function Field({ label, children, className = '' }) {
  return (
    <label className={`space-y-1 ${className}`}>
      <span className="block text-xs text-muted-foreground">{label}</span>
      {children}
    </label>
  );
}

Field.propTypes = { label: PropTypes.string, children: PropTypes.node, className: PropTypes.string };

function RuleCard({ index, total, form, onChange, onMove, onRemove }) {
  const { t } = useTranslation();
  const set = (patch) => onChange({ ...form, ...patch });
  return (
    <div className="space-y-3 rounded-[10px] border border-border p-4">
      <div className="flex items-center gap-2">
        <span className="w-6 shrink-0 text-sm tabular-nums text-muted-foreground">{index + 1}</span>
        <Input
          value={DEFAULT_RULE_NAME_KEYS[form.name] ? t(DEFAULT_RULE_NAME_KEYS[form.name]) : form.name}
          onChange={(e) => set({ name: e.target.value })}
          placeholder={t('libraryRules.namePlaceholder')}
          aria-label={t('libraryRules.name')}
          className="h-8 flex-1"
        />
        <Select value={form.action} onValueChange={(v) => set({ action: v })}>
          <SelectTrigger className="h-8 w-32" aria-label={t('libraryRules.action')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {RULE_ACTIONS.map((a) => (
              <SelectItem key={a} value={a}>
                {t(`libraryRules.actions.${a}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button
          variant="ghost"
          size="icon"
          className="size-8"
          disabled={index === 0}
          onClick={() => onMove(-1)}
          aria-label={t('libraryRules.up')}
        >
          <ArrowUp className="size-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="size-8"
          disabled={index === total - 1}
          onClick={() => onMove(1)}
          aria-label={t('libraryRules.down')}
        >
          <ArrowDown className="size-4" />
        </Button>
        <Button variant="ghost" size="icon" className="size-8" onClick={onRemove} aria-label={t('libraryRules.remove')}>
          <Trash2 className="size-4" />
        </Button>
      </div>
      <div className="grid gap-3 pl-8 sm:grid-cols-2 lg:grid-cols-4">
        <Field label={t('libraryRules.labs')} className="lg:col-span-2">
          <Input
            value={form.labs}
            onChange={(e) => set({ labs: e.target.value })}
            placeholder={t('libraryRules.anyTime')}
            className="h-8 font-mono"
          />
        </Field>
        <Field label={t('libraryRules.patterns')} className="lg:col-span-2">
          <Input
            value={form.patterns}
            onChange={(e) => set({ patterns: e.target.value })}
            placeholder={t('libraryRules.anyTime')}
            className="h-8 font-mono"
          />
        </Field>
        <Field label={t('libraryRules.days')}>
          <Input
            value={form.days}
            inputMode="numeric"
            onChange={(e) => set({ days: e.target.value.replace(/\D/g, '') })}
            placeholder={t('libraryRules.anyTime')}
            className="h-8"
          />
        </Field>
        <Field label={t('libraryRules.endpoints')}>
          <Input
            value={form.endpoints}
            onChange={(e) => set({ endpoints: e.target.value })}
            placeholder={t('libraryRules.anyTime')}
            className="h-8 font-mono"
          />
        </Field>
        <div className="flex flex-wrap gap-3 lg:col-span-2">
          <Field label={t('libraryRules.supplied')}>
            <TriSelect value={form.supplied} onChange={(v) => set({ supplied: v })} label={t('libraryRules.supplied')} />
          </Field>
          <Field label={t('libraryRules.deprecated')}>
            <TriSelect value={form.deprecated} onChange={(v) => set({ deprecated: v })} label={t('libraryRules.deprecated')} />
          </Field>
          <Field label={t('libraryRules.openWeights')}>
            <TriSelect value={form.openWeights} onChange={(v) => set({ openWeights: v })} label={t('libraryRules.openWeights')} />
          </Field>
        </div>
      </div>
    </div>
  );
}

RuleCard.propTypes = {
  index: PropTypes.number,
  total: PropTypes.number,
  form: PropTypes.object,
  onChange: PropTypes.func,
  onMove: PropTypes.func,
  onRemove: PropTypes.func
};

function PreviewList({ outcomes }) {
  const { t } = useTranslation();
  if (!outcomes) return null;
  const pending = outcomes.filter((o) => !o.done);
  if (pending.length === 0) return <p className="text-sm text-muted-foreground">{t('libraryRules.previewEmpty')}</p>;
  return (
    <div className="space-y-3">
      {RULE_ACTIONS.filter((a) => a !== 'ignore').map((action) => {
        const list = pending.filter((o) => o.action === action);
        if (!list.length) return null;
        return (
          <div key={action} className="space-y-1.5">
            <div className="text-sm font-medium">
              {t(`libraryRules.actions.${action}`)} · {list.length}
            </div>
            <div className="flex flex-wrap gap-1.5">
              {list.map((o) => (
                <span key={o.canonical} title={o.rule} className="rounded-md bg-muted px-1.5 py-0.5 font-mono text-xs">
                  {o.model}
                </span>
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}

PreviewList.propTypes = { outcomes: PropTypes.array };

// Inclusion rules: after each library sync they decide which new models join the catalog and
// which are retired. Off "apply", they only notify (a dry run).
export default function LibraryRules() {
  const { t } = useTranslation();
  const [state, setState] = useState(null);
  const [forms, setForms] = useState([]);
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState('');
  const [outcomes, setOutcomes] = useState(null);

  useEffect(() => {
    let cancelled = false;
    API.get('/api/model_info/library/rules')
      .then((res) => {
        const { success, data, message } = res.data || {};
        if (cancelled) return;
        if (!success) return toast.error(message);
        setState(data);
        setForms((data.rules || []).map(ruleToForm));
      })
      .catch(() => {
        /* shown by the API client */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const put = async (body, key) => {
    setBusy(key);
    try {
      const { success, data, message } = (await API.put('/api/model_info/library/rules', body)).data || {};
      if (!success) return toast.error(message);
      setState(data);
      setForms((data.rules || []).map(ruleToForm));
      setDirty(false);
      toast.success(t('libraryRules.saved'));
    } catch {
      /* shown by the API client */
    } finally {
      setBusy('');
    }
  };

  const edit = (next) => {
    setForms(next);
    setDirty(true);
    setOutcomes(null);
  };

  const preview = async () => {
    setBusy('preview');
    try {
      const { success, data, message } =
        (await API.post('/api/model_info/library/rules/preview', { rules: forms.map(formToRule) })).data || {};
      if (!success) return toast.error(message);
      setOutcomes(data || []);
    } catch {
      /* shown by the API client */
    } finally {
      setBusy('');
    }
  };

  const runNow = async () => {
    setBusy('run');
    try {
      const { success, data, message } = (await API.post('/api/model_info/library/rules/run')).data || {};
      if (!success) return toast.error(message);
      toast.success(
        data.apply
          ? t('libraryRules.ranApply', { added: data.added, retired: data.retired })
          : t('libraryRules.ranDry', { count: data.reported })
      );
      setOutcomes(data.outcomes || []);
    } catch {
      /* shown by the API client */
    } finally {
      setBusy('');
    }
  };

  if (!state) {
    return (
      <div className="flex justify-center py-16 text-muted-foreground">
        <Loader2 className="size-5 animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">{t('libraryRules.intro')}</p>
      <div className="flex flex-wrap items-center gap-x-6 gap-y-2 rounded-[10px] border border-border p-4 text-sm">
        <label className="flex cursor-pointer items-center gap-2">
          <Switch checked={state.enabled} disabled={busy !== ''} onCheckedChange={(v) => put({ enabled: v }, 'enabled')} />
          {t('libraryRules.enabled')}
        </label>
        <label className="flex cursor-pointer items-center gap-2">
          <Switch checked={state.apply} disabled={busy !== ''} onCheckedChange={(v) => put({ apply: v }, 'apply')} />
          {t('libraryRules.apply')}
        </label>
        <span className="text-xs text-muted-foreground">{state.apply ? t('libraryRules.applyOn') : t('libraryRules.applyOff')}</span>
      </div>

      <div className="space-y-2">
        {forms.map((form, i) => (
          <RuleCard
            key={i}
            index={i}
            total={forms.length}
            form={form}
            onChange={(next) => edit(forms.map((f, j) => (j === i ? next : f)))}
            onMove={(delta) => {
              const next = [...forms];
              [next[i], next[i + delta]] = [next[i + delta], next[i]];
              edit(next);
            }}
            onRemove={() => edit(forms.filter((_, j) => j !== i))}
          />
        ))}
        {forms.length === 0 && <p className="text-sm text-muted-foreground">{t('libraryRules.noRules')}</p>}
        <p className="text-xs text-muted-foreground">{t('libraryRules.orderHint')}</p>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Button variant="outline" size="sm" onClick={() => edit([...forms, { ...EMPTY_RULE }])}>
          <Plus className="size-4" /> {t('libraryRules.add')}
        </Button>
        <Button variant="outline" size="sm" disabled={busy !== ''} onClick={preview}>
          {busy === 'preview' ? <Loader2 className="size-4 animate-spin" /> : <Search className="size-4" />} {t('libraryRules.preview')}
        </Button>
        <Button
          variant="outline"
          size="sm"
          disabled={busy !== '' || dirty}
          onClick={runNow}
          title={dirty ? t('libraryRules.saveFirst') : undefined}
        >
          {busy === 'run' ? <Loader2 className="size-4 animate-spin" /> : <Play className="size-4" />} {t('libraryRules.runNow')}
        </Button>
        <div className="ml-auto flex gap-2">
          {state.custom && (
            <Button variant="ghost" size="sm" disabled={busy !== ''} onClick={() => put({ reset: true }, 'reset')}>
              <RotateCcw className="size-4" /> {t('libraryRules.reset')}
            </Button>
          )}
          <Button size="sm" disabled={busy !== '' || !dirty} onClick={() => put({ rules: forms.map(formToRule) }, 'save')}>
            {busy === 'save' ? <Loader2 className="size-4 animate-spin" /> : <Save className="size-4" />} {t('libraryRules.save')}
          </Button>
        </div>
      </div>

      {outcomes && (
        <div className="space-y-2 rounded-[10px] border border-border p-4">
          <div className="text-sm font-medium">{t('libraryRules.previewTitle')}</div>
          <PreviewList outcomes={outcomes} />
        </div>
      )}
    </div>
  );
}
