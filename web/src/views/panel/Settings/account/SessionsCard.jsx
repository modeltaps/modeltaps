import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Laptop, LogOut, Smartphone } from 'lucide-react';

import { API } from 'utils/api';
import { showError, showSuccess, timestamp2string } from 'utils/common';
import { describeUserAgent } from 'utils/userAgent';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { CollectionCard } from '@/components/settings/cards';

// ==============================|| SETTINGS — LOGIN SESSIONS CARD ||============================== //
// Devices signed in to this account, most recently active first. The current session is marked
// 当前设备; any other can be signed out from its row menu, or all others at once from the header.

const tk = (k) => `settingsPage.security.${k}`;

const METHOD_KEYS = {
  password: 'methodPassword',
  email_code: 'methodEmailCode',
  passkey: 'methodPasskey',
  oidc: 'methodOidc',
  github: 'methodGithub',
  wechat: 'methodWechat',
  lark: 'methodLark',
  linuxdo: 'methodLinuxdo'
};

export default function SessionsCard() {
  const { t } = useTranslation();
  const [sessions, setSessions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  // Signing out every other device is a batch action: confirm first.
  const [confirmOthers, setConfirmOthers] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await API.get('/api/user/sessions');
      if (res.data?.success) setSessions(res.data.data || []);
    } catch {
      /* surfaced by the API interceptor */
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const call = async (request, doneKey) => {
    setBusy(true);
    try {
      const res = await request();
      if (res.data?.success) {
        showSuccess(t(doneKey));
        await load();
      } else {
        showError(res.data?.message);
      }
    } catch (err) {
      showError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const others = sessions.filter((s) => !s.current);

  return (
    <>
      <CollectionCard
        title={t(tk('sessions'))}
        description={t(tk('sessionsHint'))}
        highlight="sessions"
        aside={
          others.length > 0 && (
            <Button variant="outline" size="sm" onClick={() => setConfirmOthers(true)} disabled={busy}>
              {t(tk('signOutOthers'))}
            </Button>
          )
        }
        loading={loading}
        empty={t('settingsPage.sessions.empty')}
        rows={sessions}
        columns={[
          {
            key: 'device',
            header: t('settingsUi.personal.sessions.device'),
            cell: (session) => {
              const device = describeUserAgent(session.user_agent) || t(tk('unknownDevice'));
              const Icon = /iOS|Android/.test(device) ? Smartphone : Laptop;
              return (
                <span className="flex items-center gap-2">
                  <Icon className="size-4 text-muted-foreground" />
                  <span className="font-medium">{device}</span>
                </span>
              );
            }
          },
          {
            key: 'where',
            header: t('settingsUi.personal.sessions.where'),
            cell: (session) => (
              <span className="text-muted-foreground">
                {session.ip || '-'} · {t(tk(METHOD_KEYS[session.login_method] || 'methodOther'))}
              </span>
            )
          },
          {
            key: 'last',
            header: t(tk('lastActive')),
            cell: (session) =>
              session.current ? (
                <span className="text-muted-foreground">{t(tk('currentDevice'))}</span>
              ) : (
                <span className="tabular-nums text-muted-foreground">{timestamp2string(session.last_seen_time)}</span>
              )
          }
        ]}
        menu={(session) =>
          session.current
            ? []
            : [
                {
                  label: t(tk('signOut')),
                  icon: LogOut,
                  destructive: true,
                  disabled: busy,
                  onClick: () => call(() => API.delete(`/api/user/sessions/${session.id}`), tk('signedOut'))
                }
              ]
        }
      />
      <Dialog open={confirmOthers} onOpenChange={setConfirmOthers}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t(tk('signOutOthers'))}</DialogTitle>
            <DialogDescription>{t('settingsPage.sessions.revokeOthersConfirm')}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmOthers(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="destructive"
              disabled={busy}
              onClick={async () => {
                await call(() => API.post('/api/user/sessions/logout_others'), tk('signOutOthersDone'));
                setConfirmOthers(false);
              }}
            >
              {t('common.confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
