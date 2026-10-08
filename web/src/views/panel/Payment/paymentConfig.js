// Payment gateway type metadata. Ported verbatim from the v1
// `views/Payment/type/Config.js` so the backend config contract stays identical.
// Display text is stored as i18n keys (or [key, params]); render it through configText().

const WXPAY_DOC = 'https://pay.weixin.qq.com/wiki/doc/apiv3/open/pay/chapter2_7_1.shtml';

export const PaymentType = {
  epay: 'paymentConfig.types.epay',
  alipay: 'paymentConfig.types.alipay',
  wxpay: 'paymentConfig.types.wxpay',
  stripe: 'paymentConfig.types.stripe'
};

export const CurrencyType = {
  CNY: 'paymentConfig.currency.CNY',
  USD: 'paymentConfig.currency.USD'
};

export const PaymentConfig = {
  epay: {
    pay_domain: { name: 'paymentConfig.fields.payDomain', description: 'paymentConfig.fields.payDomain', type: 'text', value: '' },
    partner_id: { name: 'paymentConfig.fields.merchantId', description: 'paymentConfig.fields.merchantId', type: 'text', value: '' },
    key: { name: 'paymentConfig.fields.key', description: 'paymentConfig.fields.key', type: 'text', value: '' },
    pay_type: {
      name: 'paymentConfig.fields.payType',
      description: 'paymentConfig.epay.payTypeDesc',
      type: 'select',
      value: '',
      options: [
        { name: 'paymentConfig.epay.checkout', value: 'epay' },
        { name: 'paymentConfig.types.alipay', value: 'alipay' },
        { name: 'paymentConfig.epay.wechat', value: 'wxpay' },
        { name: 'paymentConfig.epay.qq', value: 'qqpay' },
        { name: 'paymentConfig.epay.jd', value: 'jdpay' },
        { name: 'paymentConfig.epay.unionpay', value: 'bank' },
        { name: 'paymentConfig.epay.paypal', value: 'paypal' },
        { name: 'paymentConfig.epay.usdt', value: 'usdt' }
      ]
    }
  },
  alipay: {
    app_id: { name: 'paymentConfig.alipay.appId', description: 'paymentConfig.alipay.appIdDesc', type: 'text', value: '' },
    private_key: {
      name: 'paymentConfig.alipay.privateKey',
      description: ['paymentConfig.alipay.privateKeyDesc', { url: 'https://opendocs.alipay.com/common/02kipl?pathHash=84adb0fd' }],
      type: 'text',
      value: ''
    },
    public_key: {
      name: 'paymentConfig.alipay.publicKey',
      description: ['paymentConfig.alipay.publicKeyDesc', { url: 'https://opendocs.alipay.com/common/02kdnc?pathHash=fb0c752a' }],
      type: 'text',
      value: ''
    },
    pay_type: {
      name: 'paymentConfig.fields.payType',
      description: 'paymentConfig.alipay.payTypeDesc',
      type: 'select',
      value: '',
      options: [
        { name: 'paymentConfig.alipay.facepay', value: 'facepay' },
        { name: 'paymentConfig.alipay.pagepay', value: 'pagepay' },
        { name: 'paymentConfig.alipay.wappay', value: 'wappay' }
      ]
    }
  },
  wxpay: {
    app_id: {
      name: 'paymentConfig.wxpay.appId',
      description: ['paymentConfig.wxpay.appIdDesc', { url: WXPAY_DOC }],
      type: 'text',
      value: ''
    },
    mch_id: {
      name: 'paymentConfig.fields.merchantId',
      description: ['paymentConfig.wxpay.mchIdDesc', { url: WXPAY_DOC }],
      type: 'text',
      value: ''
    },
    mch_certificate_serial_number: {
      name: 'paymentConfig.wxpay.certSerial',
      description: ['paymentConfig.wxpay.certSerialDesc', { url: WXPAY_DOC }],
      type: 'text',
      value: ''
    },
    mch_apiv3_key: {
      name: 'paymentConfig.wxpay.apiv3Key',
      description: ['paymentConfig.wxpay.apiv3KeyDesc', { url: WXPAY_DOC }],
      type: 'text',
      value: ''
    },
    mch_private_key: {
      name: 'paymentConfig.wxpay.privateKey',
      description: ['paymentConfig.wxpay.privateKeyDesc', { url: WXPAY_DOC }],
      type: 'text',
      value: ''
    },
    pay_type: {
      name: 'paymentConfig.fields.payType',
      description: 'paymentConfig.fields.payType',
      type: 'select',
      value: '',
      options: [{ name: 'paymentConfig.wxpay.native', value: 'Native' }]
    }
  },
  stripe: {
    secret_key: { name: 'paymentConfig.stripe.secretKey', description: 'paymentConfig.stripe.secretKeyDesc', type: 'text', value: '' },
    webhook_secret: {
      name: 'paymentConfig.stripe.webhookSecret',
      description: 'paymentConfig.stripe.webhookSecretDesc',
      type: 'text',
      value: ''
    }
  }
};
