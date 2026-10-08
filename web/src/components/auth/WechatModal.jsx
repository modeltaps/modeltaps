import PropTypes from 'prop-types';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useTranslation } from 'react-i18next';

import { showError } from 'utils/common';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogBody } from '@/components/ui/dialog';

// ==============================|| AUTH — WECHAT VERIFICATION-CODE MODAL (shadcn) ||============================== //
// Ported from v1 views/Authentication/AuthForms/WechatModal to shadcn + react-hook-form.

export default function WechatModal({ open, handleClose, wechatLogin, qrCode }) {
  const { t } = useTranslation();

  const schema = z.object({
    code: z.string().min(1, t('login.codeRequired'))
  });

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting }
  } = useForm({ resolver: zodResolver(schema), defaultValues: { code: '' } });

  const onSubmit = async (values) => {
    const { success, message } = await wechatLogin(values.code);
    if (success) {
      handleClose();
    } else {
      showError(message || t('error.unknownError'));
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) handleClose();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('login.wechatVerificationCodeLogin')}</DialogTitle>
        </DialogHeader>

        <DialogBody className="space-y-4">
          <div className="flex flex-col items-center">
            <img src={qrCode} alt={t('login.qrCode')} className="h-auto max-h-[300px] w-auto max-w-[300px]" />
            <p className="mt-2 max-w-[300px] break-words text-center text-sm text-muted-foreground">{t('login.wechatLoginInfo')}</p>
          </div>

          <form className="space-y-4" onSubmit={handleSubmit(onSubmit)} noValidate>
            <FormField label={t('common.verificationCode')} htmlFor="wechat-code" error={errors.code?.message}>
              <Input id="wechat-code" autoFocus {...register('code')} />
            </FormField>
            <Button type="submit" className="w-full" disabled={isSubmitting}>
              {t('common.submit')}
            </Button>
          </form>
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}

WechatModal.propTypes = {
  open: PropTypes.bool,
  handleClose: PropTypes.func,
  wechatLogin: PropTypes.func,
  qrCode: PropTypes.string
};
