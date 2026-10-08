// 安全边界用例的共用工具：环境读取、SQLite 只读查询、Modeltaps 后台接口、
// AuthGear 配置热改、AuthGear 登录流页面驱动、id_token 抓取。
// 只被 cases/security.mjs 使用；不修改 Modeltaps 源码。
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:http';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const AG_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export const OUT_DIR = path.join(AG_DIR, 'out');
export const YAML_PATH = path.join(AG_DIR, 'var/app/authgear.yaml');

export const env = Object.fromEntries(
  readFileSync(path.join(OUT_DIR, 'modeltaps.env'), 'utf8')
    .split('\n').filter(Boolean).map((l) => l.split(/=(.*)/s).slice(0, 2)));

export const MODELTAPS = env.MODELTAPS_BASE;
export const AUTHGEAR = env.AUTHGEAR_ORIGIN;
export const DB = path.join(env.MODELTAPS_DATA_DIR, 'modeltaps.db');
const CLIENT_ID = 'modeltaps';
const CLIENT_SECRET = 'modeltaps-authgear-it-client-secret';
export const SINK_PORT = Number(process.env.SEC_SINK_PORT || 3299);
export const SINK_REDIRECT = 'http://127.0.0.1:' + SINK_PORT + '/cb';

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// ---------- 只读数据查询 ----------
export function sql(query) {
  const out = execFileSync('sqlite3', ['-json', DB, query], { encoding: 'utf8' }).trim();
  return out ? JSON.parse(out) : [];
}

// Modeltaps 的删除是软删（users.deleted_at 置位、username 改写），这里一律只看存活行。
const USER_COLS = 'id,username,email,display_name,length(password) as pw_len,status';
export const userByName = (u) => sql(
  'select ' + USER_COLS + " from users where deleted_at is null and username='" + u + "'")[0] || null;
export const userById = (id) => sql(
  'select ' + USER_COLS + ' from users where deleted_at is null and id=' + id)[0] || null;
export const identitiesOf = (id) => sql(
  'select id,user_id,provider_id,subject from user_oidc_identities where user_id=' + id);
export const allIdentities = () => sql(
  'select id,user_id,provider_id,subject from user_oidc_identities order by id');
export const userCount = () => sql('select count(*) as n from users where deleted_at is null')[0].n;
// 指向已软删/不存在用户的身份行。
export const orphanIdentities = () => sql(
  'select i.id,i.user_id,i.provider_id,i.subject from user_oidc_identities i '
  + 'left join users u on u.id = i.user_id and u.deleted_at is null where u.id is null order by i.id');

// ---------- Modeltaps 后台（root 会话）----------
let rootCookie = null;
async function rootLogin() {
  if (rootCookie) return rootCookie;
  const r = await fetch(MODELTAPS + '/api/user/login', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: 'root', password: env.MODELTAPS_ROOT_PASSWORD }),
  });
  rootCookie = (r.headers.getSetCookie() || []).map((c) => c.split(';')[0]).join('; ');
  const body = await r.json();
  if (!body.success) throw new Error('root 登录失败: ' + body.message);
  return rootCookie;
}

export async function admin(method, url, body) {
  const cookie = await rootLogin();
  const r = await fetch(MODELTAPS + url, {
    method, headers: { cookie, 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  return r.json();
}

// setProvider 局部更新 slug=authgear 提供方（PUT 是全量覆盖，先读后写）。
export async function setProvider(patch) {
  const list = await admin('GET', '/api/oidc_provider/');
  const p = (list.data || []).find((x) => x.slug === env.OIDC_SLUG);
  if (!p) throw new Error('未找到提供方 ' + env.OIDC_SLUG);
  const payload = { ...p, client_secret: CLIENT_SECRET, ...patch };
  const res = await admin('PUT', '/api/oidc_provider/' + p.id, payload);
  if (!res.success) throw new Error('更新提供方失败: ' + res.message);
  return payload;
}

// ---------- AuthGear 配置热改（CONFIG_SOURCE_WATCH=true 自动重载）----------
export function setEmailVerification(enabled) {
  let text = readFileSync(YAML_PATH, 'utf8');
  const want = 'verification:\n  claims:\n    email:\n      enabled: ' + enabled +
    '\n      required: ' + enabled + '\n';
  const next = text.replace(/verification:\n(?:[ ]{2,}.*\n)+/, want);
  if (next === text && !text.includes(want)) throw new Error('authgear.yaml verification 段未匹配');
  writeFileSync(YAML_PATH, next);
  return sleep(4000);
}

// ensureSinkRedirect 给既有 client 追加一个本地 redirect_uri，用来直接抓 id_token。
export function ensureSinkRedirect() {
  const text = readFileSync(YAML_PATH, 'utf8');
  if (text.includes(SINK_REDIRECT)) return Promise.resolve();
  const next = text.replace(/(\n    redirect_uris:\n)/, '$1    - ' + SINK_REDIRECT + '\n');
  if (next === text) throw new Error('authgear.yaml redirect_uris 段未匹配');
  writeFileSync(YAML_PATH, next);
  return sleep(4000);
}

// removeSinkRedirect 撤回 ensureSinkRedirect 写入的 redirect_uri，让 authgear.yaml 回到原状。
export function removeSinkRedirect() {
  const text = readFileSync(YAML_PATH, 'utf8');
  const next = text.replace(new RegExp('[ ]*- ' + SINK_REDIRECT.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\n'), '');
  if (next === text) return Promise.resolve();
  writeFileSync(YAML_PATH, next);
  return sleep(4000);
}

// ---------- 验证码 ----------
// otp.sh 从容器日志里捞最后一条，同一个邮箱在一轮用例里可能先后收到多封（B 注册、C 再注册），
// 新码写进日志前拿到的仍是上一封的旧码，会被判 incorrect。这里记住每个 target 上次用过的码，
// 轮询到出现新码为止。
const lastOtp = new Map();
function readOtp(target) {
  return execFileSync(path.join(AG_DIR, 'otp.sh'), [target], { encoding: 'utf8', cwd: AG_DIR }).trim();
}
export function otp(target) {
  const prev = lastOtp.get(target);
  let code = readOtp(target);
  for (let i = 0; i < 20 && prev && code === prev; i++) {
    execFileSync('sleep', ['1']);
    code = readOtp(target);
  }
  lastOtp.set(target, code);
  return code;
}

// ---------- AuthGear 登录流驱动 ----------
const CONTINUE = 'button[name=x_action][type=submit]:not([disabled]):not(#resend-button)';

// navClick 点击并等待地址变化。AuthGear 的部分步骤页（验证码）会在输入满位时自动提交，
// 也有按钮在输入合法前保持 disabled 的情况，所以点不到时回退按 Enter，并容忍页面已经跳走。
async function navClick(page, selector) {
  const before = page.url();
  try {
    await page.locator(selector).first().click({ timeout: 8000 });
  } catch {
    if (page.url() === before) await page.keyboard.press('Enter').catch(() => {});
  }
  for (let i = 0; i < 60 && page.url() === before; i++) await sleep(250);
  await page.waitForLoadState('domcontentloaded').catch(() => {});
  await sleep(400);
}

// modeltapsLogout 清掉 Modeltaps 侧会话（AuthGear 侧会话保留）。
export async function modeltapsLogout(page) {
  await page.goto(MODELTAPS + '/', { waitUntil: 'domcontentloaded' }).catch(() => {});
  await page.evaluate(async (base) => {
    await fetch(base + '/api/user/logout', { credentials: 'include' }).catch(() => {});
  }, MODELTAPS);
  await sleep(300);
}

// openAuthgearFromModeltaps 打开本站 /login：账号体系是 external，服务端直接 302 到 AuthGear
// 授权页，本站不再渲染登录页，也没有「使用 Authgear 登录」按钮。
export async function openAuthgearFromModeltaps(page, { logoutFirst = false } = {}) {
  if (logoutFirst) await modeltapsLogout(page);
  // 只等 domcontentloaded：302 之后落在 AuthGear 页面上，networkidle 偶发等不到。
  await page.goto(MODELTAPS + '/login', { waitUntil: 'domcontentloaded' });
  await page.waitForURL(/localhost:\d+/, { timeout: 30000 });
  await sleep(500);
}

// runFlow 驱动 AuthGear 的分步页面，直到回到 Modeltaps / 本地 sink / 设置页，或步数用尽。
// 判断依据是页面上实际出现的表单字段而不是 URL：AuthGear 把同一类步骤铺在
// /authflow/v2/* 与 /settings/identity/* 两套路径下，只认 URL 会漏掉后者。
// creds: { loginId, password, otpTarget, useAnotherAccount }；loginId 一律是邮箱。
export async function runFlow(page, creds, maxSteps = 12) {
  const seen = [];
  const visible = async (sel) => (await page.locator(sel + ':visible').count()) > 0;
  for (let i = 0; i < maxSteps; i++) {
    const url = page.url();
    if (url.startsWith(MODELTAPS)) return { steps: seen, landed: 'modeltaps' };
    if (url.startsWith(SINK_REDIRECT.slice(0, -3))) return { steps: seen, landed: 'sink' };
    const label = url.includes('/authflow/v2/') ? url.split('/authflow/v2/')[1].split('?')[0]
      : new URL(url).pathname.replace(/^\//, '');

    if (await visible('input[name=x_code]')) {
      // 同一个邮箱在一轮用例里会被多次用到，AuthGear 在冷却期内不重发、日志里仍是上一封的码，
      // 而那个码已被前一次流程消费掉，直接填会判 incorrect。所以验证码页第二次出现时，
      // 先等冷却结束点一次 Resend 拿新码；再被拒才算真失败。
      const repeat = seen.filter((s) => s === label + '#otp').length;
      if (repeat >= 2) {
        const body = await page.evaluate(() => document.body.innerText).catch(() => '');
        return { steps: seen, landed: 'otp_rejected:' + label, body: body.replace(/\s+/g, ' ').slice(0, 300) };
      }
      if (repeat === 1) {
        const resend = page.locator('#resend-button');
        await resend.waitFor({ state: 'visible', timeout: 5000 }).catch(() => {});
        await page.waitForFunction(() => {
          const b = document.querySelector('#resend-button');
          return b && !b.disabled;
        }, null, { timeout: 90000 }).catch(() => {});
        await resend.click().catch(() => {});
        await sleep(2500);
      }
      seen.push(label + '#otp');
      const code = otp(creds.otpTarget);
      const box = page.locator('input[name=x_code]:visible').first();
      await box.click();
      await box.pressSequentially(code, { delay: 40 });
      await sleep(600);
      await navClick(page, CONTINUE);
    } else if (await visible('input[name=x_confirm_password]')) {
      seen.push(label + '#create_password');
      await page.fill('input[name=x_password]', creds.password);
      await page.fill('input[name=x_confirm_password]', creds.password);
      await navClick(page, CONTINUE);
    } else if (await visible('input[name=x_password]')) {
      seen.push(label + '#password');
      await page.fill('input[name=x_password]', creds.password);
      await navClick(page, CONTINUE);
    } else if (await visible('input[name=q_login_id]')) {
      seen.push(label + '#login_id');
      await page.fill('input[name=q_login_id]', creds.loginId);
      await navClick(page, CONTINUE);
    } else if (await visible('input[name=x_login_id]')) {
      seen.push(label + '#new_login_id');
      await page.fill('input[name=x_login_id]', creds.loginId);
      await navClick(page, CONTINUE);
    } else if (label === 'select_account') {
      seen.push(label);
      if (creds.useAnotherAccount) await navClick(page, 'a:has-text("Use another account")');
      else await navClick(page, 'button:has-text("Continue"), a:has-text("Continue")');
    } else if (await visible('a:has-text("Skip"), button:has-text("Skip")')) {
      seen.push(label + '#skip');
      await navClick(page, 'a:has-text("Skip"), button:has-text("Skip")');
    } else if (label.startsWith('settings')) {
      return { steps: seen, landed: 'settings' };
    } else {
      const body = await page.evaluate(() => document.body.innerText).catch(() => '');
      return { steps: seen, landed: 'stuck:' + label, body: body.replace(/\s+/g, ' ').slice(0, 300) };
    }
  }
  return { steps: seen, landed: 'maxsteps' };
}

// ---------- AuthGear 自助设置页（改邮箱）----------
// 这些操作后面都会落到同一套 authflow 步骤页（验证码等），复用 runFlow 收尾。
export async function settingsChangeEmail(ctx, newEmail) {
  const s = await ctx.newPage();
  await s.goto(AUTHGEAR + '/settings', { waitUntil: 'domcontentloaded' });
  const link = s.locator('a[href*="change_email"]').first();
  if (!(await link.count())) { await s.close(); return { ok: false, reason: '当前账号没有邮箱身份' }; }
  await link.click();
  await sleep(1000);
  const r = await runFlow(s, { loginId: newEmail, otpTarget: newEmail }, 8);
  const body = await s.evaluate(() => document.body.innerText).catch(() => '');
  await s.close();
  return { ok: r.landed === 'settings', flow: r, body: body.replace(/\s+/g, ' ').slice(0, 300) };
}

export async function agLogout(ctx) {
  const s = await ctx.newPage();
  await s.goto(AUTHGEAR + '/settings', { waitUntil: 'domcontentloaded' }).catch(() => {});
  await s.evaluate(() => {
    const f = document.createElement('form');
    f.method = 'POST'; f.action = '/logout';
    document.body.appendChild(f); f.submit();
  }).catch(() => {});
  await sleep(1500);
  await s.close().catch(() => {});
}

// ---------- id_token 抓取 ----------
// 在已登录 AuthGear 的浏览器上下文里走一次授权码流程，落到本地 sink，
// 用 client_secret 换 token 后解出 id_token 的 claim。
export async function captureIdToken(ctx) {
  let resolveCode;
  const codeP = new Promise((r) => { resolveCode = r; });
  const server = createServer((req, res) => {
    resolveCode(new URL(req.url, 'http://127.0.0.1').searchParams.get('code'));
    res.writeHead(200, { 'Content-Type': 'text/plain' });
    res.end('ok');
  });
  await new Promise((r) => server.listen(SINK_PORT, '127.0.0.1', r));
  try {
    const state = 'sink' + Date.now();
    const authUrl = AUTHGEAR + '/oauth2/authorize?' + new URLSearchParams({
      client_id: CLIENT_ID, redirect_uri: SINK_REDIRECT, response_type: 'code',
      scope: 'openid email profile', state,
    });
    const page = await ctx.newPage();
    await page.goto(authUrl, { waitUntil: 'domcontentloaded' }).catch(() => {});
    await runFlow(page, { loginId: '', password: '' }, 6).catch(() => {});
    const code = await Promise.race([codeP, sleep(15000).then(() => null)]);
    await page.close().catch(() => {});
    if (!code) throw new Error('未拿到授权码（AuthGear 会话可能已失效）');
    const r = await fetch(AUTHGEAR + '/oauth2/token', {
      method: 'POST', headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({
        grant_type: 'authorization_code', code, redirect_uri: SINK_REDIRECT,
        client_id: CLIENT_ID, client_secret: CLIENT_SECRET,
      }),
    });
    const tok = await r.json();
    if (!tok.id_token) throw new Error('token 端点未返回 id_token: ' + JSON.stringify(tok));
    const payload = JSON.parse(Buffer.from(tok.id_token.split('.')[1], 'base64url').toString());
    return payload;
  } finally {
    server.close();
  }
}
