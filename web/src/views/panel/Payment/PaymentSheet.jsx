import { useEffect, useState } from 'react';
import PropTypes from 'prop-types';
import { useForm, Controller } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';

import { API } from 'utils/api';
import { trims } from 'utils/common';
import { toast } from '@/components/ui/sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetBody, SheetFooter } from '@/components/ui/sheet';
import { PaymentType, CurrencyType, PaymentConfig } from './paymentConfig';
import { configText } from 'i18n/configText';

const EMPTY = {
  type: 'epay',
  uuid: '',
  name: '',
  icon: '',
  notify_domain: '',
  fixed_fee: 0,
  percent_fee: 0,
  currency: 'CNY',
  currency_rate: 1,
  config: {},
  sort: 0,
  enable: true
};

export default function PaymentSheet({ open, paymentId, onClose, onSaved }) {
  const { t } = useTranslation();
  const [submitting, setSubmitting] = useState(false);
  const {
    register,
    handleSubmit,
    control,
    reset,
    watch,
    formState: { errors }
  } = useForm({ defaultValues: EMPTY });

  const type = watch('type');

  useEffect(() => {
    if (!open) return;
    if (paymentId) {
      loadPayment(paymentId);
    } else {
      reset(EMPTY);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, paymentId]);

  const loadPayment = async (id) => {
    try {
      const res = await API.get(`/api/payment/${id}`);
      const { success, message, data } = res.data;
      if (!success) {
        toast.error(message);
        return;
      }
      let config = {};
      try {
        config = data.config ? JSON.parse(data.config) : {};
      } catch (e) {
        config = {};
      }
      reset({ ...EMPTY, ...data, config });
    } catch (error) {
      toast.error(error.message);
    }
  };

  const onSubmit = async (raw) => {
    const values = trims({ ...raw });
    if (!values.name) return toast.error(t('validation.requiredName'));
    if (!values.icon) return toast.error(t('payment_edit.requiredIcon'));
    if (Number(values.fixed_fee) < 0) return toast.error(t('payment_edit.requiredFixedFee'));
    if (Number(values.percent_fee) < 0) return toast.error(t('payment_edit.requiredPercentFee'));
    if (!values.currency) return toast.error(t('payment_edit.requiredCurrency'));
    if (!(Number(values.currency_rate) > 0)) return toast.error(t('payment_edit.requiredCurrencyRate'));

    const config = JSON.stringify(values.config || {});
    setSubmitting(true);
    try {
      let res;
      if (paymentId) {
        res = await API.put('/api/payment/', { ...values, id: parseInt(paymentId, 10), config });
      } else {
        res = await API.post('/api/payment/', { ...values, config });
      }
      const { success, message } = res.data;
      if (success) {
        toast.success(paymentId ? t('payment_edit.updateOk') : t('payment_edit.addOk'));
        onSaved();
      } else {
        toast.error(message);
      }
    } catch (error) {
      toast.error(error.message);
    }
    setSubmitting(false);
  };

  const configFields = PaymentConfig[type] || {};

  return (
    <Sheet open={open} onOpenChange={(v) => !v && onClose()}>
      <SheetContent onClose={onClose}>
        <SheetHeader>
          <SheetTitle>{paymentId ? t('payment_edit.paymentEdit') : t('paymentGatewayPage.createPayment')}</SheetTitle>
        </SheetHeader>
        <form onSubmit={handleSubmit(onSubmit)} className="flex min-h-0 flex-1 flex-col">
          <SheetBody className="space-y-4">
            <FormField id="p-type" label={t('paymentGatewayPage.tableHeaders.type')} hint={t('payment_edit.paymentType')}>
              <Controller
                control={control}
                name="type"
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger id="p-type">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {Object.entries(PaymentType).map(([value, text]) => (
                        <SelectItem key={value} value={value}>
                          {t(text)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
            </FormField>

            <FormField id="p-name" label={t('paymentGatewayPage.tableHeaders.name')} required error={errors.name}>
              <Input id="p-name" {...register('name')} />
            </FormField>

            <FormField id="p-icon" label={t('paymentGatewayPage.tableHeaders.icon')} required error={errors.icon}>
              <Input id="p-icon" {...register('icon')} />
            </FormField>

            <FormField id="p-notify_domain" label={t('payment_edit.notifyDomain')} hint={t('payment_edit.notifyDomainTip')}>
              <Input id="p-notify_domain" {...register('notify_domain')} />
            </FormField>

            <FormField id="p-fixed_fee" label={t('paymentGatewayPage.tableHeaders.fixedFee')} hint={t('payment_edit.FixedTip')}>
              <Input id="p-fixed_fee" type="number" step="any" {...register('fixed_fee', { valueAsNumber: true })} />
            </FormField>

            <FormField id="p-percent_fee" label={t('paymentGatewayPage.tableHeaders.percentFee')} hint={t('payment_edit.percentTip')}>
              <Input id="p-percent_fee" type="number" step="any" {...register('percent_fee', { valueAsNumber: true })} />
            </FormField>

            <FormField id="p-currency" label={t('payment_edit.currencyType')} required hint={t('payment_edit.currencyTip')}>
              <Controller
                control={control}
                name="currency"
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger id="p-currency">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {Object.entries(CurrencyType).map(([value, text]) => (
                        <SelectItem key={value} value={value}>
                          {t(text)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
            </FormField>

            <FormField id="p-currency_rate" label={t('payment_edit.currencyRate')} required help={t('payment_edit.currencyRateTip')}>
              <Input id="p-currency_rate" type="number" step="any" {...register('currency_rate', { valueAsNumber: true })} />
            </FormField>

            {Object.entries(configFields).map(([key, param]) => (
              <FormField key={key} id={`p-config-${key}`} label={configText(t, param.name)} hint={configText(t, param.description)}>
                <Controller
                  control={control}
                  name={`config.${key}`}
                  render={({ field }) =>
                    param.type === 'select' ? (
                      <Select value={field.value || ''} onValueChange={field.onChange}>
                        <SelectTrigger id={`p-config-${key}`}>
                          <SelectValue placeholder={configText(t, param.name)} />
                        </SelectTrigger>
                        <SelectContent>
                          {param.options.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {configText(t, option.name)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    ) : (
                      <Textarea
                        id={`p-config-${key}`}
                        rows={2}
                        placeholder={configText(t, param.description)}
                        value={field.value || ''}
                        onChange={field.onChange}
                      />
                    )
                  }
                />
              </FormField>
            ))}
          </SheetBody>
          <SheetFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting && <Loader2 className="size-4 animate-spin" />}
              {t('common.submit')}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}

PaymentSheet.propTypes = {
  open: PropTypes.bool,
  paymentId: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  onClose: PropTypes.func,
  onSaved: PropTypes.func
};
