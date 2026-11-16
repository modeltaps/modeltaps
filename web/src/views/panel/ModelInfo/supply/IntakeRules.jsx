import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

// Intake rules decide what happens to a model a channel's upstream starts listing: switch its
// supply on, leave it for review, add it off, or skip it, and where it joins the model's order.
// The editor works on rule objects as the backend stores them (see model.SupplyIntakeRule).

export const INTAKE_ACTIONS = ['enable', 'pending', 'disable', 'skip'];
export const INTAKE_PLACEMENTS = ['auto', 'policy', 'last'];
export const INTAKE_ENDPOINTS = ['chat', 'responses', 'realtime', 'images', 'audio.speech', 'audio.transcription', 'embeddings', 'rerank'];
const TRISTATE = ['any', 'yes', 'no'];

// The built-in rules carry English names; show them in the UI language until an admin renames one.
const DEFAULT_NAME_KEYS = {
  'Previews wait for review': 'intakeRules.defaultNames.previews',
  'Chat models join at once': 'intakeRules.defaultNames.chat',
  'Everything else waits for review': 'intakeRules.defaultNames.rest'
};

export const intakeRuleName = (t, name) => (DEFAULT_NAME_KEYS[name] ? t(DEFAULT_NAME_KEYS[name]) : name);

const splitList = (text) =>
  String(text || '')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);

const triFrom = (v) => (v === true ? 'yes' : v === false ? 'no' : 'any');
const triTo = (v) => (v === 'yes' ? true : v === 'no' ? false : undefined);

// A rule as the form edits it (lists as comma text, the boolean as any / yes / no) and back.
export function intakeRuleToForm(rule = {}) {
  return {
    name: rule.name || '',
    endpoints: rule.endpoints || [],
    families: (rule.families || []).join(', '),
    patterns: (rule.patterns || []).join(', '),
    vendors: rule.vendors || [],
    supplied: triFrom(rule.supplied),
    action: rule.action || 'pending',
    placement: rule.placement || 'auto'
  };
}

export function intakeFormToRule(form) {
  const rule = { name: form.name.trim(), action: form.action };
  if (form.endpoints.length) rule.endpoints = form.endpoints;
  const families = splitList(form.families);
  const patterns = splitList(form.patterns);
  if (families.length) rule.families = families;
  if (patterns.length) rule.patterns = patterns;
  if (form.vendors.length) rule.vendors = form.vendors;
  const supplied = triTo(form.supplied);
  if (supplied !== undefined) rule.supplied = supplied;
  if (form.action !== 'skip' && form.placement !== 'auto') rule.placement = form.placement;
  return rule;
}

// Stored rules (a JSON string) as rule objects; null for an empty value (the defaults apply) or
// one that does not parse.
export function parseIntakeRules(raw) {
  if (!raw || !String(raw).trim()) return null;
  try {
    const rules = JSON.parse(raw);
    return Array.isArray(rules) ? rules : null;
  } catch {
    return null;
  }
}

export const intakeRulesJSON = (rules) => JSON.stringify(rules);

// One rule in words, for read-only lists: "chat, responses · others supply it → on, joins last".
export function describeIntakeRule(t, rule) {
  const when = [];
  if (rule.endpoints?.length) when.push(t('intakeRules.when.endpoints', { list: rule.endpoints.join(', ') }));
  if (rule.families?.length) when.push(t('intakeRules.when.families', { list: rule.families.join(', ') }));
  if (rule.patterns?.length) when.push(t('intakeRules.when.patterns', { list: rule.patterns.join(', ') }));
  if (rule.vendors?.length) when.push(t('intakeRules.when.vendors', { list: rule.vendors.join(', ') }));
  if (rule.supplied === true) when.push(t('intakeRules.when.supplied'));
  if (rule.supplied === false) when.push(t('intakeRules.when.unsupplied'));
  if (!when.length) when.push(t('intakeRules.when.any'));
  const then = [t(`intakeRules.actions.${rule.action}`)];
  if (rule.action !== 'skip') then.push(t(`intakeRules.placements.${rule.placement || 'auto'}`));
  return { when: when.join(' · '), then: then.join(' · ') };
}

function TriSelect({ value, onChange, label }) {
  const { t } = useTranslation();
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger className="h-8 w-28" aria-label={label}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {TRISTATE.map((v) => (
          <SelectItem key={v} value={v}>
            {t(`intakeRules.tri.${v}`)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

TriSelect.propTypes = { value: PropTypes.string, onChange: PropTypes.func, label: PropTypes.string };

function Labeled({ label, children, className = '' }) {
  return (
    <div className={cn('space-y-1', className)}>
      <span className="block text-xs text-muted-foreground">{label}</span>
      {children}
    </div>
  );
}

Labeled.propTypes = { label: PropTypes.string, children: PropTypes.node, className: PropTypes.string };

function RuleCard({ index, total, form, onChange, onMove, onRemove }) {
  const { t } = useTranslation();
  const set = (patch) => onChange({ ...form, ...patch });
  const toggleEndpoint = (e) =>
    set({ endpoints: form.endpoints.includes(e) ? form.endpoints.filter((x) => x !== e) : [...form.endpoints, e] });
  return (
    <div data-testid="intake-rule" className="space-y-3 rounded-[10px] border border-border p-3">
      <div className="flex flex-wrap items-center gap-2">
        <span className="w-5 shrink-0 text-sm tabular-nums text-muted-foreground">{index + 1}</span>
        <Input
          value={intakeRuleName(t, form.name)}
          onChange={(e) => set({ name: e.target.value })}
          placeholder={t('intakeRules.namePlaceholder')}
          aria-label={t('intakeRules.name')}
          className="h-8 min-w-40 flex-1"
        />
        <Button
          variant="ghost"
          size="icon"
          className="size-8"
          disabled={index === 0}
          onClick={() => onMove(-1)}
          aria-label={t('intakeRules.up')}
        >
          <ArrowUp className="size-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="size-8"
          disabled={index === total - 1}
          onClick={() => onMove(1)}
          aria-label={t('intakeRules.down')}
        >
          <ArrowDown className="size-4" />
        </Button>
        <Button variant="ghost" size="icon" className="size-8" onClick={onRemove} aria-label={t('intakeRules.remove')}>
          <Trash2 className="size-4" />
        </Button>
      </div>
      <div className="grid gap-3 pl-7 sm:grid-cols-2">
        <Labeled label={t('intakeRules.endpoints')} className="sm:col-span-2">
          <div className="flex flex-wrap gap-1.5">
            {INTAKE_ENDPOINTS.map((e) => (
              <button
                key={e}
                type="button"
                aria-pressed={form.endpoints.includes(e)}
                onClick={() => toggleEndpoint(e)}
                className={cn(
                  'rounded-md border px-2 py-0.5 font-mono text-xs',
                  form.endpoints.includes(e) ? 'border-foreground bg-foreground text-background' : 'border-border bg-field hover:bg-muted'
                )}
              >
                {e}
              </button>
            ))}
          </div>
        </Labeled>
        <Labeled label={t('intakeRules.families')}>
          <Input
            value={form.families}
            onChange={(e) => set({ families: e.target.value })}
            placeholder={t('intakeRules.tri.any')}
            className="h-8 font-mono"
          />
        </Labeled>
        <Labeled label={t('intakeRules.patterns')}>
          <Input
            value={form.patterns}
            onChange={(e) => set({ patterns: e.target.value })}
            placeholder={t('intakeRules.tri.any')}
            className="h-8 font-mono"
          />
        </Labeled>
        <Labeled label={t('intakeRules.supplied')}>
          <TriSelect value={form.supplied} onChange={(v) => set({ supplied: v })} label={t('intakeRules.supplied')} />
        </Labeled>
        <div className="flex flex-wrap gap-3">
          <Labeled label={t('intakeRules.action')}>
            <Select value={form.action} onValueChange={(v) => set({ action: v })}>
              <SelectTrigger className="h-8 w-32" aria-label={t('intakeRules.action')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {INTAKE_ACTIONS.map((a) => (
                  <SelectItem key={a} value={a}>
                    {t(`intakeRules.actions.${a}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Labeled>
          {form.action !== 'skip' && (
            <Labeled label={t('intakeRules.placement')}>
              <Select value={form.placement} onValueChange={(v) => set({ placement: v })}>
                <SelectTrigger className="h-8 w-44" aria-label={t('intakeRules.placement')}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {INTAKE_PLACEMENTS.map((p) => (
                    <SelectItem key={p} value={p}>
                      {t(`intakeRules.placements.${p}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Labeled>
          )}
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

// Edits a list of rules. `rules` are the starting rule objects (remount with a new key to load
// others); every edit calls onChange with the new list. The form keeps its own text, so a comma
// being typed is not tidied away.
export default function IntakeRulesEditor({ rules, onChange }) {
  const { t } = useTranslation();
  const [forms, setForms] = useState(() => rules.map(intakeRuleToForm));
  const emit = (next) => {
    setForms(next);
    onChange(next.map(intakeFormToRule));
  };
  return (
    <div className="space-y-2">
      {forms.map((form, i) => (
        <RuleCard
          key={i}
          index={i}
          total={forms.length}
          form={form}
          onChange={(next) => emit(forms.map((f, j) => (j === i ? next : f)))}
          onMove={(delta) => {
            const next = [...forms];
            [next[i], next[i + delta]] = [next[i + delta], next[i]];
            emit(next);
          }}
          onRemove={() => emit(forms.filter((_, j) => j !== i))}
        />
      ))}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">{t('intakeRules.orderHint')}</p>
        <Button type="button" variant="outline" size="sm" className="rounded-[10px]" onClick={() => emit([...forms, intakeRuleToForm()])}>
          <Plus className="size-4" /> {t('intakeRules.add')}
        </Button>
      </div>
    </div>
  );
}

IntakeRulesEditor.propTypes = { rules: PropTypes.array.isRequired, onChange: PropTypes.func.isRequired };

// Read-only list of rules in words.
export function IntakeRuleList({ rules }) {
  const { t } = useTranslation();
  return (
    <ul className="divide-y divide-border overflow-hidden rounded-[10px] border border-border">
      {rules.map((rule, index) => {
        const { when, then } = describeIntakeRule(t, rule);
        return (
          <li key={`${rule.name}-${index}`} className="grid gap-x-4 gap-y-0.5 px-3 py-2 text-[13px] sm:grid-cols-[12rem_1fr]">
            <span className="truncate" title={intakeRuleName(t, rule.name)}>
              {intakeRuleName(t, rule.name) || '—'}
            </span>
            <span className="min-w-0 text-muted-foreground">
              {when}
              <span aria-hidden> → </span>
              <span className="text-foreground">{then}</span>
            </span>
          </li>
        );
      })}
    </ul>
  );
}

IntakeRuleList.propTypes = { rules: PropTypes.array.isRequired };
