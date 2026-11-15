import aliyunLogo from 'assets/images/mail/aliyun.svg';
import amazonsesLogo from 'assets/images/mail/amazonses.svg';
import brevoLogo from 'assets/images/mail/brevo.svg';
import exmailLogo from 'assets/images/mail/exmail.png';
import gmailLogo from 'assets/images/mail/gmail.svg';
import mailgunLogo from 'assets/images/mail/mailgun.svg';
import netease163Logo from 'assets/images/mail/netease163.png';
import postmarkLogo from 'assets/images/mail/postmark.svg';
import qqLogo from 'assets/images/mail/qq.svg';
import resendLogo from 'assets/images/mail/resend.svg';
import sendgridLogo from 'assets/images/mail/sendgrid.svg';
import zohoLogo from 'assets/images/mail/zoho.svg';

// ==============================|| MAIL SENDING — PROVIDER PRESETS ||============================== //
// Every preset is still plain SMTP underneath: picking one fills SMTPServer / SMTPPort /
// SMTPTLSMode (and SMTPAccount where the provider fixes it), so the admin only types what the
// provider hands out — an API key, or a mailbox address and its app password.
//
// account:
//   { fixed: 'x' }      the SMTP username is always x (the key goes in the password)
//   { field: 'label' }  the provider gives a separate SMTP username
//   { fromAddress }     the mailbox address is the username and the sender
// servers: one host, or several to pick from (regions, account types); `label` is shown as is.
//
// Logos (assets/images/mail): Simple Icons (CC0) in each brand's color; Postmark from
// postmarkapp.com; NetEase 163 Mail and Tencent Exmail from their sites' icons.

export const CUSTOM_PROVIDER = 'custom';

const SES_REGIONS = [
  'us-east-1',
  'us-east-2',
  'us-west-2',
  'ca-central-1',
  'eu-west-1',
  'eu-west-2',
  'eu-central-1',
  'ap-south-1',
  'ap-southeast-1',
  'ap-southeast-2',
  'ap-northeast-1',
  'sa-east-1'
];

export const MAIL_PROVIDERS = [
  {
    id: 'resend',
    name: 'Resend',
    group: 'service',
    logo: resendLogo,
    invertDark: true,
    servers: [{ host: 'smtp.resend.com' }],
    port: 465,
    tls: 'ssl',
    account: { fixed: 'resend' },
    secret: 'apiKey',
    docs: 'https://resend.com/api-keys'
  },
  {
    id: 'sendgrid',
    name: 'SendGrid',
    group: 'service',
    logo: sendgridLogo,
    servers: [{ host: 'smtp.sendgrid.net' }],
    port: 587,
    tls: 'starttls',
    account: { fixed: 'apikey' },
    secret: 'apiKey',
    docs: 'https://app.sendgrid.com/settings/api_keys'
  },
  {
    id: 'ses',
    name: 'Amazon SES',
    group: 'service',
    logo: amazonsesLogo,
    servers: SES_REGIONS.map((r) => ({ host: `email-smtp.${r}.amazonaws.com`, label: r })),
    serverLabel: 'region',
    port: 587,
    tls: 'starttls',
    account: { field: 'smtpUsername' },
    secret: 'smtpPassword',
    docs: 'https://console.aws.amazon.com/ses/home#/smtp'
  },
  {
    id: 'mailgun',
    name: 'Mailgun',
    group: 'service',
    logo: mailgunLogo,
    servers: [
      { host: 'smtp.mailgun.org', label: 'US' },
      { host: 'smtp.eu.mailgun.org', label: 'EU' }
    ],
    serverLabel: 'region',
    port: 587,
    tls: 'starttls',
    account: { field: 'smtpUsername', placeholder: 'postmaster@mg.example.com' },
    secret: 'smtpPassword',
    docs: 'https://app.mailgun.com/mg/sending/domains'
  },
  {
    id: 'postmark',
    name: 'Postmark',
    group: 'service',
    logo: postmarkLogo,
    servers: [{ host: 'smtp.postmarkapp.com' }],
    port: 587,
    tls: 'starttls',
    account: { field: 'accessKey' },
    secret: 'secretKey',
    docs: 'https://account.postmarkapp.com/servers'
  },
  {
    id: 'brevo',
    name: 'Brevo',
    group: 'service',
    logo: brevoLogo,
    servers: [{ host: 'smtp-relay.brevo.com' }],
    port: 587,
    tls: 'starttls',
    account: { field: 'smtpLogin', placeholder: 'xxxxxx@smtp-brevo.com' },
    secret: 'smtpKey',
    docs: 'https://app.brevo.com/settings/keys/smtp'
  },
  {
    id: 'aliyun',
    name: 'Alibaba Cloud Direct Mail',
    group: 'service',
    logo: aliyunLogo,
    servers: [
      { host: 'smtpdm.aliyun.com', label: 'cn-hangzhou' },
      { host: 'smtpdm-ap-southeast-1.aliyuncs.com', label: 'ap-southeast-1' },
      { host: 'smtpdm-us-east-1.aliyuncs.com', label: 'us-east-1' },
      { host: 'smtpdm-eu-central-1.aliyuncs.com', label: 'eu-central-1' }
    ],
    serverLabel: 'region',
    port: 465,
    tls: 'ssl',
    account: { fromAddress: true },
    secret: 'smtpPassword',
    docs: 'https://dm.console.aliyun.com/'
  },
  {
    id: 'gmail',
    name: 'Gmail',
    group: 'mailbox',
    logo: gmailLogo,
    servers: [{ host: 'smtp.gmail.com' }],
    port: 465,
    tls: 'ssl',
    account: { fromAddress: true },
    secret: 'appPassword',
    docs: 'https://myaccount.google.com/apppasswords'
  },
  {
    id: 'zoho',
    name: 'Zoho Mail',
    group: 'mailbox',
    logo: zohoLogo,
    servers: [
      { host: 'smtppro.zoho.com', label: 'smtppro.zoho.com' },
      { host: 'smtp.zoho.com', label: 'smtp.zoho.com' },
      { host: 'smtppro.zoho.eu', label: 'smtppro.zoho.eu' },
      { host: 'smtp.zoho.eu', label: 'smtp.zoho.eu' },
      { host: 'smtppro.zoho.in', label: 'smtppro.zoho.in' },
      { host: 'smtp.zoho.com.cn', label: 'smtp.zoho.com.cn' }
    ],
    serverLabel: 'server',
    port: 465,
    tls: 'ssl',
    account: { fromAddress: true },
    secret: 'appPassword',
    docs: 'https://accounts.zoho.com/home#security/app_password'
  },
  {
    id: 'qq',
    name: 'QQ Mail',
    group: 'mailbox',
    logo: qqLogo,
    servers: [{ host: 'smtp.qq.com' }],
    port: 465,
    tls: 'ssl',
    account: { fromAddress: true },
    secret: 'authCode',
    docs: 'https://wx.mail.qq.com/'
  },
  {
    id: 'exmail',
    name: 'Tencent Exmail',
    group: 'mailbox',
    logo: exmailLogo,
    servers: [{ host: 'smtp.exmail.qq.com' }],
    port: 465,
    tls: 'ssl',
    account: { fromAddress: true },
    secret: 'clientPassword',
    docs: 'https://exmail.qq.com/'
  },
  {
    id: 'netease163',
    name: 'NetEase 163 Mail',
    group: 'mailbox',
    logo: netease163Logo,
    servers: [{ host: 'smtp.163.com' }],
    port: 465,
    tls: 'ssl',
    account: { fromAddress: true },
    secret: 'authCode',
    docs: 'https://mail.163.com/'
  }
];

const byId = Object.fromEntries(MAIL_PROVIDERS.map((p) => [p.id, p]));

export const getMailProvider = (id) => byId[id] || null;

const sameHost = (a, b) =>
  String(a || '')
    .trim()
    .toLowerCase() ===
  String(b || '')
    .trim()
    .toLowerCase();

// The provider the stored settings belong to: the saved choice, else the preset whose server they
// point at (settings made before presets existed), else custom when a server is set, else none.
export function detectMailProvider(inputs) {
  const saved = inputs.SMTPProvider;
  if (saved === CUSTOM_PROVIDER || byId[saved]) return saved;
  const host = inputs.SMTPServer;
  if (!host) return '';
  const match = MAIL_PROVIDERS.find((p) => p.servers.some((s) => sameHost(s.host, host)));
  return match ? match.id : CUSTOM_PROVIDER;
}

// Option values to write when the admin picks a provider. The sender address and name are kept;
// a username a previous preset filled in is cleared, and so is a key typed but not saved, so
// neither is sent to the new provider. (An empty key keeps the saved one, as everywhere.)
export function providerPatch(id, inputs) {
  if (id === CUSTOM_PROVIDER) return { SMTPProvider: CUSTOM_PROVIDER, SMTPToken: '' };
  const p = byId[id];
  if (!p) return {};
  const keepHost = p.servers.find((s) => sameHost(s.host, inputs.SMTPServer));
  const prev = byId[detectMailProvider(inputs)];
  let account = inputs.SMTPAccount ?? '';
  if (p.account.fixed) account = p.account.fixed;
  else if (p.account.fromAddress) account = inputs.SMTPFrom ?? '';
  else if (prev && prev.id !== p.id && (prev.account.fixed || prev.account.fromAddress)) account = '';
  return {
    SMTPProvider: id,
    SMTPServer: keepHost ? keepHost.host : p.servers[0].host,
    SMTPPort: String(p.port),
    SMTPTLSMode: p.tls,
    SMTPAccount: account,
    SMTPToken: ''
  };
}

// The sender address; for mailbox-style providers it is also the SMTP username.
export function fromAddressPatch(id, value) {
  const p = byId[id];
  return p && p.account.fromAddress ? { SMTPFrom: value, SMTPAccount: value } : { SMTPFrom: value };
}

// Body for POST /api/option/smtp/test, built from the form as it stands (saved or not). An
// empty token tells the server to reuse the saved one.
export function smtpTestPayload(inputs, to) {
  return {
    to,
    server: inputs.SMTPServer ?? '',
    port: Number(inputs.SMTPPort) || 0,
    account: inputs.SMTPAccount ?? '',
    from: inputs.SMTPFrom ?? '',
    from_name: inputs.SMTPFromName ?? '',
    token: inputs.SMTPToken ?? '',
    tls_mode: inputs.SMTPTLSMode || 'auto'
  };
}

// Whether the stored password still belongs to the form: same server and username as saved.
export function savedSecretApplies(inputs, origin, hasSecret) {
  return hasSecret && sameHost(inputs.SMTPServer, origin.SMTPServer) && (inputs.SMTPAccount ?? '') === (origin.SMTPAccount ?? '');
}

// What still has to be filled in before saving or testing, as field ids ([] when complete).
// `secretSaved` is true when a password is already stored.
export function missingMailFields(id, inputs, secretSaved) {
  if (!id) return [];
  const missing = [];
  const p = byId[id];
  if (!p) {
    if (!inputs.SMTPServer) missing.push('SMTPServer');
    if (!Number(inputs.SMTPPort)) missing.push('SMTPPort');
  } else if (p.account.field && !inputs.SMTPAccount) {
    missing.push('SMTPAccount');
  }
  if (!inputs.SMTPFrom) missing.push('SMTPFrom');
  if (!inputs.SMTPToken && !secretSaved) missing.push('SMTPToken');
  return missing;
}
