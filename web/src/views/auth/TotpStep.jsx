import { useRef, useState } from 'react';
import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';

// ==============================|| AUTH — TWO-STEP VERIFICATION STEP ||============================== //
// Shown after a correct password or email code when the account has two-step verification on:
// a code from the authenticator app (sent as soon as six digits are in), or a recovery code.
// The pending sign-in lives on the server; when it has timed out, onExpired takes the user back to
// the password form with the reason.

const SIX_DIGITS = /^\d{6}$/;

export default function TotpStep({ onVerify, onExpired, onCancel }) {
  const { t } = useTranslation();
  const [recovery, setRecovery] = useState(false);
  const [code, setCode] = useState('');
  const [error, setError] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  const busy = useRef(false);

  const verify = async (value) => {
    const trimmed = value.trim();
    if (!trimmed) {
      setError(t('login.totpCodeRequired'));
      return;
    }
    if (busy.current) return;
    busy.current = true;
    setSubmitting(true);
    setError(null);
    const result = await onVerify(trimmed);
    busy.current = false;
    setSubmitting(false);
    if (result.success) return;
    if (result.code === 'totp_challenge_expired') {
      onExpired(result.message);
      return;
    }
    setError(result.message || null);
    setCode('');
  };

  const onChange = (e) => {
    const value = e.target.value;
    setCode(value);
    if (!recovery && SIX_DIGITS.test(value.replace(/\s/g, ''))) verify(value.replace(/\s/g, ''));
  };

  const switchMode = () => {
    setRecovery((v) => !v);
    setCode('');
    setError(null);
  };

  return (
    <form
      noValidate
      className="space-y-6"
      onSubmit={(e) => {
        e.preventDefault();
        verify(code);
      }}
    >
      <p className="text-center text-sm text-muted-foreground">{recovery ? t('login.totpRecoveryHint') : t('login.totpHint')}</p>

      <FormField label={recovery ? t('login.recoveryCode') : t('login.totpCode')} htmlFor="totp-code" error={error}>
        <Input
          id="totp-code"
          key={recovery ? 'recovery' : 'totp'}
          autoFocus
          type="text"
          inputMode={recovery ? 'text' : 'numeric'}
          autoComplete={recovery ? 'off' : 'one-time-code'}
          autoCapitalize="characters"
          spellCheck={false}
          maxLength={recovery ? 32 : 8}
          placeholder={recovery ? 'XXXX-XXXX-XXXX-XXXX' : undefined}
          className={recovery ? 'font-mono' : 'font-mono tracking-[0.3em]'}
          value={code}
          onChange={onChange}
          disabled={submitting}
        />
      </FormField>

      <div className="space-y-3">
        <Button type="submit" disabled={submitting} className="h-11 w-full font-medium">
          {submitting && <Loader2 className="size-4 animate-spin" />}
          {t('login.totpVerify')}
        </Button>
        <div className="flex items-center justify-between text-sm">
          <button type="button" onClick={switchMode} className="font-medium text-primary-text hover:underline">
            {recovery ? t('login.useAuthenticator') : t('login.useRecoveryCode')}
          </button>
          <button type="button" onClick={onCancel} className="font-medium text-muted-foreground hover:underline">
            {t('login.backToSignIn')}
          </button>
        </div>
      </div>
    </form>
  );
}

TotpStep.propTypes = {
  onVerify: PropTypes.func.isRequired,
  onExpired: PropTypes.func.isRequired,
  onCancel: PropTypes.func.isRequired
};
