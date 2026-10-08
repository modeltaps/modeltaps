// 把 User-Agent 压成「系统 · 浏览器」一行,给账号安全页的登录会话列表用。只求可读,不求精确版本。

export function describeUserAgent(ua = '') {
  const text = String(ua || '');
  let os = '';
  if (/iPhone|iPad|iPod/.test(text)) os = 'iOS';
  else if (/Android/.test(text)) os = 'Android';
  else if (/Windows/.test(text)) os = 'Windows';
  else if (/Mac OS X|Macintosh/.test(text)) os = 'macOS';
  else if (/CrOS/.test(text)) os = 'ChromeOS';
  else if (/Linux/.test(text)) os = 'Linux';

  let browser = '';
  if (/Edg\//.test(text)) browser = 'Edge';
  else if (/OPR\/|Opera/.test(text)) browser = 'Opera';
  else if (/Firefox\//.test(text)) browser = 'Firefox';
  else if (/Chrome\/|CriOS\//.test(text)) browser = 'Chrome';
  else if (/Safari\//.test(text)) browser = 'Safari';

  return [os, browser].filter(Boolean).join(' · ');
}
