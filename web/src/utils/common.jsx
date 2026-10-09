import Decimal from 'decimal.js';
import PropTypes from 'prop-types';
import { toast } from '@/components/ui/sonner';
import { API } from './api';
import { useSelector } from 'react-redux';
import i18n from 'i18n/i18n';
import { uiLocalesQuery } from 'i18n/uiLocale';
import { brandName } from './brand';
import { sanitizeHtml } from './sanitize';

export function getSystemName() {
  return brandName(localStorage.getItem('system_name'));
}

export function isMobile() {
  return window.innerWidth <= 600;
}

// eslint-disable-next-line
export function SnackbarHTMLContent({ htmlContent }) {
  return <div dangerouslySetInnerHTML={{ __html: sanitizeHtml(htmlContent) }} />;
}

export function showError(error) {
  if (error.message) {
    if (error.name === 'AxiosError') {
      switch (error.response.status) {
        case 429:
          toast.error(i18n.t('common.errorPrefix') + (error.response.data?.error?.message || i18n.t('common.errTooManyRequests')));
          break;
        case 500:
          toast.error(i18n.t('common.errorPrefix') + i18n.t('common.errServerInternal'));
          break;
        case 405:
          toast.info(i18n.t('common.errDemoOnly'));
          break;
        default:
          toast.error(i18n.t('common.errorPrefix') + error.message);
      }
    }
  } else {
    toast.error(i18n.t('common.errorPrefix') + error);
  }
}

export function showNotice(message, isHTML = false) {
  if (isHTML) {
    toast(<SnackbarHTMLContent htmlContent={message} />);
  } else {
    toast.info(message);
  }
}

export function showWarning(message) {
  toast.warning(message);
}

export function showSuccess(message) {
  toast.success(message);
}

export function showInfo(message) {
  toast.info(message);
}

export function copy(text, name = '') {
  try {
    navigator.clipboard.writeText(text);
  } catch (error) {
    const failed = name ? i18n.t('common.copyNamedFailed', { name }) : i18n.t('common.copyFailedManual');
    text = `${failed}<br /><br />${text}`;
    toast(<SnackbarHTMLContent htmlContent={text} />);
    return;
  }
  showSuccess(name ? i18n.t('common.copyNamedSuccess', { name }) : i18n.t('common.copySucceeded'));
}

export async function getOAuthState() {
  try {
    const res = await API.get('/api/oauth/state');
    const { success, message, data } = res.data;
    if (success) {
      return data;
    } else {
      showError(message);
      return '';
    }
  } catch (error) {
    return '';
  }
}

export async function onGitHubOAuthClicked(github_client_id, openInNewTab = false) {
  const state = await getOAuthState();
  if (!state) return;
  let url = `https://github.com/login/oauth/authorize?client_id=${github_client_id}&state=${state}&scope=user:email`;
  if (openInNewTab) {
    window.open(url);
  } else {
    window.location.href = url;
  }
}

// slug 为空时走无 slug 的旧路由，后端等价于 slug=oidc（存量部署已在 IdP 侧登记该回调）。
// 带上当前界面语言（ui_locales），让 IdP 登录页与本站语言一致。
export async function getOIDCEndpoint(slug = '') {
  try {
    const path = slug ? `/api/oauth/endpoint/${encodeURIComponent(slug)}` : '/api/oauth/endpoint';
    const res = await API.get(path + uiLocalesQuery());
    const { success, message, data } = res.data;
    if (success) {
      return data;
    } else {
      showError(message);
      return '';
    }
  } catch (error) {
    return '';
  }
}

export async function onOIDCAuthClicked(slug = '', openInNewTab = false) {
  const url = await getOIDCEndpoint(slug);
  if (!url) return;
  if (openInNewTab) {
    window.open(url);
  } else {
    window.location.href = url;
  }
}
// WebAuthn 服务端稳定错误码 → i18n key 映射(与 controller/webauthn.go 保持一致)
const WEBAUTHN_SERVER_ERROR_CODES = [
  'config_failed',
  'user_not_found',
  'bad_request',
  'login_unavailable',
  'begin_login_failed',
  'begin_register_failed',
  'session_expired',
  'session_error',
  'verify_failed',
  'save_failed',
  'webauthn_disabled'
];

// 翻译 WebAuthn 服务端错误:优先 code 映射,其次 message,最后回退通用 key;detail 仅输出到 console
function translateWebAuthnServerError(data, fallbackKey) {
  if (data?.detail) {
    console.warn('WebAuthn server error detail:', data.detail);
  }
  if (data?.code && WEBAUTHN_SERVER_ERROR_CODES.includes(data.code)) {
    return i18n.t(`webauthn.server.${data.code}`);
  }
  return data?.message || i18n.t(fallbackKey);
}

export async function onWebAuthnClicked(username, showError, showSuccess, navigateToStatus) {
  // Remove username check to support discoverable login
  // if (!username || username.trim() === '') {
  //   showError('请先输入用户名');
  //   return;
  // }

  try {
    // 检查浏览器是否支持WebAuthn
    if (!window.PublicKeyCredential) {
      showError(i18n.t('webauthn.browserNotSupported'));
      return;
    }

    // Helper functions
    const base64urlToUint8Array = (base64url) => {
      try {
        if (!base64url || typeof base64url !== 'string') {
          throw new Error('Invalid base64url input');
        }

        // 移除所有空白字符
        base64url = base64url.trim();

        // 将 base64url 转换为 base64
        let base64 = base64url.replace(/-/g, '+').replace(/_/g, '/');

        // 移除现有的填充字符，然后重新添加正确的填充
        base64 = base64.replace(/=/g, '');

        // 添加正确的填充
        while (base64.length % 4) {
          base64 += '=';
        }

        const binary = atob(base64);
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) {
          bytes[i] = binary.charCodeAt(i);
        }
        return bytes;
      } catch (error) {
        throw new Error('Failed to decode base64url data: ' + error.message, { cause: error });
      }
    };

    const uint8ArrayToBase64url = (buffer) => {
      try {
        let binary = '';
        for (let i = 0; i < buffer.byteLength; i++) {
          binary += String.fromCharCode(buffer[i]);
        }

        let base64 = btoa(binary);
        return base64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
      } catch (error) {
        throw new Error('Failed to encode to base64url', { cause: error });
      }
    };

    // 开始登录流程
    const beginResponse = await fetch('/api/webauthn/login/begin', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ username: username ? username.trim() : '' })
    });

    const beginData = await beginResponse.json();

    if (!beginData.success) {
      showError(translateWebAuthnServerError(beginData, 'webauthn.loginBeginFailed'));
      return;
    }

    // 将服务器返回的选项转换为适合navigator.credentials.get的格式
    const publicKeyCredentialRequestOptions = {
      ...beginData.data.publicKey,
      challenge: base64urlToUint8Array(beginData.data.publicKey.challenge),
      allowCredentials:
        beginData.data.publicKey.allowCredentials?.map((cred) => ({
          ...cred,
          id: base64urlToUint8Array(cred.id)
        })) || []
    };

    // 调用WebAuthn API进行认证
    const credential = await navigator.credentials.get({
      publicKey: publicKeyCredentialRequestOptions
    });

    if (!credential) {
      showError(i18n.t('webauthn.authCancelled'));
      return;
    }

    // 准备发送给后端的数据
    const credentialData = {
      id: credential.id,
      rawId: uint8ArrayToBase64url(new Uint8Array(credential.rawId)),
      type: credential.type,
      response: {
        authenticatorData: uint8ArrayToBase64url(new Uint8Array(credential.response.authenticatorData)),
        clientDataJSON: uint8ArrayToBase64url(new Uint8Array(credential.response.clientDataJSON)),
        signature: uint8ArrayToBase64url(new Uint8Array(credential.response.signature)),
        userHandle: credential.response.userHandle ? uint8ArrayToBase64url(new Uint8Array(credential.response.userHandle)) : null
      }
    };

    // 完成登录流程
    const finishResponse = await fetch('/api/webauthn/login/finish', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(credentialData)
    });

    const finishData = await finishResponse.json();
    if (!finishData.success) {
      showError(translateWebAuthnServerError(finishData, 'webauthn.loginVerifyFailed'));
      return;
    }

    // 登录成功
    showSuccess(i18n.t('webauthn.loginSuccess'));
    if (navigateToStatus) {
      navigateToStatus();
    }
    window.location.reload();
  } catch (error) {
    if (error.name === 'NotAllowedError') {
      showError(i18n.t('webauthn.authRejectedOrTimeout'));
    } else if (error.name === 'NotSupportedError') {
      showError(i18n.t('webauthn.deviceNotSupported'));
    } else if (error.name === 'InvalidStateError') {
      showError(i18n.t('webauthn.invalidState'));
    } else if (error.name === 'SecurityError') {
      showError(i18n.t('webauthn.securityError'));
    } else {
      showError(i18n.t('webauthn.loginFailed', { message: error.message }));
    }
  }
}

export async function onWebAuthnRegister(showError, showSuccess, onSuccess, alias = '') {
  try {
    // 检查浏览器是否支持WebAuthn
    if (!window.PublicKeyCredential) {
      showError(i18n.t('webauthn.browserNotSupported'));
      return;
    }

    // 开始注册流程
    const beginResponse = await fetch('/api/webauthn/registration/begin', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: localStorage.getItem('token')
      },
      body: JSON.stringify({ alias })
    });

    const beginData = await beginResponse.json();
    if (!beginData.success) {
      showError(translateWebAuthnServerError(beginData, 'webauthn.registerBeginFailed'));
      return;
    }

    // Helper function to decode base64url to Uint8Array
    const base64urlToUint8Array = (base64url) => {
      try {
        // Remove any whitespace and ensure it's a string
        if (!base64url || typeof base64url !== 'string') {
          throw new Error('Invalid base64url input');
        }

        // Convert base64url to base64
        let base64 = base64url.replace(/-/g, '+').replace(/_/g, '/');

        // Add padding if necessary
        while (base64.length % 4) {
          base64 += '=';
        }

        // Decode base64 to binary string
        const binary = atob(base64);

        // Convert binary string to Uint8Array
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) {
          bytes[i] = binary.charCodeAt(i);
        }
        return bytes;
      } catch (error) {
        throw new Error('Failed to decode base64url data', { cause: error });
      }
    };

    // Helper function to encode Uint8Array to base64url
    const uint8ArrayToBase64url = (buffer) => {
      try {
        // Convert Uint8Array to binary string
        let binary = '';
        for (let i = 0; i < buffer.byteLength; i++) {
          binary += String.fromCharCode(buffer[i]);
        }

        // Encode to base64
        let base64 = btoa(binary);

        // Convert to base64url
        return base64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
      } catch (error) {
        throw new Error('Failed to encode to base64url', { cause: error });
      }
    };

    // 将服务器返回的选项转换为适合navigator.credentials.create的格式
    const publicKeyCredentialCreationOptions = {
      ...beginData.data.publicKey,
      challenge: base64urlToUint8Array(beginData.data.publicKey.challenge),
      user: {
        ...beginData.data.publicKey.user,
        id: base64urlToUint8Array(beginData.data.publicKey.user.id)
      },
      excludeCredentials:
        beginData.data.publicKey.excludeCredentials?.map((cred) => ({
          ...cred,
          id: base64urlToUint8Array(cred.id)
        })) || []
    };

    // 调用WebAuthn API创建凭据
    const credential = await navigator.credentials.create({
      publicKey: publicKeyCredentialCreationOptions
    });

    if (!credential) {
      showError(i18n.t('webauthn.registerCancelled'));
      return;
    }

    // 准备发送给后端的数据 - 使用base64url编码
    const credentialData = {
      id: credential.id,
      rawId: uint8ArrayToBase64url(new Uint8Array(credential.rawId)),
      type: credential.type,
      response: {
        attestationObject: uint8ArrayToBase64url(new Uint8Array(credential.response.attestationObject)),
        clientDataJSON: uint8ArrayToBase64url(new Uint8Array(credential.response.clientDataJSON))
      }
    };

    // 完成注册流程
    const finishResponse = await fetch('/api/webauthn/registration/finish', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: localStorage.getItem('token')
      },
      body: JSON.stringify(credentialData)
    });

    const finishData = await finishResponse.json();

    if (!finishData.success) {
      showError(translateWebAuthnServerError(finishData, 'webauthn.registerVerifyFailed'));
      return;
    }

    // 注册成功
    showSuccess(i18n.t('webauthn.registerSuccess'));
    if (onSuccess) {
      onSuccess();
    }
  } catch (error) {
    if (error.name === 'NotAllowedError') {
      showError(i18n.t('webauthn.registerRejectedOrTimeout'));
    } else if (error.name === 'NotSupportedError') {
      showError(i18n.t('webauthn.deviceNotSupported'));
    } else if (error.message.includes('base64url')) {
      showError(i18n.t('webauthn.encodingError'));
    } else {
      showError(i18n.t('webauthn.registerFailed', { message: error.message }));
    }
  }
}

export async function getWebAuthnCredentials() {
  try {
    const response = await fetch('/api/webauthn/credentials', {
      method: 'GET',
      headers: {
        Authorization: localStorage.getItem('token')
      }
    });

    const data = await response.json();
    if (data.success) {
      return data.data || [];
    } else {
      return [];
    }
  } catch (error) {
    return [];
  }
}

export async function deleteWebAuthnCredential(credentialId, showError, showSuccess, onSuccess) {
  try {
    const response = await fetch(`/api/webauthn/credentials/${credentialId}`, {
      method: 'DELETE',
      headers: {
        Authorization: localStorage.getItem('token')
      }
    });

    const data = await response.json();
    if (data.success) {
      showSuccess(i18n.t('webauthn.deleteSuccess'));
      if (onSuccess) {
        onSuccess();
      }
    } else {
      showError(data.message || i18n.t('webauthn.deleteFailed'));
    }
  } catch (error) {
    showError(i18n.t('webauthn.deleteFailedDetail', { message: error.message }));
  }
}

export async function onLarkOAuthClicked(lark_client_id) {
  const state = await getOAuthState();
  if (!state) return;
  let redirect_uri = `${window.location.origin}/oauth/lark`;
  window.open(`https://open.feishu.cn/open-apis/authen/v1/authorize?redirect_uri=${redirect_uri}&app_id=${lark_client_id}&state=${state}`);
}

export async function onLinuxDoOAuthClicked(client_id, openInNewTab = false) {
  const state = await getOAuthState();
  if (!state) return;
  let url = `https://connect.linux.do/oauth2/authorize?response_type=code&client_id=${client_id}&state=${state}`;
  if (openInNewTab) {
    window.open(url);
  } else {
    window.location.href = url;
  }
}

export function useIsAdmin() {
  const { user } = useSelector((state) => state.account);
  if (!user) return false;
  return user.role >= 10;
}

export function useIsRoot() {
  const { user } = useSelector((state) => state.account);
  if (!user) return false;
  return user.role >= 100;
}

export function useIsReliable() {
  const { user } = useSelector((state) => state.account);
  if (!user) return false;
  return user.role >= 3;
}

export function timestamp2string(timestamp) {
  let date = new Date(timestamp * 1000);
  let year = date.getFullYear().toString();
  let month = (date.getMonth() + 1).toString();
  let day = date.getDate().toString();
  let hour = date.getHours().toString();
  let minute = date.getMinutes().toString();
  let second = date.getSeconds().toString();
  if (month.length === 1) {
    month = '0' + month;
  }
  if (day.length === 1) {
    day = '0' + day;
  }
  if (hour.length === 1) {
    hour = '0' + hour;
  }
  if (minute.length === 1) {
    minute = '0' + minute;
  }
  if (second.length === 1) {
    second = '0' + second;
  }
  return year + '-' + month + '-' + day + ' ' + hour + ':' + minute + ':' + second;
}

// 紧凑时间(MM-DD HH:mm),用于日志表列显示;完整时间(YYYY-MM-DD HH:mm:ss)仍由
// timestamp2string 提供(悬浮 Tooltip 展示)。仅新增,不改动 timestamp2string 现有行为。
export function timestamp2stringCompact(timestamp) {
  return timestamp2string(timestamp).slice(5, 16);
}

export function calculateQuota(quota, digits = 2) {
  let quotaPerUnit = localStorage.getItem('quota_per_unit');
  quotaPerUnit = parseFloat(quotaPerUnit);

  return (quota / quotaPerUnit).toFixed(digits);
}

export function renderQuota(quota, digits = 2) {
  let displayInCurrency = localStorage.getItem('display_in_currency');
  displayInCurrency = displayInCurrency === 'true';
  if (displayInCurrency) {
    if (quota < 0) {
      return '-$' + calculateQuota(Math.abs(quota), digits);
    }
    return '$' + calculateQuota(quota, digits);
  }
  return renderNumber(quota);
}

// ==== 金额显示精度分层(MNY-1 / LOG-2)====
// 消费类金额统一走 OpenRouter 式规则,货币符号可参数化(默认 $,同一套阈值适用于 ¥):
//   0             → $0.00
//   0<abs<1       → 最多 6 位小数并去除尾随零(如 $0.00154、$0.000012)
//   abs≥1         → 2 位小数
// 余额/限额类固定 2 位仍走 renderBalance/renderQuota;点数模式行为不变;
// 单条日志详情(LogDetailDialog)仍用 renderQuota(x, 6) 保留精确值。

// 已换算成货币的数值 → 显示字符串(含货币符号)
export function formatSpendAmount(value, symbol = '$') {
  const num = Number(value) || 0;
  const abs = Math.abs(num);
  if (abs === 0) return symbol + '0.00';
  const sign = num < 0 ? '-' : '';
  if (abs >= 1) return sign + symbol + abs.toFixed(2);
  const trimmed = abs.toFixed(6).replace(/0+$/, '').replace(/\.$/, '');
  return sign + symbol + trimmed;
}

// 余额类(原始 quota):货币模式固定 2 位;点数模式与 renderQuota 一致走 renderNumber
export function renderBalance(quota) {
  return renderQuota(quota, 2);
}

// 消费汇总类(原始 quota):货币模式走截断规则;点数模式行为与 renderQuota 一致
export function renderSpend(quota, symbol = '$') {
  let displayInCurrency = localStorage.getItem('display_in_currency');
  if (displayInCurrency !== 'true') {
    return renderNumber(quota);
  }
  return formatSpendAmount(Number(calculateQuota(quota, 6)), symbol);
}

// 表格消费金额单元格:货币模式下按 formatSpendAmount 显示完整精度(最多 6 位小数)。
// quota 传原始点数(尊重点数模式);value 传已换算货币值(始终按货币显示)。
export function SpendAmount({ quota, value, symbol = '$', className }) {
  const hasQuota = quota !== undefined && quota !== null;
  if (hasQuota && localStorage.getItem('display_in_currency') !== 'true') {
    return <span className={className}>{renderNumber(quota)}</span>;
  }
  const amount = hasQuota ? Number(calculateQuota(quota, 6)) : Number(value) || 0;
  return <span className={className}>{formatSpendAmount(amount, symbol)}</span>;
}
SpendAmount.propTypes = {
  quota: PropTypes.number,
  value: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  symbol: PropTypes.string,
  className: PropTypes.string
};

export const verifyJSON = (str) => {
  try {
    JSON.parse(str);
  } catch (e) {
    return false;
  }
  return true;
};

export function renderNumber(num) {
  if (num >= 1000000000) {
    return (num / 1000000000).toFixed(1) + 'B';
  } else if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + 'M';
  } else if (num >= 10000) {
    return (num / 1000).toFixed(1) + 'k';
  } else {
    return num;
  }
}

// 数字千位分隔符
export function thousandsSeparator(num) {
  return num.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ',');
}

export function downloadTextAsFile(text, filename) {
  let blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
  let url = URL.createObjectURL(blob);
  let a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
}

export function printElementAsPDF(elementId, filename) {
  const element = document.getElementById(elementId);
  if (!element) {
    showError('Element not found');
    return;
  }

  // Create a new window
  const printWindow = window.open('', '_blank');

  // Get the styles from the current document
  const styles = Array.from(document.styleSheets)
    .map((styleSheet) => {
      try {
        return Array.from(styleSheet.cssRules)
          .map((rule) => rule.cssText)
          .join('\n');
      } catch (e) {
        // Ignore cross-origin stylesheets
        return '';
      }
    })
    .filter(Boolean)
    .join('\n');

  // Write the HTML content to the new window
  printWindow.document.write(`
    <!DOCTYPE html>
    <html>
      <head>
        <title>${filename}</title>
        <style>${styles}</style>
      </head>
      <body>
        ${element.outerHTML}
      </body>
    </html>
  `);

  printWindow.document.close();

  // Wait for the content to load before printing
  printWindow.onload = function () {
    printWindow.print();
    // Close the window after printing (optional)
    // printWindow.close();
  };
}

export function removeTrailingSlash(url) {
  if (url.endsWith('/')) {
    return url.slice(0, -1);
  } else {
    return url;
  }
}

export function trims(values) {
  // typeof null === 'object'，若不先拦截会走进下面的对象分支被递归成 {}，
  // 导致后端 *int/*bool 等指针字段反序列化失败(上游 de1ca26 同修)。
  if (values === null || values === undefined) {
    return values;
  }

  if (typeof values === 'string') {
    return values.trim();
  }

  if (Array.isArray(values)) {
    return values.map((value) => trims(value));
  }

  if (typeof values === 'object') {
    let newValues = {};
    for (let key in values) {
      newValues[key] = trims(values[key]);
    }
    return newValues;
  }

  return values;
}

export function ValueFormatter(value, onlyUsd = false, unitMillion = false) {
  if (value == null) {
    return '';
  }
  if (value === 0) {
    return 'Free';
  }

  let decimalValue = new Decimal(value.toString());
  if (unitMillion) {
    decimalValue = decimalValue.mul(1000);
  }

  let usd = decimalValue.mul(0.002).toPrecision(6);

  if (onlyUsd) {
    usd = usd.replace(/(\.\d*?[1-9])0+$|\.0*$/, '$1');

    return `$${usd}`;
  }

  let rmb = decimalValue.mul(0.014).toPrecision(6);

  usd = usd.replace(/(\.\d*?[1-9])0+$|\.0*$/, '$1');
  rmb = rmb.replace(/(\.\d*?[1-9])0+$|\.0*$/, '$1');

  return `$${usd} / ￥${rmb}`;
}
