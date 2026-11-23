import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ArrowDown, ArrowUp, Ban, ChevronDown, CircleCheck, CircleOff, Clock, Plus, Trash2, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';
import { Segmented } from '@/components/objects';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

// Intake rules decide what happens to a model a channel's upstream starts listing: switch its
// supply on, leave it for review, add it off, or skip it, and where it joins the model's order.
// The editor works on rule objects as the backend stores them (see model.SupplyIntakeRule).

export const INTAKE_ACTIONS = ['enable', 'pending', 'disable', 'skip'];
export const INTAKE_PLACEMENTS = ['auto', 'policy', 'last'];
export const INTAKE_ENDPOINTS = [
  'chat',
  'responses',
  'realtime',
  'images',
  'audio.speech',
  'audio.transcription',
  'embeddings',
  'rerank',
  'decisions'
];

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

const ACTION_ICONS = { enable: CircleCheck, pending: Clock, disable: CircleOff, skip: Ban };

// The conditions a rule can set, in the order the editor shows them. `vendors` has no editor and
// is kept as it is.
const CONDITIONS = ['endpoints', 'families', 'patterns', 'supplied'];
const conditionSet = (form, key) =>
  key === 'endpoints' ? form.endpoints.length > 0 : key === 'supplied' ? form.supplied !== 'any' : String(form[key]).trim() !== '';
const conditionCleared = (key) => (key === 'endpoints' ? [] : key === 'supplied' ? 'any' : '');

// One rule edited as a sentence: "When a model meets every condition below, then <action>,
// <placement>". Only the conditions in use are shown; others are added from a menu.
function RuleSentenceEditor({ form, onChange }) {
  const { t } = useTranslation();
  const [shown, setShown] = useState(() => CONDITIONS.filter((k) => conditionSet(form, k)));
  const set = (patch) => onChange({ ...form, ...patch });
  const remove = (key) => {
    setShown((list) => list.filter((k) => k !== key));
    set({ [key]: conditionCleared(key) });
  };
  const add = (key) => {
    setShown((list) => [...list, key]);
    if (key === 'supplied') set({ supplied: 'yes' });
  };
  const missing = CONDITIONS.filter((k) => !shown.includes(k));
  const toggleEndpoint = (e) =>
    set({ endpoints: form.endpoints.includes(e) ? form.endpoints.filter((x) => x !== e) : [...form.endpoints, e] });
  const row = (key, control) => (
    <div key={key} className="grid grid-cols-[7rem_1fr_auto] items-start gap-3">
      <span className="pt-1.5 text-xs text-muted-foreground">{t(`intakeRules.editor.${key}`)}</span>
      <div className="min-w-0">{control}</div>
      <Button
        variant="ghost"
        size="icon"
        className="size-7"
        onClick={() => remove(key)}
        aria-label={t('intakeRules.editor.removeCondition')}
      >
        <X className="size-3.5" />
      </Button>
    </div>
  );
  return (
    <div className="space-y-4">
      <label className="grid grid-cols-[7rem_1fr] items-center gap-3">
        <span className="text-xs text-muted-foreground">{t('intakeRules.name')}</span>
        <Input
          value={intakeRuleName(t, form.name)}
          onChange={(e) => set({ name: e.target.value })}
          placeholder={t('intakeRules.namePlaceholder')}
          className="h-8"
        />
      </label>
      <div className="space-y-2">
        <p className="text-xs font-medium">{t(shown.length ? 'intakeRules.editor.when' : 'intakeRules.editor.whenAny')}</p>
        {shown.map((key) => {
          if (key === 'endpoints')
            return row(
              key,
              <div className="flex flex-wrap gap-1.5">
                {INTAKE_ENDPOINTS.map((e) => (
                  <button
                    key={e}
                    type="button"
                    aria-pressed={form.endpoints.includes(e)}
                    onClick={() => toggleEndpoint(e)}
                    className={cn(
                      'h-7 rounded-md border px-2 font-mono text-xs transition-colors',
                      form.endpoints.includes(e)
                        ? 'border-foreground bg-foreground text-background'
                        : 'border-border bg-field hover:bg-muted'
                    )}
                  >
                    {e}
                  </button>
                ))}
              </div>
            );
          if (key === 'supplied')
            return row(
              key,
              <Segmented
                label={t('intakeRules.editor.supplied')}
                value={form.supplied === 'no' ? 'no' : 'yes'}
                onChange={(v) => set({ supplied: v })}
                options={['yes', 'no'].map((v) => ({ value: v, label: t(`intakeRules.tri.${v}`) }))}
              />
            );
          return row(
            key,
            <div className="space-y-1">
              <Input
                value={form[key]}
                onChange={(e) => set({ [key]: e.target.value })}
                placeholder={t(`intakeRules.editor.${key}Placeholder`)}
                className="h-8 font-mono"
              />
              <p className="text-xs text-muted-foreground">{t('intakeRules.editor.globHint')}</p>
            </div>
          );
        })}
        {missing.length > 0 && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="sm" className="h-7 px-2 text-xs">
                <Plus className="size-3.5" />
                {t('intakeRules.editor.addCondition')}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              {missing.map((key) => (
                <DropdownMenuItem key={key} onClick={() => add(key)}>
                  {t(`intakeRules.editor.${key}`)}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-xs font-medium">{t('intakeRules.editor.then')}</span>
        <Select value={form.action} onValueChange={(v) => set({ action: v })}>
          <SelectTrigger className="h-8 w-36" aria-label={t('intakeRules.action')}>
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
        {form.action !== 'skip' && (
          <Select value={form.placement} onValueChange={(v) => set({ placement: v })}>
            <SelectTrigger className="h-8 w-48" aria-label={t('intakeRules.placement')}>
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
        )}
      </div>
    </div>
  );
}

RuleSentenceEditor.propTypes = { form: PropTypes.object.isRequired, onChange: PropTypes.func.isRequired };

// The rules as sentences, tried from the top: what each one looks at, what it does, and (with
// `stats` from the preview API) how many catalog models it would decide, with a few of them. The
// rule `matched` (an index, -1 for the fallback) is marked. The fallback row closes the list.
// With `onChange` the board is the editor: a row opens in place to edit that rule (one at a time),
// rules move and go from the open row, and new ones are added at the end.
export function IntakeRuleBoard({ rules, stats = null, matched = null, onChange = null }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(null);
  // The open rule keeps its own form, so text being typed (a comma) is not tidied away.
  const [form, setForm] = useState(null);
  const editable = typeof onChange === 'function';
  const byIndex = new Map((stats?.rules || []).map((r) => [r.index, r]));
  const rows = [...rules.map((rule, index) => ({ rule, index })), { rule: { action: 'pending' }, index: -1 }];
  const openRule = (index, rule) => {
    setOpen(index);
    setForm(intakeRuleToForm(rule));
  };
  const editRule = (next) => {
    setForm(next);
    onChange(rules.map((r, i) => (i === open ? intakeFormToRule(next) : r)));
  };
  const move = (index, delta) => {
    const next = [...rules];
    [next[index], next[index + delta]] = [next[index + delta], next[index]];
    onChange(next);
    setOpen(index + delta);
  };
  const remove = (index) => {
    onChange(rules.filter((_, i) => i !== index));
    setOpen(null);
  };
  const addRule = () => {
    const rule = intakeFormToRule(intakeRuleToForm());
    onChange([...rules, rule]);
    openRule(rules.length, rule);
  };
  return (
    <ol className="divide-y divide-border overflow-hidden rounded-[10px] border border-border" data-testid="intake-board">
      {rows.map(({ rule, index }) => {
        const fallback = index < 0;
        const { when, then } = describeIntakeRule(t, rule);
        const Icon = ACTION_ICONS[rule.action] || Clock;
        const stat = byIndex.get(index);
        const hit = matched === index;
        const isOpen = editable && open === index;
        const summary = (
          <>
            <span
              className={cn(
                'flex size-6 items-center justify-center rounded-md text-xs tabular-nums',
                hit ? 'bg-foreground text-background' : 'bg-muted text-muted-foreground'
              )}
            >
              {fallback ? '…' : index + 1}
            </span>
            <span className="min-w-0 text-left">
              <span className="block truncate font-medium">
                {fallback ? t('intakeRules.board.fallback') : intakeRuleName(t, rule.name) || t('intakeRules.board.unnamed')}
              </span>
              <span className="flex min-w-0 flex-wrap items-center gap-x-1.5 text-xs text-muted-foreground">
                <span className="truncate">{fallback ? t('intakeRules.board.fallbackWhen') : when}</span>
                <span aria-hidden>→</span>
                <span className="inline-flex items-center gap-1 text-foreground">
                  <Icon className="size-3.5" />
                  {then}
                </span>
              </span>
            </span>
            <span
              className="flex items-center gap-2 text-right text-xs text-muted-foreground"
              title={stat?.examples?.join('\n') || undefined}
            >
              {stat ? t('intakeRules.board.count', { count: stat.count }) : ''}
              {editable && !fallback && <ChevronDown className={cn('size-4 transition-transform', isOpen && 'rotate-180')} />}
            </span>
          </>
        );
        const rowClass = cn('grid w-full grid-cols-[1.5rem_1fr_auto] items-center gap-x-3 px-3 py-2.5 text-[13px]', hit && 'bg-muted');
        return (
          <li key={`${index}`} data-testid="intake-board-row" aria-current={hit || undefined}>
            {editable && !fallback ? (
              <button
                type="button"
                aria-expanded={isOpen}
                className={cn(rowClass, 'transition-colors hover:bg-muted/60')}
                onClick={() => (isOpen ? setOpen(null) : openRule(index, rule))}
              >
                {summary}
              </button>
            ) : (
              <div className={rowClass}>{summary}</div>
            )}
            {isOpen && form && (
              <div className="space-y-4 border-t border-border bg-muted/20 px-4 py-4 sm:pl-12">
                <RuleSentenceEditor key={index} form={form} onChange={editRule} />
                <div className="flex flex-wrap items-center gap-1 border-t border-border pt-3">
                  <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" disabled={index === 0} onClick={() => move(index, -1)}>
                    <ArrowUp className="size-3.5" />
                    {t('intakeRules.up')}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2 text-xs"
                    disabled={index === rules.length - 1}
                    onClick={() => move(index, 1)}
                  >
                    <ArrowDown className="size-3.5" />
                    {t('intakeRules.down')}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2 text-xs text-destructive hover:text-destructive"
                    onClick={() => remove(index)}
                  >
                    <Trash2 className="size-3.5" />
                    {t('intakeRules.remove')}
                  </Button>
                  <span className="flex-1" />
                  <Button variant="outline" size="sm" className="h-7" onClick={() => setOpen(null)}>
                    {t('intakeRules.editor.done')}
                  </Button>
                </div>
              </div>
            )}
          </li>
        );
      })}
      {editable && (
        <li>
          <button
            type="button"
            onClick={addRule}
            className="flex w-full items-center gap-2 px-3 py-2.5 text-[13px] text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
          >
            <Plus className="size-4" />
            {t('intakeRules.add')}
          </button>
        </li>
      )}
    </ol>
  );
}

IntakeRuleBoard.propTypes = {
  rules: PropTypes.array.isRequired,
  stats: PropTypes.object,
  matched: PropTypes.number,
  onChange: PropTypes.func
};
