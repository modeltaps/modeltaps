import { useMemo } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import ModelIcon from '@/components/brand/ModelIcon';
import { cn } from '@/lib/utils';
import { EMPTY_POLICY } from './routingPolicyApi';
import { POLICY_TEMPLATES, TEMPLATE_GROUPS, TIERS, applyTemplate, templateName } from './templates';

// TierPips: three short bars, as many filled as the tier is strong (fast 1, balanced 2, strongest 3).
export function TierPips({ tier }) {
  const { t } = useTranslation();
  const level = TIERS.indexOf(tier) + 1;
  return (
    <span
      className="inline-flex items-end gap-0.5"
      role="img"
      aria-label={t(`routingPolicy.tier.${tier}`)}
      title={t(`routingPolicy.tier.${tier}`)}
    >
      {TIERS.map((_, i) => (
        <span
          key={i}
          className={cn('w-[3px] rounded-[1px]', i < level ? 'bg-foreground/70' : 'bg-foreground/15')}
          style={{ height: 6 + i * 3 }}
        />
      ))}
    </span>
  );
}

TierPips.propTypes = { tier: PropTypes.oneOf(TIERS).isRequired };

function ModelName({ model }) {
  const { t } = useTranslation();
  if (!model) return <span className="text-xs text-muted-foreground">{t('routingPolicy.new.unresolved')}</span>;
  return (
    <span className="flex min-w-0 items-center gap-1.5">
      <ModelIcon model={model} className="size-4 shrink-0" />
      <span className="truncate font-mono text-xs text-foreground">{model}</span>
    </span>
  );
}

ModelName.propTypes = { model: PropTypes.string };

// TemplatePreview: how the picked template would route on this instance. The first user message
// goes to the decision model, each route sends its kind of request to the model of its tier, and
// the fallback takes the rest. Models are the ones the template would fill in.
function TemplatePreview({ template, suggest, callPrefix }) {
  const { t } = useTranslation();
  const filled = useMemo(() => applyTemplate(EMPTY_POLICY, template, suggest), [template, suggest]);
  return (
    <div className="min-w-0" data-testid="rp-template-preview">
      <h3 className="text-base font-medium text-foreground">{t(`routingPolicy.templates.${template.id}.title`)}</h3>
      <p className="mt-1 text-[13px] text-muted-foreground">{t(`routingPolicy.templates.${template.id}.summary`)}</p>

      <div className="mt-5 rounded-xl border border-border bg-muted/30">
        <div className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
          <span className="text-xs text-muted-foreground">{t('routingPolicy.new.preview.decision')}</span>
          <ModelName model={filled.decision_model} />
        </div>
        <ol className="relative px-4 py-2">
          <span aria-hidden="true" className="absolute bottom-0 left-[21px] top-0 w-px bg-foreground/15" />
          {template.routes.map((route, index) => (
            <li key={route.label} className="relative flex items-start gap-3 py-2">
              <span
                aria-hidden="true"
                className="relative z-10 mt-[5px] size-[11px] shrink-0 rounded-full border border-foreground/30 bg-popover"
              />
              <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between gap-3">
                  <span className="font-mono text-[13px] text-foreground">{route.label}</span>
                  <span className="flex min-w-0 items-center gap-2">
                    <TierPips tier={route.tier} />
                    <ModelName model={filled.routes[index].target_model} />
                  </span>
                </div>
                <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">{route.description}</p>
              </div>
            </li>
          ))}
        </ol>
        <div className="flex items-center justify-between gap-3 border-t border-dashed border-border px-4 py-3">
          <span className="text-xs text-muted-foreground">{t('routingPolicy.new.preview.fallback', { threshold: '50%' })}</span>
          <ModelName model={filled.fallback_model} />
        </div>
      </div>

      <dl className="mt-5 space-y-3 text-xs">
        <div>
          <dt className="text-muted-foreground">{t('routingPolicy.form.question')}</dt>
          <dd className="mt-1 leading-relaxed text-foreground/80">{template.question}</dd>
        </div>
        <div>
          <dt className="text-muted-foreground">{t('routingPolicy.new.preview.callAs')}</dt>
          <dd className="mt-1 font-mono text-[13px] text-foreground">
            {callPrefix ? `${callPrefix}${template.name}` : templateName(template, callPrefix)}
          </dd>
        </div>
      </dl>
    </div>
  );
}

TemplatePreview.propTypes = {
  template: PropTypes.object.isRequired,
  suggest: PropTypes.object,
  callPrefix: PropTypes.string
};

// TemplatePicker is the first step of a new policy: templates grouped by where they are used on
// the left, and what the selected one would do on this instance on the right. Picking happens in
// the sheet's footer (or by double-clicking a row); a blank policy is offered there too.
export default function TemplatePicker({ selected, onSelect, onPick, suggest, callPrefix }) {
  const { t } = useTranslation();
  const template = POLICY_TEMPLATES.find((x) => x.id === selected) || POLICY_TEMPLATES[0];
  return (
    <div className="grid gap-6 pt-1 sm:grid-cols-[176px_minmax(0,1fr)]" data-testid="rp-template-picker">
      <nav aria-label={t('routingPolicy.new.templates')} className="space-y-5">
        {TEMPLATE_GROUPS.map((group) => (
          <div key={group}>
            <div className="mb-1 px-2 text-xs text-muted-foreground">{t(`routingPolicy.templateGroups.${group}`)}</div>
            <div className="space-y-0.5">
              {POLICY_TEMPLATES.filter((x) => x.group === group).map((x) => (
                <button
                  key={x.id}
                  type="button"
                  aria-pressed={x.id === template.id}
                  data-testid={`rp-template-${x.id}`}
                  onClick={() => onSelect(x.id)}
                  onDoubleClick={() => onPick(x)}
                  className={cn(
                    'flex h-8 w-full items-center rounded-lg px-2 text-left text-[13px] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                    x.id === template.id
                      ? 'bg-muted text-foreground shadow-[inset_0_0_0_1px_var(--color-border)]'
                      : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground'
                  )}
                >
                  <span className="truncate">{t(`routingPolicy.templates.${x.id}.short`)}</span>
                </button>
              ))}
            </div>
          </div>
        ))}
      </nav>
      <TemplatePreview template={template} suggest={suggest} callPrefix={callPrefix} />
    </div>
  );
}

TemplatePicker.propTypes = {
  selected: PropTypes.string,
  onSelect: PropTypes.func.isRequired,
  onPick: PropTypes.func.isRequired,
  suggest: PropTypes.object,
  callPrefix: PropTypes.string
};
