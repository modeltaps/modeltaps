import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Eye, Loader2, RotateCcw } from 'lucide-react';

import { API } from 'utils/api';
import { showError } from 'utils/common';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { FormField } from '@/components/ui/form-field';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import OptionCard from './OptionCard';
import useMailStatus from '../mail/useMailStatus';
import { TEMPLATE_IDS, isTemplateCustom, resetTemplate, setTemplateField, templateText } from '../mail/mailTemplates';
import i18nList from 'i18n/i18nList';

// ==============================|| PLATFORM SETTINGS — USER EMAIL TEMPLATES ||============================== //
// The emails users get (verification code, password reset, the two quota warnings), in each of the
// console's languages: subject and HTML body, with {{variables}}. Each user gets the language they
// picked in the console (or their browser's, for emails sent while they are on the page), else the
// site default. The shared header and footer are added by the server; the preview shows the
// finished email with sample values.

export function MailTemplatesCard({ ctx }) {
  const { t } = useTranslation();
  const m = (k, o) => t(`settingsUi.platform.mailTemplates.${k}`, o);
  const { inputs, setField } = ctx;
  const { status } = useMailStatus();
  const [active, setActive] = useState(TEMPLATE_IDS[0]);
  const [picked, setPicked] = useState('');
  const [preview, setPreview] = useState(null);
  const [loading, setLoading] = useState(false);

  const spec = status?.templates?.find((tpl) => tpl.id === active);
  const raw = inputs.MailTemplates;
  // The editor opens on the site default language: the one most users get.
  const siteLang = inputs.DefaultLanguage || status?.default_language || 'en_US';
  const lng = picked || siteLang;
  const defaults = spec?.defaults?.[lng] || { subject: '', body: '' };
  const subject = templateText(raw, active, lng, 'subject', defaults.subject, siteLang);
  const body = templateText(raw, active, lng, 'body', defaults.body, siteLang);
  const custom = isTemplateCustom(raw, active, lng, siteLang);

  const write = (field, value) => setField('MailTemplates', setTemplateField(raw, active, lng, field, value, defaults[field], siteLang));

  const openPreview = async () => {
    setLoading(true);
    try {
      const res = await API.post('/api/option/mail/preview', { template: active, language: lng, subject, body });
      if (res.data?.success) setPreview(res.data.data);
      else showError(res.data?.message);
    } catch {
      /* surfaced by the API interceptor */
    } finally {
      setLoading(false);
    }
  };

  return (
    <OptionCard ctx={ctx} id="mail-templates">
      <Tabs value={active} onValueChange={setActive}>
        <TabsList>
          {TEMPLATE_IDS.map((id) => (
            <TabsTrigger key={id} value={id}>
              {m(`names.${id}`)}
              {isTemplateCustom(raw, id, undefined, siteLang) && (
                <span className="ml-1.5 size-1.5 rounded-full bg-primary" aria-label={m('customized')} />
              )}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

      <div className="flex flex-wrap items-center gap-1.5" role="radiogroup" aria-label={m('language')}>
        <span className="mr-1 text-xs text-muted-foreground">{m('language')}</span>
        {i18nList.map((item) => {
          const current = item.lng === lng;
          const changed = isTemplateCustom(raw, active, item.lng, siteLang);
          return (
            <button
              key={item.lng}
              type="button"
              role="radio"
              aria-checked={current}
              onClick={() => setPicked(item.lng)}
              className={cn(
                'inline-flex h-7 items-center gap-1.5 rounded-md border px-2.5 text-xs outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring/50',
                current ? 'border-foreground bg-field font-medium' : 'border-border text-muted-foreground hover:bg-field'
              )}
            >
              {item.name}
              {item.lng === siteLang && <span className="text-muted-foreground">· {m('siteDefault')}</span>}
              <span className={cn('size-1.5 rounded-full', changed ? 'bg-primary' : 'bg-transparent')} aria-hidden />
            </button>
          );
        })}
        <span className="ml-auto text-xs text-muted-foreground">{custom ? m('customized') : m('builtin')}</span>
      </div>

      <FormField id="mail-template-subject" label={m('subject')}>
        <Input id="mail-template-subject" value={subject} onChange={(e) => write('subject', e.target.value)} disabled={!spec} />
      </FormField>
      <FormField id="mail-template-body" label={m('body')} hint={m('bodyHint')}>
        <Textarea
          id="mail-template-body"
          value={body}
          onChange={(e) => write('body', e.target.value)}
          rows={9}
          className="font-mono text-xs leading-relaxed"
          disabled={!spec}
        />
      </FormField>

      <div className="flex flex-wrap items-center gap-2">
        <span className="text-xs text-muted-foreground">{m('variables')}</span>
        {(spec?.variables || []).map((v) => (
          <code key={v} className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs" title={m(`vars.${v}`)}>
            {`{{${v}}}`}
          </code>
        ))}
        <div className="ml-auto flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setField('MailTemplates', resetTemplate(raw, active, lng, siteLang))}
            disabled={!custom}
          >
            <RotateCcw className="size-4" />
            {m('reset')}
          </Button>
          <Button variant="outline" size="sm" onClick={openPreview} disabled={!spec || loading}>
            {loading ? <Loader2 className="size-4 animate-spin" /> : <Eye className="size-4" />}
            {m('preview')}
          </Button>
        </div>
      </div>

      <Dialog open={!!preview} onOpenChange={(open) => !open && setPreview(null)}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {m(`names.${active}`)} · {i18nList.find((item) => item.lng === lng)?.name}
            </DialogTitle>
            <DialogDescription>
              {m('subject')}: {preview?.subject}
            </DialogDescription>
          </DialogHeader>
          <DialogBody>
            <iframe
              title={m('preview')}
              srcDoc={preview?.html || ''}
              sandbox=""
              className="h-[28rem] w-full rounded-md border border-border bg-white"
            />
          </DialogBody>
        </DialogContent>
      </Dialog>
    </OptionCard>
  );
}

MailTemplatesCard.propTypes = { ctx: PropTypes.object.isRequired };
