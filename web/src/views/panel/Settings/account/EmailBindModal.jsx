import { useEffect, useRef, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useTranslation } from 'react-i18next';
import Turnstile from 'react-turnstile';

import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import useRegister from 'hooks/useRegister';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody, DialogFooter } from '@/components/ui/dialog';

// Bind / change email. Ported from v1 Profile/component/EmailModal to shadcn.

export default function EmailBindModal({ open, onClose, turnstileEnabled, turnstileSiteKey }) {
  const { t } = useTranslation();
  const { sendVerificationCode } = useRegister();
  const [countdown, setCountdown] = useState(0);
  const [loading, setLoading] = useState(false);
  const [turnstileToken, setTurnstileToken] = useState('');
  const turnstileRef = useRef();

  const schema = z.object({
    email: z.string().email(t('profilePage.invalidEmail')),
    email_verification_code: z.string().min(1, t('profilePage.codeRequired'))
  });
  const { register, handleSubmit, watch, formState: { errors } } = useForm({
    resolver: zodResolver(schema),
    defaultValues: { email: '', email_verification_code: '' }
  });

  useEffect(() => {
    if (countdown <= 0) return undefined;
    const id = setInterval(() => setCountdown((c) => c - 1), 1000);
    return () => clearInterval(id);
  }, [countdown]);

  const handleSendCode = async () => {
    const email = watch('email');
    if (!email) return showError(t('profilePage.enterEmail'));
    if (turnstileEnabled && !turnstileToken) return showError(t('profilePage.turnstileWait'));
    setLoading(true);
    const { success, message } = await sendVerificationCode(email, turnstileToken);
    setLoading(false);
    if (turnstileEnabled && turnstileRef.current) {
      turnstileRef.current.reset();
      setTurnstileToken('');
    }
    if (!success) return showError(message);
    setCountdown(30);
  };

  const onSubmit = async (values) => {
    setLoading(true);
    try {
      const res = await API.get(`/api/oauth/email/bind?email=${values.email}&code=${values.email_verification_code}`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('profilePage.emailBindSuccess'));
        onClose();
      } else {
        showError(message);
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('profilePage.bindEmail')}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)}>
          <DialogBody className="space-y-4">
            <div className="space-y-4">
              <Label htmlFor="email">Email</Label>
              <div className="flex gap-2">
                <Input id="email" type="email" autoComplete="email" {...register('email')} />
                <Button type="button" variant="outline" onClick={handleSendCode} disabled={countdown > 0 || loading} className="shrink-0">
                  {countdown > 0 ? `${t('profilePage.resend')}(${countdown})` : t('profilePage.getCode')}
                </Button>
              </div>
              {errors.email && <p className="text-sm text-destructive">{errors.email.message}</p>}
            </div>
            <div className="space-y-4">
              <Label htmlFor="email_verification_code">{t('profilePage.code')}</Label>
              <Input id="email_verification_code" {...register('email_verification_code')} />
              {errors.email_verification_code && <p className="text-sm text-destructive">{errors.email_verification_code.message}</p>}
            </div>
            {turnstileEnabled && (
              <Turnstile sitekey={turnstileSiteKey} ref={turnstileRef} onVerify={(token) => setTurnstileToken(token)} />
            )}
          </DialogBody>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>{t('profilePage.cancel')}</Button>
            <Button type="submit" disabled={loading}>{t('profilePage.submit')}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
