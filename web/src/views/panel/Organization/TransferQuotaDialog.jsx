import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody, DialogFooter } from '@/components/ui/dialog';
import QuotaInput from '@/components/QuotaInput';
import { API } from 'utils/api';
import { renderBalance, showError, showSuccess } from 'utils/common';
import { useOrg } from 'contexts/OrgContext';

// ==============================|| ORGANIZATION — PERSONAL → ORG QUOTA TRANSFER (a5) ||============================== //
// Owner/Admin moves personal quota into the org pool. One-way; backend is the
// final validator (feature flag + balance), the UI pre-checks the amount.

export default function TransferQuotaDialog({ open, onOpenChange, orgId, onTransferred }) {
  const { t } = useTranslation();
  const { refreshOrgDetail } = useOrg();
  const [amount, setAmount] = useState('');
  const [personalQuota, setPersonalQuota] = useState(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setAmount('');
    // /api/user/self 不在改写规则内,组织上下文中依然返回个人账户
    API.get('/api/user/self')
      .then((res) => {
        const { success, message, data } = res.data;
        if (success) setPersonalQuota(data?.quota ?? null);
        else showError(message || t('usagePage.loadFailed'));
      })
      .catch((error) => showError(error));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const quota = parseInt(amount, 10) || 0;
  const overBalance = personalQuota != null && quota > personalQuota;
  const invalid = quota <= 0 || overBalance;

  const submit = async () => {
    if (invalid) return;
    setSaving(true);
    try {
      const res = await API.post(`/api/org/${orgId}/quota/transfer`, { quota });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('orgPage.overview.transferSuccess'));
        onTransferred?.();
        // 池余额变化,同步刷新侧栏 / 仪表盘读取的 orgDetail(UX-25)
        refreshOrgDetail();
        onOpenChange(false);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('orgPage.overview.transferQuotaTitle')}</DialogTitle>
          <DialogDescription>{t('orgPage.overview.transferQuotaDesc')}</DialogDescription>
        </DialogHeader>
        <DialogBody>
          <QuotaInput
            name="transfer_amount"
            label={t('orgPage.overview.transferAmount')}
            value={amount}
            onChange={setAmount}
            error={overBalance}
            helperText={
              personalQuota != null ? t('orgPage.overview.personalBalance', { balance: renderBalance(personalQuota) }) : undefined
            }
          />
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
            {t('common.cancel')}
          </Button>
          <Button onClick={submit} disabled={saving || invalid}>
            {saving && <Loader2 className="size-4 animate-spin" />}
            {t('common.submit')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

TransferQuotaDialog.propTypes = {
  open: PropTypes.bool.isRequired,
  onOpenChange: PropTypes.func.isRequired,
  orgId: PropTypes.number.isRequired,
  onTransferred: PropTypes.func
};
