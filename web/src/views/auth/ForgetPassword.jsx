import { useEffect, useState } from 'react';
import { Link, Navigate } from 'react-router';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import Turnstile from 'react-turnstile';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import AuthShell from '@/components/auth/AuthShell';
import { API } from 'utils/api';
import { showError, showInfo, showSuccess } from 'utils/common';
import { isExternalAccountSystem } from 'utils/authAvailability';

// ==============================|| AUTH — FORGET PASSWORD ||============================== //

export default function ForgetPassword() {
  const { t } = useTranslation();
  const siteInfo = useSelector((state) => state.siteInfo);

  const [sendEmail, setSendEmail] = useState(false);
  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');
  const [disableButton, setDisableButton] = useState(false);
  const [countdown, setCountdown] = useState(30);

  const schema = z.object({
    email: z.string().min(1, t('registerForm.emailRequired')).email(t('registerForm.validEmailRequired')).max(255)
  });

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting }
  } = useForm({ resolver: zodResolver(schema), defaultValues: { email: '' } });

  const handleFailure = (message) => {
    showError(message);
    setDisableButton(false);
    setCountdown(30);
  };

  const onSubmit = async (values) => {
    setDisableButton(true);
    if (turnstileEnabled && turnstileToken === '') {
      showInfo(t('registerForm.verificationInfo'));
      return;
    }
    try {
      const res = await API.get(`/api/reset_password?email=${values.email}&turnstile=${turnstileToken}`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('registerForm.restSendEmail'));
        setSendEmail(true);
      } else {
        handleFailure(message);
      }
    } catch (error) {
      handleFailure(t('common.serverError'));
    }
  };

  useEffect(() => {
    let interval = null;
    if (disableButton && countdown > 0) {
      interval = setInterval(() => setCountdown(countdown - 1), 1000);
    } else if (countdown === 0) {
      setDisableButton(false);
      setCountdown(30);
    }
    return () => clearInterval(interval);
  }, [disableButton, countdown]);

  useEffect(() => {
    if (siteInfo.turnstile_check) {
      setTurnstileEnabled(true);
      setTurnstileSiteKey(siteInfo.turnstile_site_key);
    }
  }, [siteInfo]);

  // 外部账号体系下密码在登录服务那里找回,本站没有这条链路(放在所有 hook 之后)
  if (isExternalAccountSystem(siteInfo)) {
    return <Navigate to="/login" replace />;
  }

  return (
    <AuthShell title={t('login.forgetPassword')}>
      {sendEmail ? (
        <p className="py-5 text-center text-lg font-semibold text-foreground">{t('registerForm.restSendEmail')}</p>
      ) : (
        <form noValidate onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <FormField label="Email" htmlFor="email" error={errors.email?.message}>
            <Input id="email" type="text" {...register('email')} />
          </FormField>

          {turnstileEnabled && <Turnstile sitekey={turnstileSiteKey} onVerify={(token) => setTurnstileToken(token)} />}

          <Button type="submit" disabled={isSubmitting || disableButton} className="h-11 w-full font-semibold">
            {disableButton ? t('common.again', { count: countdown }) : t('common.submit')}
          </Button>
        </form>
      )}

      <p className="mt-6 text-center text-sm text-muted-foreground">
        <Link to="/login" className="font-medium text-primary-text hover:underline">
          {t('menu.login')}
        </Link>
      </p>
    </AuthShell>
  );
}
