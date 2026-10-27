import { describe, it, expect } from 'vitest';
import { PaymentConfig, splitSecretConfig } from './paymentConfig';

describe('splitSecretConfig', () => {
  it('blanks secret fields and keeps their masks', () => {
    const { values, masked } = splitSecretConfig('stripe', {
      secret_key: 'sk_l****mnop',
      webhook_secret: 'whse****wxyz'
    });
    expect(values).toEqual({ secret_key: '', webhook_secret: '' });
    expect(masked).toEqual({ secret_key: 'sk_l****mnop', webhook_secret: 'whse****wxyz' });
  });

  it('leaves non-secret fields untouched and skips unset secrets', () => {
    const { values, masked } = splitSecretConfig('wxpay', { app_id: 'wx1', mch_id: 'm1', mch_apiv3_key: '' });
    expect(values).toEqual({ app_id: 'wx1', mch_id: 'm1', mch_apiv3_key: '', mch_private_key: '' });
    expect(masked).toEqual({});
  });

  it('returns the config as-is for unknown types', () => {
    const { values, masked } = splitSecretConfig('unknown', { foo: 'bar' });
    expect(values).toEqual({ foo: 'bar' });
    expect(masked).toEqual({});
  });

  it('marks every gateway credential as secret', () => {
    const secrets = Object.fromEntries(
      Object.entries(PaymentConfig).map(([type, fields]) => [
        type,
        Object.keys(fields)
          .filter((k) => fields[k].secret)
          .sort()
      ])
    );
    expect(secrets).toEqual({
      epay: ['key'],
      alipay: ['private_key'],
      wxpay: ['mch_apiv3_key', 'mch_private_key'],
      stripe: ['secret_key', 'webhook_secret']
    });
  });
});
