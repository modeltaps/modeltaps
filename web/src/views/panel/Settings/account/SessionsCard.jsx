import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Laptop, Loader2, MonitorSmartphone, Smartphone } from 'lucide-react';

import { API } from 'utils/api';
import { showError, showSuccess, timestamp2string } from 'utils/common';
import { describeUserAgent } from 'utils/userAgent';
import { Button } from '@/components/ui/button';
import { EmptyState } from '@/components/ui/empty-state';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { MethodRow, SecurityCard } from './MethodRow';

// ==============================|| SETTINGS — LOGIN SESSIONS CARD ||============================== //
// 已登录本账号的设备,最近活跃在前;当前会话只标灰字「当前设备」,其余可单独登出,也可一键登出其它全部。

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
  // 「登出其它设备」是批量动作,先确认再发请求。
  const [confirmOthers, setConfirmOthers] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await API.get('/api/user/sessions');
      if (res.data?.success) setSessions(res.data.data || []);
    } catch {
      /* 拦截器已弹过错误 */
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const revoke = async (id) => {
    setBusy(true);
    try {
      const res = await API.delete(`/api/user/sessions/${id}`);
      if (res.data?.success) {
        showSuccess(t(tk('signedOut')));
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

  const revokeOthers = async () => {
    setBusy(true);
    try {
      const res = await API.post('/api/user/sessions/logout_others');
      if (res.data?.success) {
        showSuccess(t(tk('signOutOthersDone')));
        await load();
      } else {
        showError(res.data?.message);
      }
    } catch (err) {
      showError(err.message);
    } finally {
      setBusy(false);
      setConfirmOthers(false);
    }
  };

  const others = sessions.filter((s) => !s.current);

  return (
    <SecurityCard title={t(tk('sessions'))} description={t(tk('sessionsHint'))}>
      {loading && (
        <div className="flex items-center justify-center py-8 text-muted-foreground">
          <Loader2 className="size-5 animate-spin" />
        </div>
      )}
      {!loading && sessions.length === 0 && <EmptyState icon={MonitorSmartphone} title={t('settingsPage.sessions.empty')} />}
      {sessions.map((session) => {
        const device = describeUserAgent(session.user_agent) || t(tk('unknownDevice'));
        const method = t(tk(METHOD_KEYS[session.login_method] || 'methodOther'));
        const isMobile = /iOS|Android/.test(device);
        return (
          <MethodRow
            key={session.id}
            icon={isMobile ? Smartphone : Laptop}
            title={device}
            subtitle={`${session.ip || '-'} · ${method} · ${t(tk('lastActive'))} ${timestamp2string(session.last_seen_time)}`}
          >
            {session.current ? (
              <span className="text-sm text-muted-foreground">{t(tk('currentDevice'))}</span>
            ) : (
              <Button variant="outline" size="sm" onClick={() => revoke(session.id)} disabled={busy}>
                {t(tk('signOut'))}
              </Button>
            )}
          </MethodRow>
        );
      })}
      {others.length > 0 && (
        <div className="flex justify-end pt-4">
          <Button variant="outline" size="sm" onClick={() => setConfirmOthers(true)} disabled={busy}>
            {t(tk('signOutOthers'))}
          </Button>
        </div>
      )}

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
            <Button variant="destructive" disabled={busy} onClick={revokeOthers}>
              {t('common.confirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SecurityCard>
  );
}
