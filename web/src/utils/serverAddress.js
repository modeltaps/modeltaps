// 前端展示型 URL(测试 API Key 命令、聊天链接、快速开始)不盲信后端 server_address 的
// 出厂默认值 http://localhost:3000:未真正配置时回退到当前页面地址。
// isPlaceholderServerAddress 谓词供系统设置页警告(UI-3)复用。

// 从任意形态的地址(含/不含协议、端口、路径)提取主机名,失败返回空串。
function hostnameOf(value) {
  try {
    const withScheme = /^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(value) ? value : `http://${value}`;
    return new URL(withScheme).hostname.toLowerCase();
  } catch {
    return '';
  }
}

function isLoopbackHost(hostname) {
  return hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '::1' || hostname === '[::1]';
}

// server_address 是否"未真正配置":为空/仅空白,或指向 localhost/127.0.0.1
// (含任意端口、http/https)而当前页面本身不在 localhost —— 视为出厂默认残留。
// 本地开发(页面本身在 localhost)时返回 false,行为保持不变。
export function isPlaceholderServerAddress(serverAddress) {
  if (!serverAddress || !serverAddress.trim()) return true;
  if (!isLoopbackHost(hostnameOf(serverAddress))) return false;
  return !isLoopbackHost(window.location.hostname);
}

// 解析展示用服务器地址:未真正配置时回退到 fallback(默认 window.location.origin,
// 需要 host 形态的消费点传入 window.location.host);否则用设置值并去掉尾部斜杠。
export function resolveServerAddress(serverAddress, fallback = window.location.origin) {
  if (isPlaceholderServerAddress(serverAddress)) {
    return fallback;
  }
  return serverAddress.trim().replace(/\/+$/, '');
}
