import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router';
import { Building2, Check, Loader2, Mailbox, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { API } from 'utils/api';
import { showError, showSuccess, timestamp2string } from 'utils/common';
import { useOrg } from 'contexts/OrgContext';

// ==============================|| ORGANIZATION — MY INVITATIONS (personal context) ||============================== //
// Pending directed invitations for the current user + invite-link redeem box.
// Visible whenever the user is in personal context (or opened an invite link).

// 支持粘贴完整邀请链接或纯 token
function extractToken(raw) {
  const s = (raw || '').trim();
  if (!s) return '';
  try {
    const url = new URL(s);
    return url.searchParams.get('invite') || s;
  } catch {
    return s;
  }
}

export default function MyInvitations() {
  const { t } = useTranslation();
  const { refreshOrganizations, switchOrg } = useOrg();
  const [searchParams, setSearchParams] = useSearchParams();
  const [invitations, setInvitations] = useState([]);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState(0); // invitation id being processed
  const [linkInput, setLinkInput] = useState(() => searchParams.get('invite') || '');
  const [joining, setJoining] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/org/invitations');
      const { success, message, data } = res.data;
      if (success) setInvitations(Array.isArray(data) ? data : []);
      else showError(message);
    } catch (error) {
      console.error(error);
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const afterJoin = async (organizationId) => {
    if (searchParams.get('invite')) setSearchParams({}, { replace: true });
    await refreshOrganizations();
    if (organizationId) switchOrg(organizationId);
  };

  const respond = async (invitation, action) => {
    setActing(invitation.id);
    try {
      const res = await API.post(`/api/org/invitations/${action}`, { token: invitation.token });
      const { success, message, data } = res.data;
      if (success) {
        if (action === 'accept') {
          showSuccess(t('orgPage.myInvitations.acceptSuccess'));
          await afterJoin(data?.organization_id);
        } else {
          showSuccess(t('orgPage.myInvitations.rejectSuccess'));
        }
        fetchData();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setActing(0);
    }
  };

  const joinByLink = async () => {
    const token = extractToken(linkInput);
    if (!token) return;
    setJoining(true);
    try {
      const res = await API.post('/api/org/invitations/accept', { token });
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(t('orgPage.myInvitations.acceptSuccess'));
        setLinkInput('');
        await afterJoin(data?.organization_id);
        fetchData();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setJoining(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card className="p-6">
        <div className="flex items-center gap-2">
          <Building2 className="size-5 text-muted-foreground" />
          <h3 className="text-base font-semibold">{t('orgPage.myInvitations.joinByLink')}</h3>
        </div>
        <p className="mt-1 text-sm text-muted-foreground">{t('orgPage.myInvitations.joinByLinkHint')}</p>
        <div className="mt-4 flex flex-col gap-2 sm:flex-row">
          <Input
            value={linkInput}
            onChange={(e) => setLinkInput(e.target.value)}
            placeholder={t('orgPage.myInvitations.joinByLinkPlaceholder')}
            onKeyDown={(e) => {
              if (e.key === 'Enter') joinByLink();
            }}
          />
          <Button onClick={joinByLink} disabled={joining || !extractToken(linkInput)}>
            {joining && <Loader2 className="size-4 animate-spin" />}
            {t('orgPage.myInvitations.join')}
          </Button>
        </div>
      </Card>

      <Card className="p-6">
        <div className="flex items-center gap-2">
          <Mailbox className="size-5 text-muted-foreground" />
          <h3 className="text-base font-semibold">{t('orgPage.myInvitations.title')}</h3>
        </div>
        {loading ? (
          <div className="flex justify-center p-8">
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
          </div>
        ) : invitations.length === 0 ? (
          <p className="mt-4 text-sm text-muted-foreground">{t('orgPage.myInvitations.empty')}</p>
        ) : (
          <div className="mt-4 divide-y divide-border">
            {invitations.map((inv) => (
              <div key={inv.id} className="flex flex-col gap-3 py-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0 space-y-1">
                  <p className="truncate text-sm font-medium">{inv.organization_name || `#${inv.organization_id}`}</p>
                  <p className="text-xs text-muted-foreground">
                    {t('orgPage.myInvitations.inviterLabel')}: {inv.inviter_username || inv.inviter_id} ·{' '}
                    <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
                      {t(`org.roles.${inv.role}`)}
                    </Badge>{' '}
                    ·{' '}
                    {inv.expired_time > 0
                      ? `${t('orgPage.myInvitations.expiresLabel')}: ${timestamp2string(inv.expired_time)}`
                      : t('orgPage.myInvitations.never')}
                  </p>
                </div>
                <div className="flex shrink-0 gap-2">
                  <Button size="sm" onClick={() => respond(inv, 'accept')} disabled={acting === inv.id}>
                    <Check className="size-4" /> {t('orgPage.myInvitations.accept')}
                  </Button>
                  <Button size="sm" variant="outline" onClick={() => respond(inv, 'reject')} disabled={acting === inv.id}>
                    <X className="size-4" /> {t('orgPage.myInvitations.reject')}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
