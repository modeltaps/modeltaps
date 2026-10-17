import { useEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { CircleAlert, CircleCheck, Copy, ExternalLink, Loader2, ShieldCheck } from 'lucide-react';

import { toast } from '@/components/ui/sonner';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  startChannelOAuth,
  fetchChannelOAuthStatus,
  exchangeChannelOAuthCode,
  startCodexDeviceOAuth,
  pollCodexDeviceOAuth
} from './channelApi';
import { describeOAuthPaste, formatCountdown, oauthErrorText } from './oauthPaste';

// OAuth authorization flows for the channel form, ported from the v1
// EditModal (upstream b2332f4) and rebuilt on shadcn + react-hook-form:
//   - GeminiCli (57) / Antigravity (60): popup window + postMessage fast
//     path + status polling fallback, credentials written back to `key`.
//   - ClaudeCode (58) / Codex (59): open auth page, user pastes the
//     callback URL (Codex) or the code shown by the provider (Claude) into a
//     dialog, exchange-code returns the credentials.
//   - Codex (59) also offers device code login, the default tab: the server
//     gets a one-time code, the admin enters it on the OpenAI device page and
//     the dialog polls until the credentials come back in the poll response.
const FLOW = {
  57: { kind: 'popup', endpoint: 'geminicli', messageType: 'geminicli_oauth_result' },
  60: { kind: 'popup', endpoint: 'antigravity', messageType: 'antigravity_oauth_result' },
  58: { kind: 'code', endpoint: 'claudecode', provider: 'Claude', paste: 'code' },
  59: { kind: 'code', endpoint: 'codex', provider: 'OpenAI', paste: 'url', device: true }
};

export default function ChannelOAuth({ type, channelId, getValues, onCredentials }) {
  const flow = FLOW[type];
  if (!flow) return null;
  if (flow.kind === 'popup') {
    return <PopupOAuth flow={flow} channelId={channelId} getValues={getValues} onCredentials={onCredentials} />;
  }
  return <CodeOAuth flow={flow} channelId={channelId} getValues={getValues} onCredentials={onCredentials} />;
}

// GeminiCli/Antigravity: start -> open popup -> postMessage fast path +
// poll `GET /api/{ep}/oauth/status/:state` every 2s as fallback. The
// `handledRef` guard keeps the two completion paths from double-firing.
function PopupOAuth({ flow, channelId, getValues, onCredentials }) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [authURL, setAuthURL] = useState('');
  const handledRef = useRef(false);
  const pollRef = useRef(null);
  const timeoutRef = useRef(null);
  const popupRef = useRef(null);
  const messageRef = useRef(null);

  const cleanup = () => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }
    if (messageRef.current) {
      window.removeEventListener('message', messageRef.current);
      messageRef.current = null;
    }
    if (popupRef.current && !popupRef.current.closed) {
      popupRef.current.close();
    }
    popupRef.current = null;
  };

  // Clear timers/listeners/popup when the sheet unmounts mid-flow.
  useEffect(() => cleanup, []);

  const finish = (success, credentials, message) => {
    if (handledRef.current) return;
    handledRef.current = true;
    cleanup();
    setLoading(false);
    if (success && credentials) {
      onCredentials(credentials);
      toast.success(t('channel_edit.oauth.success'));
    } else {
      toast.error(message || t('channel_edit.oauth.fail'));
    }
  };

  const poll = async (state) => {
    if (handledRef.current) return;
    try {
      const data = await fetchChannelOAuthStatus(flow.endpoint, state);
      if (data.success && data.status === 'completed') {
        finish(data.result, data.credentials, data.message);
      }
    } catch (e) {
      // transient polling error: keep polling until timeout
    }
  };

  const start = async () => {
    const projectId = (getValues('other') || '').trim();
    const proxy = (getValues('proxy') || '').trim();
    setLoading(true);
    handledRef.current = false;
    try {
      const data = await startChannelOAuth(flow.endpoint, {
        channel_id: channelId ? parseInt(channelId, 10) : 0,
        project_id: projectId,
        proxy
      });
      if (!data.success) {
        toast.error(data.message || t('channel_edit.oauth.fail'));
        setLoading(false);
        return;
      }
      setAuthURL(data.auth_url);
      popupRef.current = window.open(data.auth_url, '_blank');

      const handleMessage = (event) => {
        // only trust the popup this flow opened; anything else falls back to polling
        if (event.source !== popupRef.current) return;
        if (!event.data || event.data.type !== flow.messageType) return;
        finish(event.data.success, event.data.credentials);
      };
      messageRef.current = handleMessage;
      window.addEventListener('message', handleMessage);

      pollRef.current = setInterval(() => poll(data.state), 2000);
      timeoutRef.current = setTimeout(
        () => {
          if (handledRef.current) return;
          handledRef.current = true;
          cleanup();
          setLoading(false);
          toast.error(t('channel_edit.oauth.timeout'));
        },
        10 * 60 * 1000
      );
    } catch (error) {
      toast.error(t('channel_edit.oauth.fail') + ': ' + (error.message || error));
      setLoading(false);
    }
  };

  const copyURL = () => {
    if (!authURL) {
      toast.error(t('channel_edit.oauth.copyEmpty'));
      return;
    }
    navigator.clipboard
      .writeText(authURL)
      .then(() => toast.success(t('channel_edit.oauth.copySuccess')))
      .catch(() => toast.error(t('channel_edit.oauth.copyFail')));
  };

  return (
    <div className="space-y-2">
      <div className="flex gap-2">
        <Button type="button" variant="outline" className="flex-1" disabled={loading} onClick={start}>
          {loading ? <Loader2 className="size-4 animate-spin" /> : <ShieldCheck className="size-4" />}
          {loading ? t('channel_edit.oauth.authorizing') : t('channel_edit.oauth.authorize')}
        </Button>
        <Button type="button" variant="outline" disabled={!authURL} onClick={copyURL}>
          <Copy className="size-4" />
          {t('channel_edit.oauth.copyLink')}
        </Button>
      </div>
      <Alert variant="info">
        <AlertDescription>
          {!(getValues('other') || '').trim() && <p className="font-medium">{t('channel_edit.oauth.googleAutoProject')}</p>}
          <p>{t('channel_edit.oauth.googleRedirectTip')}</p>
        </AlertDescription>
      </Alert>
    </div>
  );
}

const copyText = (t, text, successKey) =>
  navigator.clipboard
    .writeText(text)
    .then(() => toast.success(t(successKey)))
    .catch(() => toast.error(t('channel_edit.oauth.copyFail')));

// Copies synchronously inside a click, so it works before a new tab takes focus and on plain
// http deployments where navigator.clipboard is missing. The textarea goes into `host` (inside
// the dialog) so the dialog keeps focus. Returns whether the copy worked.
function copyNow(text, host) {
  const area = document.createElement('textarea');
  area.value = text;
  area.setAttribute('readonly', '');
  area.style.position = 'fixed';
  area.style.opacity = '0';
  (host || document.body).appendChild(area);
  area.select();
  let ok = false;
  try {
    ok = document.execCommand('copy');
  } catch {
    // execCommand is missing or refused; fall back to the async clipboard below
  }
  area.remove();
  if (!ok && navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(text).catch(() => {});
    ok = true;
  }
  return ok;
}

const channelIdOf = (channelId) => (channelId ? parseInt(channelId, 10) : 0);

// ClaudeCode/Codex dialog. Codex opens on the device code tab; the paste tab
// (the only one for Claude) starts its own authorization session.
function CodeOAuth({ flow, channelId, getValues, onCredentials }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState(flow.device ? 'device' : 'paste');

  const done = (credentials) => {
    onCredentials(credentials);
    toast.success(t('channel_edit.oauth.success'));
    setOpen(false);
  };

  return (
    <div className="space-y-2">
      <Button
        type="button"
        variant="outline"
        className="w-full"
        onClick={() => {
          setMode(flow.device ? 'device' : 'paste');
          setOpen(true);
        }}
      >
        <ShieldCheck className="size-4" />
        {t('channel_edit.oauth.authorize')}
      </Button>
      <Alert variant="info">
        <AlertDescription>
          {t(flow.device ? 'channelOAuth.tip.device' : 'channelOAuth.tip.code', { provider: flow.provider })}
        </AlertDescription>
      </Alert>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>{t('channel_edit.oauth.dialogTitle', { provider: flow.provider })}</DialogTitle>
          </DialogHeader>
          <DialogBody className="space-y-4">
            {flow.device && (
              <Tabs value={mode} onValueChange={setMode}>
                <TabsList className="w-full">
                  <TabsTrigger value="device" className="flex-1">
                    {t('channelOAuth.tabs.device')}
                  </TabsTrigger>
                  <TabsTrigger value="paste" className="flex-1">
                    {t('channelOAuth.tabs.paste')}
                  </TabsTrigger>
                </TabsList>
              </Tabs>
            )}
            {open && mode === 'device' && <DeviceCodePanel channelId={channelId} getValues={getValues} onDone={done} />}
            {open && mode === 'paste' && (
              <PastePanel flow={flow} channelId={channelId} getValues={getValues} onDone={done} autoOpen={!flow.device} />
            )}
          </DialogBody>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>
              {t('common.cancel')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function OAuthError({ error, action }) {
  if (!error) return null;
  return (
    <Alert variant="error">
      <CircleAlert />
      <AlertDescription className="space-y-2">
        <p>{error.text}</p>
        {error.detail && <p className="break-all font-mono text-xs text-muted-foreground">{error.detail}</p>}
        {action}
      </AlertDescription>
    </Alert>
  );
}

// Device code login (Codex). Polls one request at a time, at the interval the
// server reports, until approved, refused or expired; unmounting stops it.
function DeviceCodePanel({ channelId, getValues, onDone }) {
  const { t } = useTranslation();
  const [device, setDevice] = useState(null); // { sessionId, userCode, verificationUrl, expiresAt }
  const [error, setError] = useState(null);
  const [now, setNow] = useState(() => Date.now());
  const [copied, setCopied] = useState(''); // empty until the code is copied, then yes or no
  const aliveRef = useRef(true);
  const timerRef = useRef(null);
  const onDoneRef = useRef(onDone);
  useEffect(() => {
    onDoneRef.current = onDone;
  });

  const stopTimer = () => {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = null;
  };

  const poll = (session, interval) => {
    timerRef.current = setTimeout(async () => {
      if (!aliveRef.current) return;
      if (Date.now() >= session.expiresAt) {
        setError({ text: t('apiError.channel_oauth_device_code_expired') });
        return;
      }
      let data;
      try {
        data = await pollCodexDeviceOAuth(session.sessionId);
      } catch {
        // network hiccup: try again at the same pace
        if (aliveRef.current) poll(session, interval);
        return;
      }
      if (!aliveRef.current) return;
      if (data.success && data.data?.status === 'completed') {
        onDoneRef.current(data.data.credentials);
      } else if (data.success) {
        poll(session, data.data?.interval || interval);
      } else {
        setError(oauthErrorText(t, data, 'channel_edit.oauth.fail'));
      }
    }, interval * 1000);
  };

  const request = async () => {
    try {
      const data = await startCodexDeviceOAuth({
        channel_id: channelIdOf(channelId),
        proxy: (getValues('proxy') || '').trim()
      });
      if (!aliveRef.current) return;
      if (!data.success) {
        setError(oauthErrorText(t, data, 'channel_edit.oauth.startFail'));
        return;
      }
      const session = {
        sessionId: data.data.session_id,
        userCode: data.data.user_code,
        verificationUrl: data.data.verification_url,
        expiresAt: Date.now() + data.data.expires_in * 1000
      };
      setDevice(session);
      poll(session, data.data.interval || 5);
    } catch (e) {
      if (aliveRef.current) setError({ text: t('channel_edit.oauth.startFail') + ': ' + (e.message || e) });
    }
  };

  const restart = () => {
    stopTimer();
    setError(null);
    setDevice(null);
    setCopied('');
    request();
  };

  useEffect(() => {
    aliveRef.current = true;
    request();
    const tick = setInterval(() => setNow(Date.now()), 1000);
    return () => {
      aliveRef.current = false;
      stopTimer();
      clearInterval(tick);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const retry = (
    <Button type="button" size="sm" variant="outline" onClick={restart}>
      {t('channelOAuth.device.retry')}
    </Button>
  );

  if (error) {
    return (
      <div className="space-y-3">
        <OAuthError error={error} action={retry} />
        <p className="text-sm text-muted-foreground">{t('channelOAuth.device.fallbackHint')}</p>
      </div>
    );
  }
  if (!device) {
    return (
      <p className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" />
        {t('channelOAuth.device.requesting')}
      </p>
    );
  }

  // Copy first, while this page still has focus (the browser refuses clipboard writes once the
  // new tab takes it), then open the page within the same click so popup blockers allow it.
  const openPage = (e) => {
    setCopied(copyNow(device.userCode, e.currentTarget.parentElement) ? 'yes' : 'no');
    window.open(device.verificationUrl, '_blank', 'noopener,noreferrer');
  };
  const copyCode = (e) => {
    const ok = copyNow(device.userCode, e.currentTarget.parentElement);
    setCopied(ok ? 'yes' : 'no');
    if (ok) toast.success(t('channelOAuth.device.codeCopied'));
    else toast.error(t('channel_edit.oauth.copyFail'));
  };
  const hint = {
    '': t('channelOAuth.device.openHint'),
    yes: t('channelOAuth.device.copiedHint'),
    no: t('channelOAuth.device.copyFailedHint')
  }[copied];

  return (
    <div className="space-y-5">
      <div className="flex flex-col items-center gap-3 pt-2 text-center">
        <p className="text-sm text-muted-foreground">{t('channelOAuth.device.codeLabel')}</p>
        <div className="flex items-center gap-1 rounded-lg border bg-muted/50 py-1 pl-5 pr-1">
          <code data-testid="device-user-code" className="font-mono text-3xl font-semibold tracking-[0.15em]">
            {device.userCode}
          </code>
          <Button type="button" variant="ghost" size="icon" aria-label={t('channelOAuth.device.copyCode')} onClick={copyCode}>
            <Copy className="size-4" />
          </Button>
        </div>
        <Button type="button" size="lg" className="mt-1 w-full max-w-xs" onClick={openPage}>
          <ExternalLink className="size-4" />
          {t('channelOAuth.device.open')}
        </Button>
        <p
          className={`flex max-w-sm items-start justify-center gap-1.5 text-xs ${copied === 'yes' ? 'text-emerald-600' : 'text-muted-foreground'}`}
          aria-live="polite"
        >
          {copied === 'yes' && <CircleCheck className="mt-px size-3.5 shrink-0" />}
          <span>{hint}</span>
        </p>
        <p className="flex w-full items-center justify-center gap-2 border-t pt-4 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" />
          {t('channelOAuth.device.waiting', { time: formatCountdown((device.expiresAt - now) / 1000) })}
        </p>
      </div>
      <Alert variant="info">
        <AlertDescription className="space-y-1 text-xs">
          <p>{t('channelOAuth.device.enableTip')}</p>
          <p>{t('channelOAuth.device.phishingTip')}</p>
        </AlertDescription>
      </Alert>
    </div>
  );
}

// What the admin should look for after approving, drawn instead of a
// screenshot: the Codex error page's address bar, or Claude's code box.
function PasteExample({ paste }) {
  const { t } = useTranslation();
  if (paste === 'url') {
    return (
      <figure className="overflow-hidden rounded-md border text-xs" aria-hidden="true">
        <div className="flex items-center gap-2 border-b bg-muted px-3 py-2">
          <span className="flex-1 truncate rounded bg-background px-2 py-1 font-mono ring-2 ring-primary">
            localhost:1455/auth/callback?code=ac_…&amp;state=…
          </span>
        </div>
        <div className="px-3 py-3 text-muted-foreground">{t('channelOAuth.paste.url.mockError')}</div>
        <figcaption className="border-t px-3 py-1.5 text-muted-foreground">{t('channelOAuth.paste.url.mockCaption')}</figcaption>
      </figure>
    );
  }
  return (
    <figure className="overflow-hidden rounded-md border text-xs" aria-hidden="true">
      <div className="space-y-2 px-3 py-3">
        <div className="font-medium">{t('channelOAuth.paste.code.mockTitle')}</div>
        <div className="flex items-center gap-2">
          <span className="flex-1 truncate rounded bg-muted px-2 py-1 font-mono">xxxxxxxxxxxx#yyyyyyyy</span>
          <span className="rounded bg-primary px-2 py-1 text-primary-foreground ring-2 ring-primary ring-offset-1">
            {t('channelOAuth.paste.code.mockCopy')}
          </span>
        </div>
      </div>
      <figcaption className="border-t px-3 py-1.5 text-muted-foreground">{t('channelOAuth.paste.code.mockCaption')}</figcaption>
    </figure>
  );
}

const PASTE_STATUS_OK = new Set(['ok']);

// Paste flow: start an authorization session, open the provider page, then
// paste what it ends on. The input is checked as you type; a valid paste for
// this session is submitted right away.
function PastePanel({ flow, channelId, getValues, onDone, autoOpen }) {
  const { t } = useTranslation();
  const [session, setSession] = useState(null); // { authURL, state, minutes }
  const [input, setInput] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);
  const aliveRef = useRef(true);

  const request = async (openPage) => {
    try {
      const data = await startChannelOAuth(flow.endpoint, {
        channel_id: channelIdOf(channelId),
        proxy: (getValues('proxy') || '').trim()
      });
      if (!aliveRef.current) return;
      if (!data.success) {
        setError(oauthErrorText(t, data, 'channel_edit.oauth.startFail'));
        return;
      }
      setSession({
        authURL: data.data.auth_url,
        state: data.data.session_id,
        minutes: Math.round((data.data.expires_in || 600) / 60)
      });
      if (openPage) window.open(data.data.auth_url, '_blank', 'noopener,noreferrer');
    } catch (e) {
      if (aliveRef.current) setError({ text: t('channel_edit.oauth.startFail') + ': ' + (e.message || e) });
    }
  };

  const restart = () => {
    setError(null);
    setInput('');
    setSession(null);
    request(false);
  };

  useEffect(() => {
    aliveRef.current = true;
    request(autoOpen);
    return () => {
      aliveRef.current = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const submit = async (value) => {
    if (busy || !session) return;
    setBusy(true);
    setError(null);
    try {
      const data = await exchangeChannelOAuthCode(flow.endpoint, session.state, value.trim());
      if (!aliveRef.current) return;
      if (data.success) {
        onDone(data.data.credentials);
        return;
      }
      setError(oauthErrorText(t, data, 'channel_edit.oauth.exchangeFail'));
    } catch (e) {
      if (aliveRef.current) setError({ text: t('channel_edit.oauth.exchangeFail') + ': ' + (e.message || e) });
    }
    if (aliveRef.current) setBusy(false);
  };

  const status = describeOAuthPaste(input, session?.state);
  const statusText = {
    ok: t('channelOAuth.paste.status.ok'),
    stateMismatch: t('channelOAuth.paste.status.stateMismatch'),
    denied: t('channelOAuth.paste.status.denied', { error: status.error }),
    authLink: t('channelOAuth.paste.status.authLink'),
    invalid: t(`channelOAuth.paste.status.invalid_${flow.paste}`)
  }[status.kind];
  const restartButton =
    error?.code === 'channel_oauth_session_expired' ? (
      <Button type="button" size="sm" variant="outline" onClick={restart}>
        {t('channelOAuth.paste.restart')}
      </Button>
    ) : null;

  if (!session) {
    return error ? (
      <OAuthError error={error} action={restartButton} />
    ) : (
      <p className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="size-4 animate-spin" />
        {t('channel_edit.oauth.gettingLink')}
      </p>
    );
  }

  return (
    <div className="space-y-4">
      <ol className="list-decimal space-y-1 pl-5 text-sm">
        <li>{t(`channelOAuth.paste.${flow.paste}.step1`, { provider: flow.provider })}</li>
        <li>{t(`channelOAuth.paste.${flow.paste}.step2`)}</li>
        <li>{t(`channelOAuth.paste.${flow.paste}.step3`)}</li>
      </ol>
      <PasteExample paste={flow.paste} />
      <div className="flex gap-2">
        <Button type="button" className="flex-1" onClick={() => window.open(session.authURL, '_blank', 'noopener,noreferrer')}>
          <ExternalLink className="size-4" />
          {t('channel_edit.oauth.openAuthPage')}
        </Button>
        <Button type="button" variant="outline" onClick={() => copyText(t, session.authURL, 'channel_edit.oauth.copySuccess')}>
          <Copy className="size-4" />
          {t('channel_edit.oauth.copyLink')}
        </Button>
      </div>
      <div className="space-y-1.5">
        <Textarea
          rows={3}
          value={input}
          aria-label={t(`channelOAuth.paste.${flow.paste}.placeholder`)}
          placeholder={t(`channelOAuth.paste.${flow.paste}.placeholder`)}
          onChange={(e) => {
            setInput(e.target.value);
            setError(null);
          }}
          onPaste={(e) => {
            const text = e.clipboardData?.getData('text') || '';
            if (describeOAuthPaste(text, session.state).kind === 'ok') {
              e.preventDefault();
              setInput(text.trim());
              submit(text);
            }
          }}
        />
        {statusText && (
          <p
            className={`flex items-start gap-1.5 text-xs ${PASTE_STATUS_OK.has(status.kind) ? 'text-emerald-600' : 'text-destructive'}`}
            aria-live="polite"
          >
            {PASTE_STATUS_OK.has(status.kind) ? (
              <CircleCheck className="mt-px size-3.5 shrink-0" />
            ) : (
              <CircleAlert className="mt-px size-3.5 shrink-0" />
            )}
            {statusText}
          </p>
        )}
      </div>
      <OAuthError error={error} action={restartButton} />
      <p className="text-xs text-muted-foreground">{t('channelOAuth.paste.expiresTip', { minutes: session.minutes })}</p>
      <Button type="button" className="w-full" disabled={busy || status.kind !== 'ok'} onClick={() => submit(input)}>
        {busy && <Loader2 className="size-4 animate-spin" />}
        {busy ? t('channel_edit.oauth.submitting') : t('channel_edit.oauth.submitCode')}
      </Button>
    </div>
  );
}

const sharedPropTypes = {
  channelId: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  getValues: PropTypes.func.isRequired,
  onCredentials: PropTypes.func.isRequired
};

ChannelOAuth.propTypes = {
  type: PropTypes.number,
  ...sharedPropTypes
};

PopupOAuth.propTypes = {
  flow: PropTypes.object.isRequired,
  ...sharedPropTypes
};

CodeOAuth.propTypes = {
  flow: PropTypes.object.isRequired,
  ...sharedPropTypes
};

DeviceCodePanel.propTypes = {
  channelId: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  getValues: PropTypes.func.isRequired,
  onDone: PropTypes.func.isRequired
};

PastePanel.propTypes = {
  flow: PropTypes.object.isRequired,
  channelId: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  getValues: PropTypes.func.isRequired,
  onDone: PropTypes.func.isRequired,
  autoOpen: PropTypes.bool
};

PasteExample.propTypes = {
  paste: PropTypes.oneOf(['url', 'code']).isRequired
};

OAuthError.propTypes = {
  error: PropTypes.shape({ text: PropTypes.string, detail: PropTypes.string }),
  action: PropTypes.node
};
