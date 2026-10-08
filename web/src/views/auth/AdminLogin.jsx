import { useEffect, useRef, useState } from 'react';
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useSelector } from 'react-redux';
import { useTranslation } from 'react-i18next';
import Turnstile from 'react-turnstile';
import { Eye, EyeOff, Fingerprint, Loader2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import AuthShell from '@/components/auth/AuthShell';
import useLogin from 'hooks/useLogin';
import { onWebAuthnClicked, showSuccess } from 'utils/common';
import { adminLoginEnabled, siteInfoLoaded } from 'utils/authAvailability';

// ==============================|| AUTH — ADMIN EMERGENCY LOGIN (/login/admin) ||============================== //
// 只在外部账号体系下存在:身份提供方不可用时 root 用本站的应急密码或通行密钥进后台改配置。
// 只走本站凭据,不出站;普通账号提交得到与「关闭密码登录」相同的拒绝文案(后端判定)。
// 内置账号模式(管理员走普通登录)或后台关闭了应急登录时,直接并入 /login。

export default function AdminLogin() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const redirectParam = searchParams.get('redirect');
  const redirectTo = redirectParam && redirectParam.startsWith('/panel') ? redirectParam : '/panel/dashboard';
  const { login } = useLogin(redirectTo);
  const siteInfo = useSelector((state) => state.siteInfo);

  const [showPassword, setShowPassword] = useState(false);
  const [submitError, setSubmitError] = useState(null);
  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');
  const turnstileRef = useRef();

  useEffect(() => {
    if (siteInfo.turnstile_check) {
      setTurnstileEnabled(true);
      setTurnstileSiteKey(siteInfo.turnstile_site_key);
    }
  }, [siteInfo]);

  const schema = z.object({
    username: z.string().min(1, t('login.usernameRequired')).max(255),
    password: z.string().min(1, t('login.passwordRequired')).max(255)
  });

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors, isSubmitting }
  } = useForm({ resolver: zodResolver(schema), defaultValues: { username: '', password: '' } });

  // 状态未知时先转圈:此时判定只会拿到初值(非外部模式)而把 root 弹去 /login,而 /login 会再直达
  // 身份提供方 —— 恰好是这条逃生路径最需要可用的时候。
  if (!siteInfoLoaded(siteInfo)) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <Loader2 className="size-8 animate-spin text-muted-foreground" aria-hidden="true" />
      </div>
    );
  }

  if (!adminLoginEnabled(siteInfo)) {
    return <Navigate to="/login" replace />;
  }

  const onSubmit = async (values) => {
    setSubmitError(null);
    if (turnstileEnabled && turnstileToken === '') {
      setSubmitError(t('registerForm.verificationInfo'));
      return;
    }
    const { success, message } = await login(values.username, values.password, turnstileToken);
    if (!success) {
      if (message) setSubmitError(message);
      if (turnstileEnabled && turnstileRef.current) {
        setTurnstileToken('');
        turnstileRef.current.reset();
      }
    }
  };

  return (
    <AuthShell title={t('login.adminSignIn')}>
      <p className="mb-6 text-center text-sm text-muted-foreground">{t('login.adminSignInHint')}</p>

      <form noValidate onSubmit={handleSubmit(onSubmit)} className="space-y-6">
        <div className="space-y-5">
          <FormField label={t('login.username')} htmlFor="username" error={errors.username?.message}>
            <Input id="username" type="text" autoComplete="username" {...register('username')} />
          </FormField>

          <FormField label={t('login.password')} htmlFor="password" error={errors.password?.message}>
            <div className="relative">
              <Input
                id="password"
                type={showPassword ? 'text' : 'password'}
                autoComplete="current-password"
                className="pr-11"
                {...register('password')}
              />
              <button
                type="button"
                aria-label="toggle password visibility"
                onClick={() => setShowPassword((v) => !v)}
                className="absolute inset-y-0 right-0 flex items-center rounded-r-md px-3 text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                {showPassword ? <Eye className="size-5" /> : <EyeOff className="size-5" />}
              </button>
            </div>
          </FormField>
        </div>

        {turnstileEnabled && <Turnstile sitekey={turnstileSiteKey} ref={turnstileRef} onVerify={(token) => setTurnstileToken(token)} />}

        {/* 通行密钥报错也落在这里:表单常驻,不会被折叠区藏住。 */}
        {submitError && <p className="text-sm text-destructive">{submitError}</p>}

        <div className="space-y-3">
          <Button type="submit" disabled={isSubmitting} className="h-11 w-full font-semibold">
            {isSubmitting && <Loader2 className="size-4 animate-spin" />}
            {isSubmitting ? t('login.loggingIn') : t('menu.login')}
          </Button>

          <Button
            type="button"
            variant="outline"
            className="h-11 w-full gap-2 font-medium"
            onClick={() =>
              onWebAuthnClicked(
                watch('username'),
                (msg) => setSubmitError(msg),
                (msg) => showSuccess(msg),
                () => navigate(redirectTo)
              )
            }
          >
            <Fingerprint />
            {t('login.usePasskey')}
          </Button>
        </div>
      </form>

      <p className="mt-6 text-center text-sm text-muted-foreground">
        <Link to="/login" className="font-medium text-primary-text hover:underline">
          {t('login.backToSignIn')}
        </Link>
      </p>
    </AuthShell>
  );
}
