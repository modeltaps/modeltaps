import { useEffect, useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Copy, ExternalLink, Loader2, ShieldCheck } from 'lucide-react';

import { toast } from '@/components/ui/sonner';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { startChannelOAuth, fetchChannelOAuthStatus, exchangeChannelOAuthCode } from './channelApi';

// OAuth authorization flows for the channel form, ported from the v1
// EditModal (upstream b2332f4) and rebuilt on shadcn + react-hook-form:
//   - GeminiCli (57) / Antigravity (60): popup window + postMessage fast
//     path + status polling fallback, credentials written back to `key`.
//   - ClaudeCode (58) / Codex (59): open auth page, user pastes the
//     callback URL into a dialog, exchange-code returns the credentials.
const FLOW = {
  57: { kind: 'popup', endpoint: 'geminicli', messageType: 'geminicli_oauth_result' },
  60: { kind: 'popup', endpoint: 'antigravity', messageType: 'antigravity_oauth_result' },
  58: {
    kind: 'code',
    endpoint: 'claudecode',
    provider: 'Claude',
    example: 'https://console.anthropic.com/oauth/code/callback?code=xxx&state=xxx'
  },
  59: { kind: 'code', endpoint: 'codex', provider: 'OpenAI', example: 'http://localhost:1455/auth/callback?code=xxx&state=xxx' }
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

// ClaudeCode/Codex: start returns the auth URL; the user finishes the
// authorization in the browser, then pastes the callback URL (or code)
// into this dialog and we exchange it for the credentials JSON.
function CodeOAuth({ flow, channelId, getValues, onCredentials }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [authURL, setAuthURL] = useState('');
  const [sessionId, setSessionId] = useState('');
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);

  const close = () => {
    setOpen(false);
    setAuthURL('');
    setSessionId('');
    setCode('');
    setBusy(false);
  };

  const start = async () => {
    const proxy = (getValues('proxy') || '').trim();
    setBusy(true);
    try {
      const data = await startChannelOAuth(flow.endpoint, {
        channel_id: channelId ? parseInt(channelId, 10) : 0,
        proxy
      });
      if (!data.success) {
        toast.error(data.message || t('channel_edit.oauth.startFail'));
        setBusy(false);
        return;
      }
      setAuthURL(data.data.auth_url);
      setSessionId(data.data.session_id);
      setCode('');
      setOpen(true);
      window.open(data.data.auth_url, '_blank');
    } catch (error) {
      toast.error(t('channel_edit.oauth.startFail') + ': ' + (error.message || error));
    }
    setBusy(false);
  };

  const submit = async () => {
    if (!code.trim()) {
      toast.error(t('channel_edit.oauth.codeRequired'));
      return;
    }
    setBusy(true);
    try {
      const data = await exchangeChannelOAuthCode(flow.endpoint, sessionId, code.trim());
      if (!data.success) {
        toast.error(data.message || t('channel_edit.oauth.exchangeFail'));
        setBusy(false);
        return;
      }
      onCredentials(data.data.credentials);
      toast.success(t('channel_edit.oauth.success'));
      close();
    } catch (error) {
      toast.error(t('channel_edit.oauth.exchangeFail') + ': ' + (error.message || error));
      setBusy(false);
    }
  };

  const copyAuthURL = () => {
    navigator.clipboard
      .writeText(authURL)
      .then(() => toast.success(t('channel_edit.oauth.copySuccess')))
      .catch(() => toast.error(t('channel_edit.oauth.copyFail')));
  };

  return (
    <div className="space-y-2">
      <Button type="button" variant="outline" className="w-full" disabled={busy && !open} onClick={start}>
        {busy && !open ? <Loader2 className="size-4 animate-spin" /> : <ShieldCheck className="size-4" />}
        {busy && !open ? t('channel_edit.oauth.gettingLink') : t('channel_edit.oauth.authorize')}
      </Button>
      <Alert variant="info">
        <AlertDescription>{t('channel_edit.oauth.codeFlowTip', { provider: flow.provider })}</AlertDescription>
      </Alert>

      <Dialog open={open} onOpenChange={(v) => !v && close()}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>{t('channel_edit.oauth.dialogTitle', { provider: flow.provider })}</DialogTitle>
          </DialogHeader>
          <DialogBody className="space-y-4">
            <Alert variant="info">
              <AlertDescription>
                <p className="font-medium">{t('channel_edit.oauth.stepsTitle')}</p>
                <ol className="list-decimal space-y-1 pl-5 pt-1">
                  <li>{t('channel_edit.oauth.step1')}</li>
                  <li>{t('channel_edit.oauth.step2', { provider: flow.provider })}</li>
                  <li>{t('channel_edit.oauth.step3')}</li>
                  <li>{t('channel_edit.oauth.step4')}</li>
                </ol>
              </AlertDescription>
            </Alert>
            <div className="flex gap-2">
              <Button type="button" className="flex-1" onClick={() => window.open(authURL, '_blank')}>
                <ExternalLink className="size-4" />
                {t('channel_edit.oauth.openAuthPage')}
              </Button>
              <Button type="button" variant="outline" onClick={copyAuthURL}>
                <Copy className="size-4" />
                {t('channel_edit.oauth.copyLink')}
              </Button>
            </div>
            <Textarea
              rows={3}
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder={t('channel_edit.oauth.codePlaceholder', { example: flow.example })}
            />
          </DialogBody>
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={close}>
              {t('common.cancel')}
            </Button>
            <Button type="button" disabled={busy || !code.trim()} onClick={submit}>
              {busy && <Loader2 className="size-4 animate-spin" />}
              {busy ? t('channel_edit.oauth.submitting') : t('channel_edit.oauth.submitCode')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
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
