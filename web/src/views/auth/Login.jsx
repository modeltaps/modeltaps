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
import OAuthButtons from '@/components/auth/OAuthButtons';
import useLogin from 'hooks/useLogin';
import { getOIDCEndpoint, onWebAuthnClicked, showSuccess } from 'utils/common';
import {
  adminLoginEnabled,
  emailCodeLoginEnabled,
  isExternalAccountSystem,
  oidcAutoRedirectTarget,
  passkeyLoginEnabled,
  passwordLoginEnabled,
  passwordRegisterEnabled,
  siteInfoLoaded,
  socialLoginEnabled
} from 'utils/authAvailability';

// ==============================|| AUTH — LOGIN ||============================== //
// 账号体系二选一:
//   external —— 本站不画登录框,直接跳到身份提供方;提供方没配好时只给一句提示(和 root 的应急入口)。
//   builtin  —— 一套骨架:社交登录按钮 → 本站表单(邮箱 / 用户名 + 密码,或邮箱 + 验证码)→ 通行密钥 → 注册。
// 按钮上只出现第三方账号名(GitHub、微信),永远不出现身份提供方的名字。

const RESEND_SECONDS = 60;

// 状态未知与外部模式直达共用同一个不带文字的转圈:跳转前一闪而过的过渡页没有价值。
const Spinner = () => (
  <div className="flex min-h-[40vh] items-center justify-center">
    <Loader2 className="size-8 animate-spin text-muted-foreground" aria-hidden="true" />
  </div>
);

export default function Login() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  // Honor an optional ?redirect= target so deep links (e.g. the Hero "Get API key" CTA)
  // return the user to their destination. Restricted to internal /panel paths.
  const redirectParam = searchParams.get('redirect');
  const redirectTo = redirectParam && redirectParam.startsWith('/panel') ? redirectParam : '/panel/dashboard';
  const { login, sendLoginCode, loginWithCode } = useLogin(redirectTo);
  const siteInfo = useSelector((state) => state.siteInfo);

  const external = isExternalAccountSystem(siteInfo);
  const passwordLogin = passwordLoginEnabled(siteInfo);
  const passwordRegister = passwordRegisterEnabled(siteInfo);
  const emailCodeLogin = emailCodeLoginEnabled(siteInfo);
  const passkeyLogin = passkeyLoginEnabled(siteInfo);
  const social = socialLoginEnabled(siteInfo);

  // 外部模式直达身份提供方:直接访问 /login 时服务端已 302,这里只覆盖站内跳转(如 401 后 navigate('/login'))。
  const [redirectFailed, setRedirectFailed] = useState(false);
  const autoTarget = redirectFailed ? '' : oidcAutoRedirectTarget(siteInfo);
  const redirectStarted = useRef(false);
  useEffect(() => {
    if (!autoTarget || redirectStarted.current) return;
    redirectStarted.current = true;
    getOIDCEndpoint(autoTarget).then((url) => {
      if (url) window.location.assign(url);
      else setRedirectFailed(true);
    });
  }, [autoTarget]);

  // 登录方式:密码 / 邮箱验证码。密码登录关闭但验证码开着时默认落到验证码。
  const [mode, setMode] = useState(passwordLogin || !emailCodeLogin ? 'password' : 'code');
  const [showPassword, setShowPassword] = useState(false);
  const [submitError, setSubmitError] = useState(null);
  const [codeSentTo, setCodeSentTo] = useState('');
  const [countdown, setCountdown] = useState(0);
  const [sendingCode, setSendingCode] = useState(false);
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

  useEffect(() => {
    if (countdown <= 0) return undefined;
    const timer = setTimeout(() => setCountdown((v) => v - 1), 1000);
    return () => clearTimeout(timer);
  }, [countdown]);

  const passwordSchema = z.object({
    username: z.string().min(1, t('login.usernameRequired')).max(255),
    password: z.string().min(1, t('login.passwordRequired')).max(255)
  });
  const codeSchema = z.object({
    email: z.string().min(1, t('registerForm.emailRequired')).email(t('registerForm.validEmailRequired')).max(255),
    code: z.string().min(1, t('login.codeRequired')).max(32)
  });

  const {
    register,
    handleSubmit,
    watch,
    getValues,
    formState: { errors, isSubmitting }
  } = useForm({
    resolver: zodResolver(mode === 'code' ? codeSchema : passwordSchema),
    defaultValues: { username: '', password: '', email: '', code: '' }
  });

  const resetTurnstile = () => {
    if (turnstileEnabled && turnstileRef.current) {
      setTurnstileToken('');
      turnstileRef.current.reset();
    }
  };

  const onSubmit = async (values) => {
    setSubmitError(null);
    if (mode === 'password' && turnstileEnabled && turnstileToken === '') {
      setSubmitError(t('registerForm.verificationInfo'));
      return;
    }
    const result =
      mode === 'code' ? await loginWithCode(values.email, values.code) : await login(values.username, values.password, turnstileToken);
    if (!result.success) {
      if (result.message) setSubmitError(result.message);
      // Turnstile token 一次性，失败后需重置组件才能再次提交。
      resetTurnstile();
    }
  };

  const handleSendCode = async () => {
    setSubmitError(null);
    const email = (getValues('email') || '').trim();
    if (!codeSchema.shape.email.safeParse(email).success) {
      setSubmitError(t('registerForm.validEmailRequired'));
      return;
    }
    if (turnstileEnabled && turnstileToken === '') {
      setSubmitError(t('registerForm.verificationInfo'));
      return;
    }
    setSendingCode(true);
    const { success, message } = await sendLoginCode(email, turnstileToken);
    setSendingCode(false);
    resetTurnstile();
    if (!success) {
      if (message) setSubmitError(message);
      return;
    }
    setCodeSentTo(email);
    setCountdown(RESEND_SECONDS);
  };

  const switchMode = (next) => {
    setMode(next);
    setSubmitError(null);
  };

  // 旧的管理员逃生口地址并入 /login/admin(直接访问时后端也 301 过去)
  if (searchParams.get('local') === '1') {
    return <Navigate to="/login/admin" replace />;
  }

  // 状态未知时先转圈:此时所有开关都是初值(按内置模式算),外部模式下会先闪一帧本站登录表单。
  if (!siteInfoLoaded(siteInfo)) {
    return <Spinner />;
  }

  // 外部模式直达:不渲染标题、logo 与任何文案。
  if (autoTarget) {
    return <Spinner />;
  }

  // 外部模式但提供方没配好(或授权地址拿不到):本站没有可用的登录方式,只能提示;root 走应急入口。
  if (external) {
    return (
      <AuthShell title={t('menu.login')}>
        <p className="text-center text-sm text-muted-foreground">
          {redirectFailed ? t('login.redirectFailed') : t('login.externalNotConfigured')}
        </p>
        {adminLoginEnabled(siteInfo) && (
          <p className="mt-6 text-center text-sm text-muted-foreground">
            <Link to="/login/admin" className="font-medium text-primary-text hover:underline">
              {t('login.adminSignIn')}
            </Link>
          </p>
        )}
      </AuthShell>
    );
  }

  const passkeyButton = passkeyLogin && (
    <Button
      type="button"
      variant="outline"
      className="h-11 w-full gap-2 font-medium"
      onClick={() =>
        onWebAuthnClicked(
          mode === 'password' ? watch('username') : '',
          (msg) => setSubmitError(msg),
          (msg) => showSuccess(msg),
          () => navigate(redirectTo)
        )
      }
    >
      <Fingerprint />
      {t('login.usePasskey')}
    </Button>
  );

  const passwordFields = (
    <div className="space-y-5">
      <FormField label={t('login.emailOrUsername')} htmlFor="username" error={errors.username?.message}>
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
  );

  const codeFields = (
    <div className="space-y-5">
      <FormField label={t('login.email')} htmlFor="email" error={errors.email?.message}>
        <Input id="email" type="email" autoComplete="email" {...register('email')} />
      </FormField>

      <FormField
        label={t('login.code')}
        htmlFor="code"
        error={errors.code?.message}
        hint={codeSentTo ? t('login.codeSent', { email: codeSentTo }) : undefined}
      >
        {/* 发码按钮嵌在验证码框内,与注册页同形,375px 下输入框仍有空间 */}
        <div className="flex h-9 w-full items-center rounded-lg border border-input bg-background pl-3 pr-1 focus-within:border-primary">
          <input
            id="code"
            type="text"
            inputMode="text"
            autoComplete="one-time-code"
            className="h-full min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
            {...register('code')}
          />
          <Button
            type="button"
            size="sm"
            disabled={countdown > 0 || sendingCode || isSubmitting}
            onClick={handleSendCode}
            className="shrink-0 px-3 font-medium"
          >
            {countdown > 0 ? t('login.resendCode', { countdown }) : t('login.sendCode')}
          </Button>
        </div>
      </FormField>
    </div>
  );

  const showForm = passwordLogin || emailCodeLogin;
  const showPasswordForm = showForm && mode === 'password' && passwordLogin;
  const showCodeForm = showForm && mode === 'code' && emailCodeLogin;

  return (
    <AuthShell title={t('menu.login')}>
      <OAuthButtons divider={social && (showForm || passkeyLogin)} />

      {showForm && (
        <form noValidate onSubmit={handleSubmit(onSubmit)} className="space-y-6">
          {showPasswordForm && passwordFields}
          {showCodeForm && codeFields}

          {turnstileEnabled && <Turnstile sitekey={turnstileSiteKey} ref={turnstileRef} onVerify={(token) => setTurnstileToken(token)} />}

          {submitError && <p className="text-sm text-destructive">{submitError}</p>}

          <div className="space-y-3">
            <Button type="submit" disabled={isSubmitting} className="h-11 w-full font-semibold">
              {isSubmitting && <Loader2 className="size-4 animate-spin" />}
              {isSubmitting ? t('login.loggingIn') : t('menu.login')}
            </Button>

            <div className="flex items-center justify-between text-sm text-muted-foreground">
              {passwordLogin && emailCodeLogin ? (
                <button
                  type="button"
                  onClick={() => switchMode(mode === 'code' ? 'password' : 'code')}
                  className="font-medium text-primary-text hover:underline"
                >
                  {mode === 'code' ? t('login.usePassword') : t('login.useEmailCode')}
                </button>
              ) : (
                <span />
              )}
              {passwordLogin && (
                <Link to="/forgot-password" className="font-medium text-primary-text hover:underline">
                  {t('login.forgetPassword')}
                </Link>
              )}
            </div>
          </div>
        </form>
      )}

      {!showForm && submitError && <p className="text-sm text-destructive">{submitError}</p>}

      {passkeyButton && (
        <>
          {showForm && (
            <div className="flex items-center gap-3 py-4">
              <span className="h-px flex-1 bg-border" />
              <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                {t('login.or', { defaultValue: 'OR' })}
              </span>
              <span className="h-px flex-1 bg-border" />
            </div>
          )}
          {passkeyButton}
        </>
      )}

      {passwordRegister && (
        <p className="mt-6 text-center text-sm text-muted-foreground">
          {t('login.noAccount')}
          <Link to="/register" className="ml-1 font-medium text-primary-text hover:underline">
            {t('menu.signup')}
          </Link>
        </p>
      )}
    </AuthShell>
  );
}
