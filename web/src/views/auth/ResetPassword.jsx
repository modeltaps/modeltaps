import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router';
import { useTranslation } from 'react-i18next';

import { Button } from '@/components/ui/button';
import AuthShell from '@/components/auth/AuthShell';
import { API } from 'utils/api';
import { copy, showError } from 'utils/common';

// ==============================|| AUTH — RESET PASSWORD ||============================== //

export default function ResetPassword() {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const [inputs, setInputs] = useState({ email: '', token: '' });
  const [newPassword, setNewPassword] = useState('');

  const submit = async () => {
    try {
      const res = await API.post(`/api/user/reset`, inputs);
      const { success, message } = res.data;
      if (success) {
        const password = res.data.data;
        setNewPassword(password);
        copy(password, t('auth.newPassword'));
      } else {
        showError(message);
      }
    } catch (error) {
      return;
    }
  };

  useEffect(() => {
    const email = searchParams.get('email');
    const token = searchParams.get('token');
    setInputs({ token, email });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <AuthShell title={t('login.forgetPassword')}>
      <div className="flex flex-col items-center justify-center gap-6 py-6 text-center">
        {!inputs.email || !inputs.token ? (
          <p className="text-lg font-semibold text-foreground">{t('auth.invalidLink')}</p>
        ) : newPassword ? (
          <div className="w-full rounded-md border border-destructive/40 bg-destructive/10 px-4 py-3 text-left text-sm text-foreground">
            {t('auth.newPasswordInfo')} <b>{newPassword}</b>
            <br />
            {t('auth.newPasswordEdit')}
          </div>
        ) : (
          <Button onClick={submit} type="submit" className="h-11 w-full font-semibold">
            {t('auth.restPasswordClick')}
          </Button>
        )}
      </div>

      <p className="mt-2 text-center text-sm text-muted-foreground">
        <Link to="/login" className="font-medium text-primary-text hover:underline">
          {t('menu.login')}
        </Link>
      </p>
    </AuthShell>
  );
}
