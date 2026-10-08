import PropTypes from 'prop-types';
import { useState } from 'react';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';

import { Dialog, DialogBody, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { API } from 'utils/api';
import { showError, showInfo, showSuccess, trims } from 'utils/common';
import { RedemptionTopup } from './TopupSections';

// ==============================|| PANEL — REDEMPTION DIALOG ||============================== //
// 兑换码弹层:输入框 + 兑换按钮,站点配了 top_up_link 时附「获取兑换码」入口。
// 兑换成功后把到账额度回传给调用方刷新余额卡(组织上下文由调用方刷新组织池)。

export default function RedemptionDialog({ open, onClose, onSuccess }) {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);

  const [redemptionCode, setRedemptionCode] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const topUp = async () => {
    if (redemptionCode === '') {
      showInfo(t('topupCard.inputPlaceholder'));
      return;
    }
    setIsSubmitting(true);
    try {
      const res = await API.post('/api/user/topup', { key: trims(redemptionCode) });
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(t('topupCard.topupsuccess'));
        setRedemptionCode('');
        onSuccess?.(data);
      } else {
        showError(message);
      }
    } catch (err) {
      showError(t('topupCard.requestFailed'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const openTopUpLink = () => {
    if (!siteInfo.top_up_link) {
      showError(t('topupCard.adminSetupRequired'));
      return;
    }
    window.open(siteInfo.top_up_link, '_blank');
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t('topupCard.redemptionCodeTopup')}</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <RedemptionTopup
            t={t}
            redemptionCode={redemptionCode}
            setRedemptionCode={setRedemptionCode}
            topUp={topUp}
            isSubmitting={isSubmitting}
            siteInfo={siteInfo}
            openTopUpLink={openTopUpLink}
          />
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}

RedemptionDialog.propTypes = {
  open: PropTypes.bool,
  onClose: PropTypes.func.isRequired,
  onSuccess: PropTypes.func
};
