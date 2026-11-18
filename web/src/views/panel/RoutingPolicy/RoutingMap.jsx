import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import {
  ArrowDown,
  ArrowUp,
  FileDiff,
  Headset,
  LayoutTemplate,
  Layers,
  PenLine,
  Plus,
  Scale,
  Shapes,
  ListOrdered,
  Split,
  SquareTerminal,
  Trash2
} from 'lucide-react';

import ModelIcon from '@/components/brand/ModelIcon';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { ModelCombobox } from '@/components/ui/model-combobox';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import { ValueFormatter } from 'utils/common';
import { MAX_FALLBACK_MODELS, MAX_ROUTES, MIN_FALLBACK_MODELS, MIN_ROUTES, POLICY_TEMPLATES } from './templates';
import { useClassifierGroups } from './policyFields';

// One icon per template, so the templates read apart at a glance.
export const TEMPLATE_ICONS = {
  coding_change: FileDiff,
  agent_task: SquareTerminal,
  reasoning_depth: Layers,
  content_type: Shapes,
  writing_task: PenLine,
  support_triage: Headset,
  stakes: Scale
};

// TemplateBar: the templates as a row of chips; the applied one is filled.
export function TemplateBar({ active, onApply }) {
  const { t } = useTranslation();
  return (
    <div>
      <div className="mb-2 text-[13px] font-medium text-foreground">{t('routingPolicy.map.startFrom')}</div>
      <div className="flex flex-wrap gap-1.5" data-testid="rp-template-bar">
        {POLICY_TEMPLATES.map((template) => {
          const Icon = TEMPLATE_ICONS[template.id] || LayoutTemplate;
          const on = template.id === active;
          return (
            <button
              key={template.id}
              type="button"
              aria-pressed={on}
              title={t(`routingPolicy.templates.${template.id}.summary`)}
              data-testid={`rp-template-${template.id}`}
              onClick={() => onApply(template)}
              className={cn(
                'inline-flex h-8 items-center gap-1.5 rounded-lg border px-2.5 text-xs transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                on
                  ? 'border-foreground bg-foreground text-background'
                  : 'border-border bg-field text-muted-foreground hover:border-foreground/30 hover:text-foreground'
              )}
            >
              <Icon className="size-4" strokeWidth={1.5} />
              {t(`routingPolicy.templates.${template.id}.short`)}
            </button>
          );
        })}
      </div>
    </div>
  );
}

TemplateBar.propTypes = { active: PropTypes.string, onApply: PropTypes.func.isRequired };

// TemplateMenu: the same templates as a menu, for a policy that already exists.
export function TemplateMenu({ onApply }) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="outline" size="sm" className="h-8 rounded-lg px-2.5 text-xs font-normal">
          <LayoutTemplate className="size-4" strokeWidth={1.5} />
          {t('routingPolicy.form.useTemplate')}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {POLICY_TEMPLATES.map((template) => {
          const Icon = TEMPLATE_ICONS[template.id] || LayoutTemplate;
          return (
            <DropdownMenuItem key={template.id} onClick={() => onApply(template)}>
              <Icon className="size-4 text-muted-foreground" strokeWidth={1.5} />
              {t(`routingPolicy.templates.${template.id}.title`)}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

TemplateMenu.propTypes = { onApply: PropTypes.func.isRequired };

// priceText: a model's price per million tokens, "in / out"; input only when output is free
// (decision models are charged on input).
export function priceText(price, t) {
  if (!price || price.type !== 'tokens') return '';
  const input = ValueFormatter(price.input, true, true);
  if (!price.output) return t('routingPolicy.map.priceInput', { input });
  return t('routingPolicy.map.price', { input, output: ValueFormatter(price.output, true, true) });
}

// ModelPick: a model picker with the model's logo in front and its price under it.
function ModelPick({ id, value, onChange, options, groups, placeholder, prices, ariaLabel }) {
  const { t } = useTranslation();
  const text = priceText(prices?.[value], t);
  return (
    <div className="min-w-0">
      <div className="relative" aria-label={ariaLabel}>
        {value && <ModelIcon model={value} className="pointer-events-none absolute left-2.5 top-1/2 z-10 size-4 -translate-y-1/2" />}
        <ModelCombobox
          id={id}
          value={value || ''}
          onChange={onChange}
          options={options}
          groups={groups}
          placeholder={placeholder}
          inputClassName={cn('font-mono text-[13px]', value && 'pl-8')}
          createLabel={(input) => t('routingPolicy.form.useTyped', { value: input })}
        />
      </div>
      {text && <div className="mt-1 px-1 text-[11px] tabular-nums text-muted-foreground">{text}</div>}
    </div>
  );
}

ModelPick.propTypes = {
  id: PropTypes.string.isRequired,
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired,
  options: PropTypes.array,
  groups: PropTypes.array,
  placeholder: PropTypes.string,
  prices: PropTypes.object,
  ariaLabel: PropTypes.string
};

const label = 'mb-1.5 block text-xs text-muted-foreground';
const card = 'rounded-xl border border-border bg-background/60';

// RoutingMap edits a policy as the routing it describes: the decision model and its question
// at the top, one card per route hanging off a line below it, and the fallback at the end of
// the line. Each model shows its logo and price, so the cost of every route is in view.
// `decisionExtra` renders under the question (the scoring field in the policy panel).
export default function RoutingMap({ idPrefix = 'rp', form, setForm, modelOptions = [], classifiers, prices, decisionExtra }) {
  const { t } = useTranslation();
  const routes = form.routes || [];
  const set = (key) => (value) => setForm((f) => ({ ...f, [key]: value }));
  const setRoutes = (next) => setForm((f) => ({ ...f, routes: next }));
  const update = (index, patch) => setRoutes(routes.map((r, i) => (i === index ? { ...r, ...patch } : r)));
  const move = (index, delta) => {
    const target = index + delta;
    if (target < 0 || target >= routes.length) return;
    const next = [...routes];
    [next[index], next[target]] = [next[target], next[index]];
    setRoutes(next);
  };
  const decisionGroups = useClassifierGroups(classifiers);
  const threshold = Math.round(Number(form.threshold ?? 0.5) * 100);

  return (
    <div data-testid="rp-routing-map">
      <section className={cn(card, 'p-4')}>
        <div className="mb-3 flex items-center gap-1.5 text-xs text-muted-foreground">
          <Split className="size-3.5" strokeWidth={1.5} />
          {t('routingPolicy.map.decision')}
        </div>
        <label className={label} htmlFor={`${idPrefix}-decision_model`}>
          {t('routingPolicy.form.decisionModel')}
        </label>
        <ModelPick
          id={`${idPrefix}-decision_model`}
          value={form.decision_model}
          onChange={set('decision_model')}
          groups={decisionGroups}
          placeholder={t('routingPolicy.form.modelPlaceholder')}
          prices={prices}
        />
        <label className={cn(label, 'mt-3')} htmlFor={`${idPrefix}-question`}>
          {t('routingPolicy.form.question')}
        </label>
        <Textarea
          id={`${idPrefix}-question`}
          rows={3}
          placeholder={t('routingPolicy.map.questionPlaceholder')}
          value={form.question || ''}
          onChange={(e) => set('question')(e.target.value)}
        />
        {decisionExtra}
        <p className="mt-3 text-xs text-muted-foreground">{t('routingPolicy.map.decisionNote')}</p>
      </section>

      <div className="relative ml-5 pl-8 pt-3">
        <div className="relative">
          <span aria-hidden="true" className="absolute -left-8 -top-3 bottom-0 w-px bg-foreground/15" />
          <ol className="space-y-2">
            {routes.map((route, index) => (
              <li key={index} className="relative" data-testid={`route-row-${index}`}>
                <span aria-hidden="true" className="absolute -left-8 top-[22px] h-px w-8 bg-foreground/15" />
                <span
                  aria-hidden="true"
                  className="absolute -left-8 top-3 z-10 flex size-5 -translate-x-1/2 items-center justify-center rounded-md border border-foreground/20 bg-popover text-[11px] tabular-nums text-muted-foreground"
                >
                  {index + 1}
                </span>
                <div className={cn(card, 'grid gap-2 p-2 sm:grid-cols-[minmax(0,1fr)_224px]')}>
                  <div className="flex items-center justify-between gap-2 sm:col-span-2">
                    <Input
                      aria-label={t('routingPolicy.form.routeLabel')}
                      placeholder={`route_${index + 1}`}
                      value={route.label}
                      maxLength={64}
                      className="h-7 w-44 border-transparent bg-transparent px-1 font-mono text-xs shadow-none hover:border-border focus-visible:border-border"
                      onChange={(e) => update(index, { label: e.target.value })}
                    />
                    <div className="flex shrink-0 items-center">
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-7"
                        aria-label={t('routingPolicy.form.moveUp')}
                        disabled={index === 0}
                        onClick={() => move(index, -1)}
                      >
                        <ArrowUp className="size-3.5" />
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-7"
                        aria-label={t('routingPolicy.form.moveDown')}
                        disabled={index === routes.length - 1}
                        onClick={() => move(index, 1)}
                      >
                        <ArrowDown className="size-3.5" />
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="size-7 text-destructive hover:text-destructive"
                        aria-label={t('routingPolicy.form.removeRoute')}
                        disabled={routes.length <= MIN_ROUTES}
                        onClick={() => setRoutes(routes.filter((_, i) => i !== index))}
                      >
                        <Trash2 className="size-3.5" />
                      </Button>
                    </div>
                  </div>
                  <Textarea
                    aria-label={t('routingPolicy.form.routeDescription')}
                    placeholder={t('routingPolicy.form.routeDescriptionPlaceholder')}
                    value={route.description}
                    rows={2}
                    className="min-h-[60px] resize-none text-[13px] [field-sizing:content]"
                    onChange={(e) => update(index, { description: e.target.value })}
                  />
                  <ModelPick
                    id={`${idPrefix}-route-target-${index}`}
                    value={route.target_model}
                    onChange={(v) => update(index, { target_model: v })}
                    options={modelOptions}
                    placeholder={t('routingPolicy.form.routeTargetPlaceholder')}
                    prices={prices}
                  />
                </div>
              </li>
            ))}
          </ol>
          <button
            type="button"
            disabled={routes.length >= MAX_ROUTES}
            onClick={() => setRoutes([...routes, { label: '', description: '', target_model: '' }])}
            className="mt-2 flex h-9 w-full items-center gap-1.5 rounded-xl border border-dashed border-border px-3 text-[13px] text-muted-foreground transition-colors hover:border-foreground/30 hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
          >
            <Plus className="size-4" />
            {t('routingPolicy.form.addRoute')}
          </button>
        </div>

        <div className="relative mt-2">
          <span aria-hidden="true" className="absolute -left-8 -top-2 h-[30px] border-l border-dashed border-foreground/25" />
          <span aria-hidden="true" className="absolute -left-8 top-[22px] w-8 border-t border-dashed border-foreground/25" />
          <span
            aria-hidden="true"
            className="absolute -left-8 top-3 z-10 flex h-5 -translate-x-1/2 items-center rounded-md border border-foreground/20 bg-popover px-1 text-[11px] tabular-nums text-muted-foreground"
          >
            &lt;{threshold}%
          </span>
          <section className="rounded-xl border border-dashed border-border p-3" data-testid="rp-fallback">
            <div className="mb-2 text-xs text-muted-foreground">{t('routingPolicy.map.fallback')}</div>
            <div className="grid items-start gap-3 sm:grid-cols-[minmax(0,1fr)_224px]">
              <div>
                <label className={label} htmlFor={`${idPrefix}-threshold`}>
                  {t('routingPolicy.map.threshold', { threshold: `${threshold}%` })}
                </label>
                <input
                  id={`${idPrefix}-threshold`}
                  type="range"
                  min={0}
                  max={100}
                  step={5}
                  value={threshold}
                  onChange={(e) => set('threshold')(Number(e.target.value) / 100)}
                  className="h-9 w-full cursor-pointer accent-foreground"
                />
              </div>
              <div>
                <label className={label} htmlFor={`${idPrefix}-fallback_model`}>
                  {t('routingPolicy.form.fallbackModel')}
                </label>
                <ModelPick
                  id={`${idPrefix}-fallback_model`}
                  value={form.fallback_model}
                  onChange={set('fallback_model')}
                  options={modelOptions}
                  placeholder={t('routingPolicy.form.modelPlaceholder')}
                  prices={prices}
                />
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}

RoutingMap.propTypes = {
  idPrefix: PropTypes.string,
  form: PropTypes.object.isRequired,
  setForm: PropTypes.func.isRequired,
  modelOptions: PropTypes.array,
  classifiers: PropTypes.shape({ decision: PropTypes.array, chat: PropTypes.array }),
  prices: PropTypes.object,
  decisionExtra: PropTypes.node
};

// PolicyTypeSwitch: auto routing (a decision model picks the route) or fallback (models in order).
export function PolicyTypeSwitch({ value, onChange }) {
  const { t } = useTranslation();
  const options = [
    { value: 'auto', icon: Split },
    { value: 'fallback', icon: ListOrdered }
  ];
  return (
    <div role="radiogroup" aria-label={t('routingPolicy.type.label')} className="grid grid-cols-2 gap-2">
      {options.map(({ value: v, icon: Icon }) => {
        const on = (value || 'auto') === v;
        return (
          <button
            key={v}
            type="button"
            role="radio"
            aria-checked={on}
            data-testid={`rp-type-${v}`}
            onClick={() => onChange(v)}
            className={cn(
              'flex items-start gap-3 rounded-xl border p-3 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
              on ? 'border-foreground bg-background/60' : 'border-border bg-field hover:border-foreground/30'
            )}
          >
            <span
              className={cn(
                'flex size-8 shrink-0 items-center justify-center rounded-lg border',
                on ? 'border-foreground bg-foreground text-background' : 'border-border text-muted-foreground'
              )}
            >
              <Icon className="size-4" strokeWidth={1.5} />
            </span>
            <span className="min-w-0">
              <span className="block text-[13px] font-medium text-foreground">{t(`routingPolicy.type.${v}`)}</span>
              <span className="mt-0.5 block text-xs text-muted-foreground">{t(`routingPolicy.type.${v}Hint`)}</span>
            </span>
          </button>
        );
      })}
    </div>
  );
}

PolicyTypeSwitch.propTypes = { value: PropTypes.string, onChange: PropTypes.func.isRequired };

// FallbackChain edits a fallback policy's models as the line a request walks down: the first
// model takes every request, and the next one only when every line of the one before has failed.
export function FallbackChain({ idPrefix = 'rp', form, setForm, modelOptions = [], prices }) {
  const { t } = useTranslation();
  const models = form.models?.length ? form.models : ['', ''];
  const setModels = (next) => setForm((f) => ({ ...f, models: next }));
  const move = (index, delta) => {
    const target = index + delta;
    if (target < 0 || target >= models.length) return;
    const next = [...models];
    [next[index], next[target]] = [next[target], next[index]];
    setModels(next);
  };
  return (
    <div data-testid="rp-fallback-chain">
      <p className="mb-3 text-xs text-muted-foreground">{t('routingPolicy.chain.note')}</p>
      <div className="relative ml-5 pl-8">
        <span aria-hidden="true" className="absolute bottom-11 left-0 top-5 w-px bg-foreground/15" />
        <ol className="space-y-2">
          {models.map((name, index) => (
            <li key={index} className="relative" data-testid={`chain-row-${index}`}>
              <span aria-hidden="true" className="absolute -left-8 top-[18px] h-px w-8 bg-foreground/15" />
              <span
                aria-hidden="true"
                className="absolute -left-8 top-2 z-10 flex size-5 -translate-x-1/2 items-center justify-center rounded-md border border-foreground/20 bg-popover text-[11px] tabular-nums text-muted-foreground"
              >
                {index + 1}
              </span>
              <div className={cn(card, 'flex items-start gap-2 p-2')}>
                <div className="min-w-0 flex-1">
                  <ModelPick
                    id={`${idPrefix}-chain-${index}`}
                    value={name}
                    onChange={(v) => setModels(models.map((m, i) => (i === index ? v : m)))}
                    options={modelOptions}
                    placeholder={t(index === 0 ? 'routingPolicy.chain.first' : 'routingPolicy.chain.next')}
                    prices={prices}
                  />
                </div>
                <div className="flex shrink-0 items-center pt-1">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    aria-label={t('routingPolicy.form.moveUp')}
                    disabled={index === 0}
                    onClick={() => move(index, -1)}
                  >
                    <ArrowUp className="size-3.5" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    aria-label={t('routingPolicy.form.moveDown')}
                    disabled={index === models.length - 1}
                    onClick={() => move(index, 1)}
                  >
                    <ArrowDown className="size-3.5" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-7 text-destructive hover:text-destructive"
                    aria-label={t('routingPolicy.chain.remove')}
                    disabled={models.length <= MIN_FALLBACK_MODELS}
                    onClick={() => setModels(models.filter((_, i) => i !== index))}
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
              </div>
            </li>
          ))}
        </ol>
        <button
          type="button"
          disabled={models.length >= MAX_FALLBACK_MODELS}
          onClick={() => setModels([...models, ''])}
          className="mt-2 flex h-9 w-full items-center gap-1.5 rounded-xl border border-dashed border-border px-3 text-[13px] text-muted-foreground transition-colors hover:border-foreground/30 hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
        >
          <Plus className="size-4" />
          {t('routingPolicy.chain.add')}
        </button>
      </div>
    </div>
  );
}

FallbackChain.propTypes = {
  idPrefix: PropTypes.string,
  form: PropTypes.object.isRequired,
  setForm: PropTypes.func.isRequired,
  modelOptions: PropTypes.array,
  prices: PropTypes.object
};
