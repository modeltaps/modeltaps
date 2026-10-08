import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody, DialogFooter } from '@/components/ui/dialog';
import { Select, SelectTrigger, SelectContent, SelectItem } from '@/components/ui/select';
import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import { useOrg } from 'contexts/OrgContext';

// ==============================|| SETTINGS — ORG DANGER ZONE ||============================== //
// 退出组织(任意成员,Owner 须先转让);转让所有权 / 解散组织(仅 Owner)。后端同样校验。
// 设置页可管理的组织未必是左上角的当前上下文,仅当两者一致时才回落到个人上下文。

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
      <Card className="space-y-4 border-destructive/40 p-6">
        <h3 className="text-base font-semibold text-destructive">{t('orgPage.settings.dangerZone')}</h3>

        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div className="space-y-0.5">
            <p className="text-sm font-medium">{t('orgPage.settings.leave')}</p>
            <p className="text-xs text-muted-foreground">{isOwner ? t('orgPage.settings.leaveOwnerHint') : t('orgPage.settings.leaveConfirm')}</p>
          </div>
          <Button variant="outline" className="shrink-0" disabled={isOwner} onClick={() => setLeaveOpen(true)}>
            {t('orgPage.settings.leave')}
          </Button>
        </div>

        {isOwner && (
          <>
            <div className="flex flex-col gap-2 border-t border-border pt-4 sm:flex-row sm:items-center sm:justify-between">
              <div className="space-y-0.5">
                <p className="text-sm font-medium">{t('orgPage.settings.transferOwnership')}</p>
                <p className="text-xs text-muted-foreground">{t('orgPage.settings.transferOwnershipDesc')}</p>
              </div>
              <Button variant="outline" className="shrink-0" onClick={() => setTransferOpen(true)}>
                {t('orgPage.settings.transferOwnership')}
              </Button>
            </div>
            <div className="flex flex-col gap-2 border-t border-border pt-4 sm:flex-row sm:items-center sm:justify-between">
              <div className="space-y-0.5">
                <p className="text-sm font-medium">{t('orgPage.settings.dissolve')}</p>
                <p className="text-xs text-muted-foreground">{t('orgPage.settings.dissolveDesc')}</p>
              </div>
              <Button
                variant="destructive"
                className="shrink-0"
                onClick={() => {
                  setDissolveName('');
                  setDissolveOpen(true);
                }}
              >
                {t('orgPage.settings.dissolve')}
              </Button>
            </div>
          </>
        )}
      </Card>

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
