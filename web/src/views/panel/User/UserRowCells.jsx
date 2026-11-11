import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { MoreHorizontal, PanelRight, Trash2, Wallet, User as UserIcon, ShieldCheck, ShieldUser } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Switch } from '@/components/ui/switch';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import QuotaInput, { QUOTA_UNIT_CURRENCY } from '@/components/QuotaInput';
import { showError, calculateQuota, renderNumber } from 'utils/common';

// 启用开关:以行数据为准(操作成功后列表重拉),请求期间禁用防连点。
export function StatusCell({ item, manageUser }) {
  const { t } = useTranslation();
  const [pending, setPending] = useState(false);
  const enabled = item.status === 1;
  const onToggle = async () => {
    setPending(true);
    try {
      await manageUser(item.id, 'status', enabled ? 2 : 1);
    } finally {
      setPending(false);
    }
  };
  return (
    <Switch
      size="sm"
      checked={enabled}
      disabled={pending}
      onCheckedChange={onToggle}
      aria-label={enabled ? t('userPage.statusEnabled') : t('userPage.statusDisabled')}
      title={enabled ? t('userPage.statusEnabled') : t('userPage.statusDisabled')}
    />
  );
}

// 状态:小圆点 + 文字。
export function StatusDot({ status }) {
  const { t } = useTranslation();
  const enabled = status === 1;
  return (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap text-sm">
      <span className={cn('size-1.5 rounded-full', enabled ? 'bg-success' : 'bg-destructive')} />
      {enabled ? t('userPage.statusEnabled') : t('userPage.statusDisabled')}
    </span>
  );
}

// 调整余额对话框:正数增加、负数扣减(不超过当前余额),附备注。
export function QuotaDialog({ item, open, onOpenChange, manageUser }) {
  const { t } = useTranslation();
  const [quotaDelta, setQuotaDelta] = useState('');
  const [remark, setRemark] = useState('');

  const close = () => {
    onOpenChange(false);
    setQuotaDelta('');
    setRemark('');
  };

  const applyQuota = async () => {
    const quota = Number(quotaDelta);
    if (!quota) {
      showError(t('userPage.changeQuotaNotEmpty'));
      return;
    }
    if (quota < 0 && Math.abs(quota) > Math.max(0, item.quota)) {
      showError(t('userPage.changeQuotaNotEnough'));
      return;
    }
    const res = await manageUser(item.id, 'quota', { quota, remark });
    if (res?.success) close();
  };

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? onOpenChange(true) : close())}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t('userPage.changeQuota')}</DialogTitle>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <QuotaInput
            id={`q-${item.id}`}
            name="quotaDelta"
            label={t('userPage.changeQuota')}
            value={quotaDelta}
            onChange={setQuotaDelta}
            defaultUnit={QUOTA_UNIT_CURRENCY}
            maxDeduct={Math.max(0, item.quota)}
            helperText={t('userPage.changeQuotaHelperText', {
              tokens: renderNumber(Math.max(0, item.quota)),
              money: '$' + calculateQuota(Math.max(0, item.quota), 6)
            })}
          />
          <div className="space-y-2.5">
            <Label htmlFor={`r-${item.id}`}>{t('userPage.quotaRemark')}</Label>
            <Input id={`r-${item.id}`} value={remark} onChange={(e) => setRemark(e.target.value)} />
          </div>
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={close}>
            {t('userPage.cancel')}
          </Button>
          <Button onClick={applyQuota}>{t('common.submit')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// 行 / 面板的「⋯」菜单:打开(仅列表行)、调整余额、设角色、删除(先确认)。
export function ActionsCell({ item, manageUser, onOpen, triggerClassName = 'size-7' }) {
  const { t } = useTranslation();
  const [openDelete, setOpenDelete] = useState(false);
  const [openQuota, setOpenQuota] = useState(false);

  const roleActions =
    item.role === 100
      ? []
      : [
          { role: 1, label: t('userPage.setCommonUser'), icon: UserIcon },
          { role: 3, label: t('userPage.setReliable'), icon: ShieldCheck },
          { role: 10, label: t('userPage.setAdmin'), icon: ShieldUser }
        ].filter((a) => a.role !== item.role);

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button type="button" variant="ghost" size="icon" className={triggerClassName} aria-label={t('userPage.moreActions')}>
            <MoreHorizontal className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-48">
          {onOpen && (
            <DropdownMenuItem onClick={() => onOpen(item.id)}>
              <PanelRight className="size-4" />
              {t('objects.open')}
            </DropdownMenuItem>
          )}
          <DropdownMenuItem onClick={() => setOpenQuota(true)}>
            <Wallet className="size-4" />
            {t('userPage.changeQuota')}
          </DropdownMenuItem>
          {roleActions.length > 0 && <DropdownMenuSeparator />}
          {roleActions.map(({ role, label, icon: Icon }) => (
            <DropdownMenuItem key={role} onClick={() => manageUser(item.id, 'set_role', role)}>
              <Icon className="size-4" />
              {label}
            </DropdownMenuItem>
          ))}
          <DropdownMenuSeparator />
          <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => setOpenDelete(true)}>
            <Trash2 className="size-4" />
            {t('common.delete')}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={openDelete} onOpenChange={setOpenDelete}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('common.delete')}</DialogTitle>
          </DialogHeader>
          <DialogBody>
            <p className="text-sm text-muted-foreground">
              {t('common.deleteConfirm', { title: `${t('userPage.userLabel')} "${item.username}"` })}
            </p>
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpenDelete(false)}>
              {t('userPage.cancel')}
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                setOpenDelete(false);
                manageUser(item.id, 'delete', '');
              }}
            >
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <QuotaDialog item={item} open={openQuota} onOpenChange={setOpenQuota} manageUser={manageUser} />
    </>
  );
}
