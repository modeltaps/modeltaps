import { useTranslation } from 'react-i18next';
import { ArrowDown, ArrowRight, ArrowUp, Plus, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { ModelCombobox } from '@/components/ui/model-combobox';

export const MIN_ROUTES = 2;
export const MAX_ROUTES = 16;

// Route table editor. Each row leads with the description (what the decision model reads to tell
// the routes apart), then an optional short name for logs and the target model. Rows move up and
// down and can be removed; 2 to 16 routes. Targets use a typeable picker (hidden models can be typed).
export default function RoutesEditor({ routes, onChange, modelOptions = [] }) {
  const { t } = useTranslation();

  const update = (index, patch) => onChange(routes.map((r, i) => (i === index ? { ...r, ...patch } : r)));
  const remove = (index) => onChange(routes.filter((_, i) => i !== index));
  const move = (index, delta) => {
    const target = index + delta;
    if (target < 0 || target >= routes.length) return;
    const next = [...routes];
    [next[index], next[target]] = [next[target], next[index]];
    onChange(next);
  };
  const add = () => onChange([...routes, { label: '', description: '', target_model: '' }]);

  return (
    <div className="space-y-3">
      {routes.map((route, index) => (
        <div key={index} className="rounded-lg border border-border p-3" data-testid={`route-row-${index}`}>
          <Textarea
            aria-label={t('routingPolicy.form.routeDescription')}
            placeholder={t('routingPolicy.form.routeDescriptionPlaceholder')}
            value={route.description}
            rows={2}
            className="min-h-0 resize-none"
            onChange={(e) => update(index, { description: e.target.value })}
          />
          <div className="mt-2 flex items-center gap-2">
            <Input
              aria-label={t('routingPolicy.form.routeLabel')}
              placeholder={t('routingPolicy.form.routeLabelPlaceholder')}
              value={route.label}
              maxLength={64}
              className="w-32 shrink-0 font-mono text-[13px]"
              onChange={(e) => update(index, { label: e.target.value })}
            />
            <ArrowRight className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <div className="min-w-0 flex-1">
              <ModelCombobox
                id={`route-target-${index}`}
                value={route.target_model}
                onChange={(v) => update(index, { target_model: v })}
                options={modelOptions}
                placeholder={t('routingPolicy.form.routeTargetPlaceholder')}
                createLabel={(input) => t('routingPolicy.form.useTyped', { value: input })}
              />
            </div>
            <div className="flex shrink-0 items-center">
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={t('routingPolicy.form.moveUp')}
                disabled={index === 0}
                onClick={() => move(index, -1)}
              >
                <ArrowUp className="size-4" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={t('routingPolicy.form.moveDown')}
                disabled={index === routes.length - 1}
                onClick={() => move(index, 1)}
              >
                <ArrowDown className="size-4" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={t('routingPolicy.form.removeRoute')}
                disabled={routes.length <= MIN_ROUTES}
                onClick={() => remove(index)}
                className="text-destructive hover:text-destructive"
              >
                <Trash2 className="size-4" />
              </Button>
            </div>
          </div>
        </div>
      ))}
      <Button type="button" variant="outline" size="sm" onClick={add} disabled={routes.length >= MAX_ROUTES}>
        <Plus className="mr-1 size-4" />
        {t('routingPolicy.form.addRoute')}
      </Button>
    </div>
  );
}
