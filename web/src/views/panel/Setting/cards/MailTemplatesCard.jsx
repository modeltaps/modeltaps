import { useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Eye, Loader2, RotateCcw } from 'lucide-react';

import { API } from 'utils/api';
import { showError } from 'utils/common';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { FormField } from '@/components/ui/form-field';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import OptionCard from './OptionCard';
import useMailStatus from '../mail/useMailStatus';
import { TEMPLATE_IDS, isTemplateCustom, resetTemplate, setTemplateField, templateText } from '../mail/mailTemplates';

// ==============================|| PLATFORM SETTINGS — USER EMAIL TEMPLATES ||============================== //
// The emails users get (verification code, password reset, quota warning): subject and HTML body,
// with {{variables}}. The shared header and footer are added by the server; the preview shows the
// finished email with sample values.

export function MailTemplatesCard({ ctx }) {
  const { t } = useTranslation();
  const m = (k, o) => t(`settingsUi.platform.mailTemplates.${k}`, o);
  const { inputs, setField } = ctx;
  const { status } = useMailStatus();
  const [active, setActive] = useState(TEMPLATE_IDS[0]);
  const [preview, setPreview] = useState(null);
  const [loading, setLoading] = useState(false);

  const spec = status?.templates?.find((tpl) => tpl.id === active);
  const raw = inputs.MailTemplates;
  const defaults = spec?.default || { subject: '', body: '' };
  const subject = templateText(raw, active, 'subject', defaults.subject);
  const body = templateText(raw, active, 'body', defaults.body);
  const custom = isTemplateCustom(raw, active);

  const write = (field, value) => setField('MailTemplates', setTemplateField(raw, active, field, value, defaults[field]));

  const openPreview = async () => {
    setLoading(true);
    try {
      const res = await API.post('/api/option/mail/preview', { template: active, subject, body });
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
              {isTemplateCustom(raw, id) && <span className="ml-1.5 size-1.5 rounded-full bg-primary" aria-label={m('customized')} />}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

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
          <Button variant="ghost" size="sm" onClick={() => setField('MailTemplates', resetTemplate(raw, active))} disabled={!custom}>
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
            <DialogTitle>{m(`names.${active}`)}</DialogTitle>
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
