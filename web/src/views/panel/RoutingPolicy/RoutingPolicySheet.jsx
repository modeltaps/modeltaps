import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { LayoutTemplate } from 'lucide-react';

import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { FormField } from '@/components/ui/form-field';
import { ModelCombobox } from '@/components/ui/model-combobox';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { showError, showSuccess } from 'utils/common';
import RoutesEditor, { MIN_ROUTES } from './RoutesEditor';
import TryItPanel from './TryItPanel';
import { POLICY_TEMPLATES, applyTemplate } from './templates';
import {
  EMPTY_POLICY,
  createRoutingPolicy,
  getRoutingPolicy,
  listCatalogModelNames,
  normalizePolicy,
  updateRoutingPolicy
} from './routingPolicyApi';

// 新建 / 编辑策略的抽屉。字段多且跨「基础 / 高级 / 试一试」三个关注点，按约定用 Tab 抽屉，
// 必填全部在第一个 Tab；提交缺必填时跳回第一个 Tab 并提示。
export default function RoutingPolicySheet({ open, policyId, onClose, onSaved }) {
  const { t } = useTranslation();
  const [form, setForm] = useState(EMPTY_POLICY);
  const [activeTab, setActiveTab] = useState('basic');
  const [submitting, setSubmitting] = useState(false);
  const [modelOptions, setModelOptions] = useState([]);
  const isEdit = Boolean(policyId);

  const set = (key, value) => setForm((prev) => ({ ...prev, [key]: value }));

  useEffect(() => {
    if (!open) return;
    setActiveTab('basic');
    if (policyId) {
      (async () => {
        try {
          const { success, message, data } = await getRoutingPolicy(policyId);
          if (success) setForm({ ...EMPTY_POLICY, ...data, routes: data.routes?.length ? data.routes : EMPTY_POLICY.routes });
          else showError(message);
        } catch (e) {
          // surfaced globally
        }
      })();
    } else {
      setForm(EMPTY_POLICY);
    }
  }, [open, policyId]);

  // 目录模型名只在抽屉打开时取一次，供三处下拉建议。
  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    listCatalogModelNames()
      .then((names) => {
        if (!cancelled) setModelOptions(names);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [open]);

  const normalized = useMemo(() => normalizePolicy(form), [form]);

  // 前端只拦最明显的缺项，细规则（名字格式、重复标签、嵌套策略）交给后端的稳定错误码。
  const missing = () => {
    if (!normalized.name) return 'name';
    if (!normalized.decision_model) return 'decision_model';
    if (!normalized.fallback_model) return 'fallback_model';
    const routes = normalized.routes;
    if (routes.length < MIN_ROUTES || routes.some((r) => !r.label || !r.target_model)) return 'routes';
    return '';
  };

  const submit = async () => {
    const field = missing();
    if (field) {
      setActiveTab('basic');
      showError(t(`routingPolicy.form.missing.${field}`));
      return;
    }
    setSubmitting(true);
    try {
      const { success, message } = isEdit ? await updateRoutingPolicy(policyId, normalized) : await createRoutingPolicy(normalized);
      if (success) {
        showSuccess(t('userPage.saveSuccess'));
        onSaved?.();
      } else {
        showError(message);
      }
    } catch (e) {
      // surfaced globally
    } finally {
      setSubmitting(false);
    }
  };

  const modelField = (key, label, { required, hint, help } = {}) => (
    <FormField id={`rp-${key}`} label={label} required={required} hint={hint} help={help}>
      <ModelCombobox
        id={`rp-${key}`}
        value={form[key] || ''}
        onChange={(v) => set(key, v)}
        options={modelOptions}
        placeholder={t('routingPolicy.form.modelPlaceholder')}
        createLabel={(input) => t('routingPolicy.form.useTyped', { value: input })}
      />
    </FormField>
  );

  const numberField = (key, label, { hint, help, min, max, step } = {}) => (
    <FormField id={`rp-${key}`} label={label} hint={hint} help={help} row>
      <Input
        id={`rp-${key}`}
        type="number"
        min={min}
        max={max}
        step={step}
        value={form[key] ?? ''}
        onChange={(e) => set(key, e.target.value)}
      />
    </FormField>
  );

  return (
    <Sheet open={open} onOpenChange={(o) => !o && onClose?.()}>
      <SheetContent onClose={onClose} className="max-w-3xl">
        <SheetHeader>
          <SheetTitle>{isEdit ? t('routingPolicy.editTitle') : t('routingPolicy.createTitle')}</SheetTitle>
        </SheetHeader>
        <SheetBody>
          <Tabs value={activeTab} onValueChange={setActiveTab}>
            <TabsList>
              <TabsTrigger value="basic">{t('routingPolicy.form.tabBasic')}</TabsTrigger>
              <TabsTrigger value="advanced">{t('routingPolicy.form.tabAdvanced')}</TabsTrigger>
              <TabsTrigger value="try">{t('routingPolicy.form.tabTry')}</TabsTrigger>
            </TabsList>

            <TabsContent value="basic" className="space-y-5">
              <FormField
                id="rp-name"
                label={t('routingPolicy.form.name')}
                required
                hint={t('routingPolicy.form.nameHint')}
                help={t('routingPolicy.form.nameHelp')}
              >
                <Input id="rp-name" value={form.name} placeholder="auto/coder" onChange={(e) => set('name', e.target.value)} />
              </FormField>
              <FormField id="rp-display-name" label={t('routingPolicy.form.displayName')} hint={t('routingPolicy.form.displayNameHint')}>
                <Input
                  id="rp-display-name"
                  value={form.display_name}
                  maxLength={100}
                  onChange={(e) => set('display_name', e.target.value)}
                />
              </FormField>
              <FormField id="rp-description" label={t('routingPolicy.form.description')}>
                <Textarea id="rp-description" rows={2} value={form.description} onChange={(e) => set('description', e.target.value)} />
              </FormField>
              <div className="flex items-center justify-between">
                <div>
                  <Label htmlFor="rp-enabled">{t('routingPolicy.form.enabled')}</Label>
                  <p className="text-xs text-muted-foreground">{t('routingPolicy.form.enabledHint')}</p>
                </div>
                <Switch id="rp-enabled" checked={Boolean(form.enabled)} onCheckedChange={(v) => set('enabled', v)} />
              </div>

              <div className="border-t border-border/60 pt-5" />

              {modelField('decision_model', t('routingPolicy.form.decisionModel'), {
                required: true,
                hint: t('routingPolicy.form.decisionModelHint')
              })}
              <FormField
                id="rp-question"
                label={t('routingPolicy.form.question')}
                hint={t('routingPolicy.form.questionHint')}
                help={t('routingPolicy.form.questionHelp')}
              >
                <Textarea id="rp-question" rows={3} value={form.question} onChange={(e) => set('question', e.target.value)} />
              </FormField>

              <FormField
                id="rp-routes"
                label={t('routingPolicy.form.routes')}
                required
                hint={t('routingPolicy.form.routesHint')}
                help={t('routingPolicy.form.routesHelp')}
              >
                <div className="space-y-3">
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button type="button" variant="outline" size="sm">
                        <LayoutTemplate className="mr-1 size-4" />
                        {t('routingPolicy.form.useTemplate')}
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="start">
                      {POLICY_TEMPLATES.map((template) => (
                        <DropdownMenuItem key={template.id} onClick={() => setForm((prev) => applyTemplate(prev, template))}>
                          {t(`routingPolicy.templates.${template.id}`)}
                        </DropdownMenuItem>
                      ))}
                    </DropdownMenuContent>
                  </DropdownMenu>
                  <RoutesEditor routes={form.routes} onChange={(routes) => set('routes', routes)} modelOptions={modelOptions} />
                </div>
              </FormField>

              {modelField('fallback_model', t('routingPolicy.form.fallbackModel'), {
                required: true,
                hint: t('routingPolicy.form.fallbackModelHint')
              })}
              {numberField('threshold', t('routingPolicy.form.threshold'), {
                hint: t('routingPolicy.form.thresholdHint'),
                help: t('routingPolicy.form.thresholdHelp'),
                min: 0,
                max: 1,
                step: 0.05
              })}
            </TabsContent>

            <TabsContent value="advanced" className="space-y-5">
              {numberField('pin_ttl_seconds', t('routingPolicy.form.pinTtl'), {
                hint: t('routingPolicy.form.pinTtlHint'),
                help: t('routingPolicy.form.pinTtlHelp'),
                min: 0,
                max: 604800,
                step: 60
              })}
              {numberField('max_input_chars', t('routingPolicy.form.maxInputChars'), {
                hint: t('routingPolicy.form.maxInputCharsHint'),
                min: 0,
                max: 200000,
                step: 100
              })}
              {numberField('decision_timeout_ms', t('routingPolicy.form.decisionTimeout'), {
                hint: t('routingPolicy.form.decisionTimeoutHint'),
                min: 0,
                max: 60000,
                step: 100
              })}
            </TabsContent>

            <TabsContent value="try">
              <TryItPanel policy={normalized} modelOptions={modelOptions} />
            </TabsContent>
          </Tabs>
        </SheetBody>
        <SheetFooter>
          <Button variant="outline" onClick={onClose}>
            {t('userPage.cancel')}
          </Button>
          <Button disabled={submitting} onClick={submit}>
            {t('userPage.submit')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
