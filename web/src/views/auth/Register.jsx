import { useEffect, useState } from 'react';
import { Navigate, Link, useSearchParams } from 'react-router';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import Turnstile from 'react-turnstile';
import { Eye, EyeOff, Loader2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import AuthShell from '@/components/auth/AuthShell';
import OAuthButtons from '@/components/auth/OAuthButtons';
import useRegister from 'hooks/useRegister';
import { strengthColor, strengthIndicator } from 'utils/password-strength';
import { showError, showInfo } from 'utils/common';
import { isExternalAccountSystem, passwordRegisterEnabled } from 'utils/authAvailability';

// ==============================|| AUTH — REGISTER ||============================== //

export default function Register() {
  const { t } = useTranslation();
  const { register: registerUser, sendVerificationCode } = useRegister('/login');
  const siteInfo = useSelector((state) => state.siteInfo);
  const [searchParams] = useSearchParams();
  // 站点关闭密码注册时,直接访问 /register 只给提示与第三方入口(UX-33);key 缺失时保持既有行为。
  const passwordRegister = passwordRegisterEnabled(siteInfo);

  const [showPassword, setShowPassword] = useState(false);
  const [countdown, setCountdown] = useState(30);
  const [disableButton, setDisableButton] = useState(false);
  const [showEmailVerification, setShowEmailVerification] = useState(false);
  const [showInviteCode, setShowInviteCode] = useState(false);
  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');
  const [level, setLevel] = useState();
  const [strength, setStrength] = useState(0);
  const [submitError, setSubmitError] = useState(null);

  const {
    register,
    handleSubmit,
    watch,
    setError,
    formState: { errors, isSubmitting }
  } = useForm({
    defaultValues: { username: '', password: '', confirmPassword: '', email: '', verification_code: '', invite_code: '' }
  });

  useEffect(() => {
    const affCode = searchParams.get('aff');
    if (affCode) localStorage.setItem('aff', affCode);
    setShowEmailVerification(siteInfo.email_verification);
    setShowInviteCode(siteInfo.invite_code_register);
    if (siteInfo.turnstile_check) {
      setTurnstileEnabled(true);
      setTurnstileSiteKey(siteInfo.turnstile_site_key);
    }
  }, [siteInfo, searchParams]);

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

  const changePassword = (value) => {
    const temp = strengthIndicator(value);
    setStrength(temp);
    setLevel(strengthColor(temp));
  };

  const handleSendCode = async () => {
    const email = watch('email');
    if (!email) return showError(t('registerForm.enterEmail'));
    if (turnstileEnabled && turnstileToken === '') return showError(t('registerForm.turnstileError'));
    setDisableButton(true);
    const { success, message } = await sendVerificationCode(email, turnstileToken);
    if (!success) {
      showError(message);
      setDisableButton(false);
    }
  };

  const buildSchema = () =>
    z
      .object({
        username: z.string().min(1, t('registerForm.usernameRequired')).max(255),
        password: z.string().min(8, t('registerForm.passwordLength')).max(64, t('registerForm.passwordLength')),
        confirmPassword: z.string().min(1, t('registerForm.confirmPasswordRequired')),
        email: showEmailVerification
          ? z.string().min(1, t('registerForm.emailRequired')).email(t('registerForm.validEmailRequired')).max(255)
          : z.string().optional(),
        verification_code: showEmailVerification
          ? z.string().min(1, t('registerForm.verificationCodeRequired')).max(255)
          : z.string().optional(),
        invite_code: showInviteCode ? z.string().min(1, t('registerForm.inviteCodeRequired')).max(255) : z.string().optional()
      })
      .refine((d) => d.password === d.confirmPassword, { path: ['confirmPassword'], message: t('registerForm.passwordsNotMatch') });

  const onSubmit = async (values) => {
    setSubmitError(null);
    const parsed = buildSchema().safeParse(values);
    if (!parsed.success) {
      parsed.error.issues.forEach((i) => setError(i.path[0], { message: i.message }));
      return;
    }
    if (turnstileEnabled && turnstileToken === '') return showInfo(t('registerForm.verificationInfo'));
    const { success, message } = await registerUser(values, turnstileToken);
    if (!success && message) setSubmitError(message);
  };

  // 外部账号体系下注册在身份提供方完成:/register 直接并入 /login(那里再跳去提供方)。
  if (isExternalAccountSystem(siteInfo)) {
    return <Navigate to="/login" replace />;
  }

  if (!passwordRegister) {
    return (
      <AuthShell title={t('menu.signup')}>
        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">{t('registerForm.passwordRegisterDisabled')}</p>
          <OAuthButtons divider={false} />
        </div>
        <p className="mt-6 text-center text-sm text-muted-foreground">
          <Link to="/login" className="font-medium text-primary-text hover:underline">
            {t('menu.login')}
          </Link>
        </p>
      </AuthShell>
    );
  }

  return (
    <AuthShell title={t('menu.signup')}>
      <form noValidate onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <FormField label={t('registerForm.username')} htmlFor="username" error={errors.username?.message}>
          <Input id="username" type="text" autoComplete="username" {...register('username')} />
        </FormField>

        <FormField label={t('registerForm.password')} htmlFor="password" error={errors.password?.message}>
          <div className="relative">
            <Input
              id="password"
              type={showPassword ? 'text' : 'password'}
              className="pr-10"
              {...register('password', { onChange: (e) => changePassword(e.target.value) })}
            />
            <button
              type="button"
              aria-label="toggle password visibility"
              onClick={() => setShowPassword((v) => !v)}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
            >
              {showPassword ? <Eye className="size-5" /> : <EyeOff className="size-5" />}
            </button>
          </div>
          {strength !== 0 && (
            <div className="flex items-center gap-2">
              <span className="h-2 w-[85px] rounded-full" style={{ backgroundColor: level?.color }} />
              <span className="text-xs text-muted-foreground">{level?.label}</span>
            </div>
          )}
        </FormField>

        <FormField label={t('registerForm.confirmPassword')} htmlFor="confirmPassword" error={errors.confirmPassword?.message}>
          <Input id="confirmPassword" type={showPassword ? 'text' : 'password'} {...register('confirmPassword')} />
        </FormField>

        {showEmailVerification && (
          <>
            <FormField label="Email" htmlFor="email" error={errors.email?.message}>
              {/* v1-style end adornment: full-width field with the send-code button embedded, so the input keeps room at 375px */}
              <div className="flex h-9 w-full items-center rounded-lg border border-input bg-background pl-3 pr-1 focus-within:border-primary">
                <input
                  id="email"
                  type="text"
                  className="h-full min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                  {...register('email')}
                />
                <Button
                  type="button"
                  size="sm"
                  disabled={disableButton || isSubmitting}
                  onClick={handleSendCode}
                  className="shrink-0 px-3 font-medium"
                >
                  {disableButton ? t('registerForm.resendCode', { countdown }) : t('registerForm.getCode')}
                </Button>
              </div>
            </FormField>
            <FormField label={t('registerForm.verificationCode')} htmlFor="verification_code" error={errors.verification_code?.message}>
              <Input id="verification_code" type="text" {...register('verification_code')} />
            </FormField>
          </>
        )}

        {showInviteCode && (
          <FormField label={t('registerForm.inviteCode')} htmlFor="invite_code" error={errors.invite_code?.message}>
            <Input id="invite_code" type="text" {...register('invite_code')} />
          </FormField>
        )}

        {submitError && <p className="text-sm text-destructive">{submitError}</p>}

        {turnstileEnabled && <Turnstile sitekey={turnstileSiteKey} onVerify={(token) => setTurnstileToken(token)} />}

        <Button type="submit" disabled={isSubmitting} className="h-11 w-full font-semibold">
          {isSubmitting && <Loader2 className="size-4 animate-spin" />}
          {isSubmitting ? t('registerForm.registering') : t('menu.signup')}
        </Button>
      </form>

      <p className="mt-6 text-center text-sm text-muted-foreground">
        <Link to="/login" className="font-medium text-primary-text hover:underline">
          {t('menu.login')}
        </Link>
      </p>
    </AuthShell>
  );
}
