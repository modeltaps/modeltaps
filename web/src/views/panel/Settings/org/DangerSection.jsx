import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { ArrowRightLeft, LogOut, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { ActionCard, ActionRow } from '@/components/settings/cards';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { Select, SelectTrigger, SelectContent, SelectItem } from '@/components/ui/select';
import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import { useOrg } from 'contexts/OrgContext';

// ==============================|| SETTINGS — ORG DANGER ZONE ||============================== //
// Action rows: leave (any member; the owner transfers first), transfer ownership and dissolve
// (owner only; the backend checks too). The organization on this page is not necessarily the
// active context, so only leaving or dissolving the active one drops back to personal context.

export default function DangerSection({ orgId, role, detail, onRefresh }) {
  const { t } = useTranslation();
  const { refreshOrganizations, switchOrg, currentOrgId } = useOrg();
  const isOwner = role === 'owner';
  const org = detail?.organization;

  const [leaveOpen, setLeaveOpen] = useState(false);
  const [transferOpen, setTransferOpen] = useState(false);
  const [dissolveOpen, setDissolveOpen] = useState(false);
  const [acting, setActing] = useState(false);
  const [admins, setAdmins] = useState([]);
  const [transferTarget, setTransferTarget] = useState('');
  const [dissolveName, setDissolveName] = useState('');

  // 转移所有权:目标须为该组织 Admin
  useEffect(() => {
    if (!transferOpen) return;
    setTransferTarget('');
    API.get(`/api/org/${orgId}/members`, { params: { page: 1, size: 100, order: 'id' } })
      .then((res) => {
        const { success, message, data } = res.data;
        if (success) setAdmins((data?.data || []).filter((m) => m.role === 'admin'));
        else showError(message || t('usagePage.loadFailed'));
      })
      .catch((error) => showError(error));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [transferOpen, orgId]);

  // 离开 / 解散的是当前上下文所在组织时才回到个人上下文
  const dropContextIfActive = () => {
    if (currentOrgId === orgId) switchOrg(null);
  };

  const leave = async () => {
    setActing(true);
    try {
      const res = await API.post(`/api/org/${orgId}/leave`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('orgPage.settings.leaveSuccess'));
        dropContextIfActive();
        refreshOrganizations();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setActing(false);
      setLeaveOpen(false);
    }
  };

  const transferOwnership = async () => {
    const userId = parseInt(transferTarget, 10);
    if (!userId) return;
    setActing(true);
    try {
      const res = await API.post(`/api/org/${orgId}/transfer`, { user_id: userId });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('orgPage.settings.transferSuccess'));
        onRefresh?.();
        refreshOrganizations();
        setTransferOpen(false);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setActing(false);
    }
  };

  const dissolve = async () => {
    setActing(true);
    try {
      const res = await API.delete(`/api/org/${orgId}/`, { data: { name: dissolveName.trim() } });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('orgPage.settings.dissolveSuccess'));
        dropContextIfActive();
        refreshOrganizations();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setActing(false);
      setDissolveOpen(false);
    }
  };

  return (
    <>
      <ActionCard title={t('orgPage.settings.dangerZone')} description={t('settingsUi.org.cards.danger.description')} highlight="danger">
        <ActionRow
          icon={LogOut}
          title={t('orgPage.settings.leave')}
          description={isOwner ? t('orgPage.settings.leaveOwnerHint') : t('orgPage.settings.leaveConfirm')}
          actionLabel={t('orgPage.settings.leave')}
          disabled={isOwner}
          onAction={() => setLeaveOpen(true)}
        />
        {isOwner && (
          <>
            <ActionRow
              icon={ArrowRightLeft}
              title={t('orgPage.settings.transferOwnership')}
              description={t('orgPage.settings.transferOwnershipDesc')}
              actionLabel={t('orgPage.settings.transferOwnership')}
              onAction={() => setTransferOpen(true)}
            />
            <ActionRow
              icon={Trash2}
              title={t('orgPage.settings.dissolve')}
              description={t('orgPage.settings.dissolveDesc')}
              actionLabel={t('orgPage.settings.dissolve')}
              destructive
              onAction={() => {
                setDissolveName('');
                setDissolveOpen(true);
              }}
            />
          </>
        )}
      </ActionCard>

      <Dialog open={leaveOpen} onOpenChange={setLeaveOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('orgPage.settings.leave')}</DialogTitle>
            <DialogDescription>{t('orgPage.settings.leaveConfirm')}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setLeaveOpen(false)} disabled={acting}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" onClick={leave} disabled={acting}>
              {t('orgPage.settings.leave')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={transferOpen} onOpenChange={setTransferOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('orgPage.settings.transferOwnership')}</DialogTitle>
            <DialogDescription>{t('orgPage.settings.transferOwnershipDesc')}</DialogDescription>
          </DialogHeader>
          <DialogBody>
            {admins.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t('orgPage.settings.transferNoAdmins')}</p>
            ) : (
              <div className="space-y-4">
                <Label>{t('orgPage.settings.transferTarget')}</Label>
                <Select value={transferTarget} onValueChange={setTransferTarget}>
                  <SelectTrigger>
                    <span className="truncate">
                      {transferTarget
                        ? (() => {
                            const m = admins.find((a) => String(a.user_id) === transferTarget);
                            return m ? m.display_name || m.username || `#${m.user_id}` : transferTarget;
                          })()
                        : '-'}
                    </span>
                  </SelectTrigger>
                  <SelectContent>
                    {admins.map((m) => (
                      <SelectItem key={m.user_id} value={String(m.user_id)}>
                        {m.display_name || m.username || `#${m.user_id}`}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" onClick={() => setTransferOpen(false)} disabled={acting}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" onClick={transferOwnership} disabled={acting || !transferTarget}>
              {t('orgPage.settings.transferConfirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={dissolveOpen} onOpenChange={setDissolveOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('orgPage.settings.dissolve')}</DialogTitle>
            <DialogDescription>{t('orgPage.settings.dissolveDesc')}</DialogDescription>
          </DialogHeader>
          <DialogBody className="space-y-4">
            <Label htmlFor="org-dissolve-name">{t('orgPage.settings.dissolveConfirmLabel', { name: org?.name })}</Label>
            <Input id="org-dissolve-name" value={dissolveName} onChange={(e) => setDissolveName(e.target.value)} autoFocus />
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDissolveOpen(false)} disabled={acting}>
              {t('common.cancel')}
            </Button>
            <Button variant="destructive" onClick={dissolve} disabled={acting || dissolveName.trim() !== org?.name}>
              {t('orgPage.settings.dissolve')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

DangerSection.propTypes = {
  orgId: PropTypes.number.isRequired,
  role: PropTypes.string,
  detail: PropTypes.object,
  onRefresh: PropTypes.func
};
