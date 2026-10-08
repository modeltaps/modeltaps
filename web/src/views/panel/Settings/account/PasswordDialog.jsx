import PropTypes from 'prop-types';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useTranslation } from 'react-i18next';

import { API } from 'utils/api';
import { showError, showSuccess } from 'utils/common';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogBody, DialogFooter } from '@/components/ui/dialog';

// 设置 / 修改本站密码。改密后后端会踢掉其它设备的会话,对话框里先说清楚。
// emergency:外部账号体系下 root 的应急密码,说明文案不同(只用于 /login/admin)。

export default function PasswordDialog({ open, onOpenChange, hasPassword, inputs, emergency = false, onSaved }) {
  const { t } = useTranslation();
  const emptyValues = { original_password: '', password: '', confirm_password: '' };
  const schema = z
    .object({
      original_password: hasPassword ? z.string().min(1, t('settingsPage.security.currentPasswordRequired')) : z.string(),
      password: z.string().min(8, t('profilePage.passwordLength')).max(64, t('profilePage.passwordLength')),
      confirm_password: z.string()
    })
    .refine((d) => d.password === d.confirm_password, { message: t('profilePage.passwordMismatch'), path: ['confirm_password'] });

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting }
  } = useForm({ resolver: zodResolver(schema), defaultValues: emptyValues });

  const close = () => {
    reset(emptyValues);
    onOpenChange(false);
  };

  const onSubmit = async (values) => {
    try {
      const payload = { ...inputs, password: values.password };
      if (hasPassword) payload.original_password = values.original_password;
      const res = await API.put('/api/user/self', payload);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('profilePage.updateSuccess'));
        close();
        onSaved?.();
      } else {
        showError(message);
      }
    } catch (err) {
      showError(err.message);
    }
  };

  const title = hasPassword ? t('settingsPage.security.changePassword') : t('settingsPage.security.setPassword');
  const hint = emergency
    ? t('settingsPage.security.emergencyPasswordHint')
    : hasPassword
      ? t('settingsPage.security.changePasswordHint')
      : t('settingsPage.security.setPasswordHint');

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? onOpenChange(true) : close())}>
      <DialogContent>
        <form noValidate onSubmit={handleSubmit(onSubmit)}>
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{hint}</DialogDescription>
          </DialogHeader>
          <DialogBody className="space-y-4">
            {hasPassword && (
              <FormField
                label={t('settingsPage.security.currentPassword')}
                htmlFor="original_password"
                error={errors.original_password?.message}
              >
                <Input id="original_password" type="password" autoComplete="current-password" {...register('original_password')} />
              </FormField>
            )}
            <FormField label={t('settingsPage.security.newPassword')} htmlFor="password" error={errors.password?.message}>
              <Input id="password" type="password" autoComplete="new-password" {...register('password')} />
            </FormField>
            <FormField label={t('profilePage.confirmPassword')} htmlFor="confirm_password" error={errors.confirm_password?.message}>
              <Input id="confirm_password" type="password" autoComplete="new-password" {...register('confirm_password')} />
            </FormField>
          </DialogBody>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={close} disabled={isSubmitting}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {t('common.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

PasswordDialog.propTypes = {
  open: PropTypes.bool,
  onOpenChange: PropTypes.func,
  hasPassword: PropTypes.bool,
  inputs: PropTypes.object,
  emergency: PropTypes.bool,
  onSaved: PropTypes.func
};
