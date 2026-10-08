import PropTypes from 'prop-types';
import { useState, useEffect, useCallback, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { QRCode } from 'react-qrcode-logo';

import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { API } from 'utils/api';
import { showError } from 'utils/common';
import successSvg from 'assets/images/success.svg';

// Tailwind/shadcn port of the v1 Topup PayDialog. Creates an order, then either
// redirects via a generated form (type 1) or shows a QR code (type 2), polling
// the order status until it succeeds.
export default function PayDialog({ open, onClose, amount, uuid, onSuccess }) {
  const { t } = useTranslation();
  const [message, setMessage] = useState(t('payDialog.initiating'));
  const [subMessage, setSubMessage] = useState(null);
  const [loading, setLoading] = useState(false);
  const [qrCodeUrl, setQrCodeUrl] = useState(null);
  const [success, setSuccess] = useState(false);
  const intervalRef = useRef(null);

  const clearValue = () => {
    setMessage(t('payDialog.initiating'));
    setSubMessage(null);
    setLoading(false);
    setQrCodeUrl(null);
    setSuccess(false);
  };

  const stopPolling = () => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
  };

  const pollOrderStatus = useCallback(
    (tradeNo) => {
      const id = setInterval(() => {
        API.get(`/api/user/order/status?trade_no=${tradeNo}`).then((response) => {
          if (response.data.success) {
            stopPolling();
            setMessage(t('payDialog.success'));
            setLoading(false);
            setSuccess(true);
            setQrCodeUrl(null);
            onSuccess?.();
          }
        });
      }, 3000);
      intervalRef.current = id;
    },
    [t, onSuccess]
  );

  const openPayUrl = (method, url, params) => {
    const form = document.createElement('form');
    form.method = method;
    form.action = url;
    form.target = '_blank';
    for (const key in params) {
      const input = document.createElement('input');
      input.name = key;
      input.value = params[key];
      form.appendChild(input);
    }
    document.body.appendChild(form);
    form.submit();
    document.body.removeChild(form);
  };

  useEffect(() => {
    if (!open) return;
    setMessage(t('payDialog.initiating'));
    setLoading(true);

    API.post('/api/user/order', { uuid, amount: Number(amount) }).then((response) => {
      if (!response.data.success) {
        showError(response.data.message);
        setLoading(false);
        onClose();
        return;
      }

      const { type, data } = response.data.data;
      if (type === 1) {
        setMessage(t('payDialog.waiting'));
        setSubMessage(
          <>
            {t('payDialog.redirectHint')}
            <a className="text-primary-text underline" href="#" onClick={() => openPayUrl(data.method, data.url, data.params)}>
              {t('payDialog.redirectHere')}
            </a>
          </>
        );
        openPayUrl(data.method, data.url, data.params);
      } else if (type === 2) {
        setQrCodeUrl(data.url);
        setLoading(false);
        setMessage(t('payDialog.scanToPay'));
      }
      pollOrderStatus(response.data.data.trade_no);
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, amount, uuid]);

  useEffect(() => stopPolling, []);

  const handleClose = () => {
    stopPolling();
    clearValue();
    onClose();
  };

  const handleOpenAlipay = (alipayUrl) => {
    if (alipayUrl && alipayUrl.startsWith('https://qr.alipay.com')) {
      window.open(alipayUrl, '_blank');
    }
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !v && handleClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t('payDialog.title')}</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col items-center justify-center gap-4 py-4">
          {loading && (
            <div
              aria-label="loading"
              className="size-24 animate-pulse bg-muted-foreground"
              style={{
                maskImage: 'url(/brand/logo-mark.svg)',
                WebkitMaskImage: 'url(/brand/logo-mark.svg)',
                maskRepeat: 'no-repeat',
                WebkitMaskRepeat: 'no-repeat',
                maskPosition: 'center',
                WebkitMaskPosition: 'center',
                maskSize: 'contain',
                WebkitMaskSize: 'contain'
              }}
            />
          )}
          {qrCodeUrl && <QRCode value={qrCodeUrl} size={256} qrStyle="dots" eyeRadius={20} fgColor="#1D1D1F" />}
          {success && <img src={successSvg} alt="success" height="100" />}
          <p className="text-lg font-semibold">{message}</p>
          {subMessage && <p className="text-sm text-muted-foreground">{subMessage}</p>}
          {qrCodeUrl && qrCodeUrl.startsWith('https://qr.alipay.com') && !success && (
            <Button onClick={() => handleOpenAlipay(qrCodeUrl)}>{t('payDialog.openAlipay')}</Button>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

PayDialog.propTypes = {
  open: PropTypes.bool,
  onClose: PropTypes.func,
  amount: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  uuid: PropTypes.string,
  onSuccess: PropTypes.func
};
