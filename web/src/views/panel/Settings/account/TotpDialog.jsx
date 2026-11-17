import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import { QRCode } from 'react-qrcode-logo';
import { Copy, Download, Loader2 } from 'lucide-react';

import { API } from 'utils/api';
import { brandName } from 'utils/brand';
import { copy, showError, showSuccess, timestamp2string } from 'utils/common';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody, DialogFooter } from '@/components/ui/dialog';

// ==============================|| SETTINGS — TWO-STEP VERIFICATION DIALOG ||============================== //
// Off: scan the QR code (or type the key) into an authenticator app, confirm with a code, then
// the ten recovery codes are shown once. On: a code from the app or a recovery code turns it off
// or replaces the recovery codes. The dialog cannot be dismissed while the codes are on screen,
// only with "I've saved them". Render it only while open (each opening starts afresh).

const tk = (k) => `settingsPage.security.${k}`;

// GET /api/user/totp → { enabled, enabled_time, recovery_codes_left }, or null when it fails.
export const fetchTotpStatus = async () => {
  try {
    const res = await API.get('/api/user/totp');
    return res.data?.success ? res.data.data : null;
  } catch {
    return null;
  }
};

export default function TotpDialog({ open, onOpenChange, status, account, onChanged }) {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);
  // The parent mounts the dialog only while it is open, so each opening starts from here.
  const [view, setView] = useState(status?.enabled ? 'manage' : 'loading'); // loading | setup | codes | manage
  const [setup, setSetup] = useState(null);
  const [codes, setCodes] = useState([]);
  const [code, setCode] = useState('');
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!open || status?.enabled) return;
    API.post('/api/user/totp/setup')
      .then((res) => {
        if (res.data?.success) {
          setSetup(res.data.data);
          setView('setup');
        } else {
          showError(res.data?.message);
          onOpenChange(false);
        }
      })
      .catch((err) => {
        showError(err.message);
        onOpenChange(false);
      });
    // Only on opening: a status change while open comes from this dialog.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const close = () => onOpenChange(false);
  // The recovery codes leave the screen only through "I've saved them".
  const requestClose = (next) => {
    if (!next && view !== 'codes') close();
  };

  const post = async (path, onSuccess) => {
    if (!code.trim()) {
      setError(t('login.totpCodeRequired'));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const res = await API.post(path, { code: code.trim() });
      if (res.data?.success) {
        setCode('');
        onSuccess(res.data.data);
      } else {
        setError(res.data?.message || null);
      }
    } catch (err) {
      setError(err.message || null);
    } finally {
      setBusy(false);
    }
  };

  const showCodes = (data) => {
    setCodes(Array.isArray(data?.recovery_codes) ? data.recovery_codes : []);
    setView('codes');
    onChanged?.();
  };

  const enable = () =>
    post('/api/user/totp/enable', (data) => {
      showSuccess(t(tk('totpTurnedOn')));
      showCodes(data);
    });
  const regenerate = () => post('/api/user/totp/recovery_codes', showCodes);
  const disable = () =>
    post('/api/user/totp/disable', () => {
      showSuccess(t(tk('totpTurnedOff')));
      onChanged?.();
      close();
    });

  const codesText = () => `${t(tk('totpRecoveryFile'), { account, site: brandName(siteInfo?.system_name) })}\n\n${codes.join('\n')}\n`;
  const download = () => {
    const url = URL.createObjectURL(new Blob([codesText()], { type: 'text/plain' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = 'recovery-codes.txt';
    a.click();
    URL.revokeObjectURL(url);
  };

  const codeField = (label) => (
    <FormField label={label} htmlFor="totp-dialog-code" error={error}>
      <Input
        id="totp-dialog-code"
        autoFocus
        type="text"
        autoComplete="one-time-code"
        spellCheck={false}
        maxLength={32}
        className="font-mono"
        value={code}
        onChange={(e) => setCode(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault();
            if (view === 'setup') enable();
          }
        }}
        disabled={busy}
      />
    </FormField>
  );

  let body;
  if (view === 'loading') {
    body = (
      <>
        <DialogHeader>
          <DialogTitle>{t(tk('totpSetupTitle'))}</DialogTitle>
        </DialogHeader>
        <DialogBody className="flex justify-center py-10">
          <Loader2 className="size-6 animate-spin text-muted-foreground" aria-hidden="true" />
        </DialogBody>
      </>
    );
  } else if (view === 'setup') {
    body = (
      <>
        <DialogHeader>
          <DialogTitle>{t(tk('totpSetupTitle'))}</DialogTitle>
          <DialogDescription>{t(tk('totpSetupScan'))}</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <div className="flex justify-center">
            {/* A white quiet zone keeps the code readable in the dark theme too. */}
            <div className="rounded-lg bg-white p-2" data-testid="totp-qr">
              <QRCode value={setup?.otpauth_url || ''} size={176} quietZone={4} ecLevel="M" fgColor="#1D1D1F" bgColor="#FFFFFF" />
            </div>
          </div>
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">{t(tk('totpSecret'))}</p>
            <div className="flex items-center gap-2">
              <code className="min-w-0 flex-1 break-all rounded-md bg-muted px-3 py-2 font-mono text-sm">{setup?.secret}</code>
              <Button type="button" variant="outline" size="sm" onClick={() => copy(setup?.secret || '')} aria-label={t('common.copy')}>
                <Copy className="size-4" />
              </Button>
            </div>
          </div>
          {codeField(t(tk('totpConfirmCode')))}
        </DialogBody>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={close} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button type="button" onClick={enable} disabled={busy}>
            {busy && <Loader2 className="size-4 animate-spin" />}
            {t(tk('enable'))}
          </Button>
        </DialogFooter>
      </>
    );
  } else if (view === 'codes') {
    body = (
      <>
        <DialogHeader>
          <DialogTitle>{t(tk('totpRecoveryTitle'))}</DialogTitle>
          <DialogDescription>{t(tk('totpRecoveryHint'))}</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <ul
            className="grid grid-cols-2 gap-x-6 gap-y-2 rounded-lg border border-border bg-muted/40 p-4 font-mono text-sm"
            data-testid="totp-recovery-codes"
          >
            {codes.map((c) => (
              <li key={c}>{c}</li>
            ))}
          </ul>
          <div className="flex gap-2">
            <Button type="button" variant="outline" size="sm" onClick={() => copy(codesText())}>
              <Copy className="size-4" />
              {t(tk('totpRecoveryCopy'))}
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={download}>
              <Download className="size-4" />
              {t(tk('totpRecoveryDownload'))}
            </Button>
          </div>
        </DialogBody>
        <DialogFooter>
          <Button type="button" onClick={close}>
            {t(tk('totpRecoverySaved'))}
          </Button>
        </DialogFooter>
      </>
    );
  } else {
    body = (
      <>
        <DialogHeader>
          <DialogTitle>{t(tk('mfa'))}</DialogTitle>
          <DialogDescription>{t(tk('totpManageHint'), { time: timestamp2string(status?.enabled_time || 0) })}</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <p className="text-sm text-muted-foreground">{t(tk('totpOn'), { count: status?.recovery_codes_left ?? 0 })}</p>
          {codeField(t(tk('totpCodeOrRecovery')))}
        </DialogBody>
        <DialogFooter>
          <Button type="button" variant="ghost" onClick={close} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button type="button" variant="outline" onClick={regenerate} disabled={busy}>
            {t(tk('totpNewRecoveryCodes'))}
          </Button>
          <Button type="button" variant="destructive" onClick={disable} disabled={busy}>
            {busy && <Loader2 className="size-4 animate-spin" />}
            {t(tk('totpTurnOff'))}
          </Button>
        </DialogFooter>
      </>
    );
  }

  return (
    <Dialog open={open} onOpenChange={requestClose}>
      <DialogContent>{body}</DialogContent>
    </Dialog>
  );
}

TotpDialog.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func,
  status: PropTypes.object,
  account: PropTypes.string,
  onChanged: PropTypes.func
};
