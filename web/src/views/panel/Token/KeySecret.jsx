import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Check, Copy, TriangleAlert } from 'lucide-react';

import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { maskSecret } from 'utils/maskSecret';

// API keys are stored as hashes only (SEC-47): the full key is in the answer to the request that
// created or reset it and nowhere else. These parts show it that one time, and let a dialog that
// needs the key (test command, client setup) take it pasted by the user instead of fetching it.

export function useCopied() {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return undefined;
    const id = setTimeout(() => setCopied(false), 1500);
    return () => clearTimeout(id);
  }, [copied]);
  return [copied, () => setCopied(true)];
}

// A read-only value with an icon copy button inside its box: the whole value is shown (wrapped,
// never cut off) and a click on it selects all of it. The icon turns into a check for a moment.
export function CopyField({ label, value, onCopy, testId }) {
  const { t } = useTranslation();
  const [copied, markCopied] = useCopied();
  return (
    <div>
      <p className="mb-1.5 text-sm font-medium">{label}</p>
      <div className="flex items-start gap-1 rounded-lg border border-input bg-muted/40 py-1 pl-3 pr-1">
        <code className="min-w-0 flex-1 select-all break-all py-1.5 font-mono text-[13px] leading-5" data-testid={testId}>
          {value}
        </code>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-8 shrink-0 rounded-md text-muted-foreground hover:text-foreground"
          aria-label={copied ? t('apiKey.new.copied') : t('token_index.copy')}
          title={copied ? t('apiKey.new.copied') : t('token_index.copy')}
          onClick={() => {
            onCopy();
            markCopied();
          }}
        >
          {copied ? <Check className="size-4 text-success" /> : <Copy className="size-4" />}
        </Button>
      </div>
    </div>
  );
}

CopyField.propTypes = { label: PropTypes.node, value: PropTypes.string, onCopy: PropTypes.func.isRequired, testId: PropTypes.string };

// The base URL as a secondary line under the key: needed to point a client at this gateway, but
// also on the list page, so it does not compete with the key.
export function BaseUrlLine({ value, onCopy }) {
  const { t } = useTranslation();
  const [copied, markCopied] = useCopied();
  return (
    <div className="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
      <span className="shrink-0">{t('token_index.apiAddress')}</span>
      <code className="min-w-0 select-all truncate font-mono text-foreground/80">{value}</code>
      <button
        type="button"
        className="flex size-6 shrink-0 items-center justify-center rounded-md transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
        aria-label={copied ? t('apiKey.new.copied') : t('token_index.copy')}
        title={copied ? t('apiKey.new.copied') : t('token_index.copy')}
        onClick={() => {
          onCopy();
          markCopied();
        }}
      >
        {copied ? <Check className="size-3.5 text-success" /> : <Copy className="size-3.5" />}
      </button>
    </div>
  );
}

BaseUrlLine.propTypes = { value: PropTypes.string.isRequired, onCopy: PropTypes.func.isRequired };

// The key, shown once: the "you won't see it again" note, the key with copy, and the base URL.
// Management keys pass their own label and note (they are deleted and recreated, not reset).
export function SecretKeyReveal({ secret, baseUrl, onCopy, label, note }) {
  const { t } = useTranslation();
  const keyLabel = label || t('token_index.token');
  return (
    <div className="space-y-2.5">
      <p
        role="note"
        className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/[0.06] px-3 py-2 text-xs text-foreground"
        data-testid="key-once-note"
      >
        <TriangleAlert className="mt-0.5 size-3.5 shrink-0 text-amber-600 dark:text-amber-500" aria-hidden="true" />
        <span>{note || t('apiKey.secret.onceNote')}</span>
      </p>
      <CopyField label={keyLabel} value={secret} onCopy={() => onCopy(secret, keyLabel)} testId="new-token-key" />
      {baseUrl && <BaseUrlLine value={baseUrl} onCopy={() => onCopy(baseUrl, t('token_index.apiAddress'))} />}
    </div>
  );
}

SecretKeyReveal.propTypes = {
  secret: PropTypes.string.isRequired,
  baseUrl: PropTypes.string,
  onCopy: PropTypes.func.isRequired,
  label: PropTypes.string,
  note: PropTypes.string
};

// The answer to "reset key": the new key, shown once.
export function KeySecretDialog({ name, secret, baseUrl, onCopy, onClose }) {
  const { t } = useTranslation();
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-md" aria-labelledby="key-secret-title">
        <DialogHeader className="border-b-0 pb-2">
          <DialogTitle id="key-secret-title" className="text-base font-semibold">
            {t('apiKey.secret.resetDoneTitle', { name })}
          </DialogTitle>
          <DialogDescription>{t('apiKey.secret.resetDoneHint')}</DialogDescription>
        </DialogHeader>
        <div className="px-6 pb-6">
          <SecretKeyReveal secret={secret} baseUrl={baseUrl} onCopy={onCopy} />
        </div>
        <DialogFooter>
          <Button className="rounded-lg" onClick={onClose}>
            {t('apiKey.new.done')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

KeySecretDialog.propTypes = {
  name: PropTypes.string,
  secret: PropTypes.string.isRequired,
  baseUrl: PropTypes.string,
  onCopy: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired
};

// What a user pasted as a key, with the optional "Bearer " and "sk-" stripped and the "sk-" put
// back, or '' when nothing was pasted.
export function normalizePastedKey(value) {
  const raw = String(value || '')
    .trim()
    .replace(/^Bearer\s+/i, '')
    .replace(/^sk-/, '')
    .trim();
  return raw ? `sk-${raw}` : '';
}

// Whether a pasted key can be this API key: its masked form must equal the stored hint.
export function pastedKeyMatches(fullKey, hint) {
  if (!fullKey || !hint) return true;
  return maskSecret(fullKey) === hint;
}

// The key a dialog needs (test command, client setup) when it is not known: the user pastes it.
// Nothing is fetched or saved; the value only lives in the dialog's state.
export function PasteKeyField({ id, value, onChange, hint }) {
  const { t } = useTranslation();
  const full = normalizePastedKey(value);
  const mismatch = full && !pastedKeyMatches(full, hint);
  return (
    <div className="grid gap-1.5">
      <label htmlFor={id} className="text-sm font-medium text-foreground">
        {t('apiKey.secret.pasteLabel')}
      </label>
      <Input
        id={id}
        type="password"
        autoComplete="off"
        spellCheck={false}
        className="font-mono"
        placeholder={hint ? t('apiKey.secret.pastePlaceholderHint', { hint }) : t('apiKey.secret.pastePlaceholder')}
        value={value}
        aria-invalid={Boolean(mismatch)}
        onChange={(e) => onChange(e.target.value)}
      />
      <p className={mismatch ? 'text-xs text-destructive' : 'text-xs text-muted-foreground'}>
        {mismatch ? t('apiKey.secret.pasteMismatch', { hint }) : t('apiKey.secret.pasteHint')}
      </p>
    </div>
  );
}

PasteKeyField.propTypes = {
  id: PropTypes.string.isRequired,
  value: PropTypes.string,
  onChange: PropTypes.func.isRequired,
  hint: PropTypes.string
};
