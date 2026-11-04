import { useTranslation } from 'react-i18next';
import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ModelCombobox } from '@/components/ui/model-combobox';

export const MIN_ROUTES = 2;
export const MAX_ROUTES = 16;

// 路由表编辑器：每行 标签 / 描述 / 目标模型，可上下移动与删除；至少 2 条、最多 16 条。
// 标签与描述是给决策模型看的，目标模型用可打字的下拉（建议来自目录，隐藏的模型也能手填）。
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
        <div key={index} className="rounded-md border border-border p-3" data-testid={`route-row-${index}`}>
          <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)]">
            <Input
              aria-label={t('routingPolicy.form.routeLabel')}
              placeholder={t('routingPolicy.form.routeLabelPlaceholder')}
              value={route.label}
              maxLength={64}
              onChange={(e) => update(index, { label: e.target.value })}
            />
            <ModelCombobox
              id={`route-target-${index}`}
              value={route.target_model}
              onChange={(v) => update(index, { target_model: v })}
              options={modelOptions}
              placeholder={t('routingPolicy.form.routeTargetPlaceholder')}
              createLabel={(input) => t('routingPolicy.form.useTyped', { value: input })}
            />
          </div>
          <div className="mt-2 flex items-start gap-2">
            <Input
              aria-label={t('routingPolicy.form.routeDescription')}
              placeholder={t('routingPolicy.form.routeDescriptionPlaceholder')}
              value={route.description}
              onChange={(e) => update(index, { description: e.target.value })}
              className="flex-1"
            />
            <div className="flex shrink-0 items-center gap-1">
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
