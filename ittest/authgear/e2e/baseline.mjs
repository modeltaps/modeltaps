// 基线矩阵：邮箱项目的两种登录组合（邮箱+密码 / 邮箱+验证码）+ 一例「同一账号两个邮箱」，
// 每个用例执行「AuthGear 注册 → Modeltaps 登录 → 读 Modeltaps 账号 → 退出 → 再登录 → 断言同一账号」，
// 并顺带核对退出走的是 RP-Initiated Logout（落回本站 /signed-out）。
// AuthGear 项目只有 email 一种 login_id，用户名 / 手机号组合已不存在（见 init-project.sh）。
// 只读 Modeltaps 数据，不改任何源码。
//   node baseline.mjs [--only <caseId>[,<caseId>]] [--out ../out/baseline.json] [--headed]
import { chromium } from 'playwright';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { bindIdentity, driveAuthgear, dumpScreen, fetchOtp } from './authgear-flow.mjs';

const E2E_DIR = path.dirname(fileURLToPath(import.meta.url));
const AG_DIR = path.resolve(E2E_DIR, '..');

function parseEnvFile(file) {
  const out = {};
  for (const line of fs.readFileSync(file, 'utf8').split('\n')) {
    const m = line.match(/^([A-Z0-9_]+)=(.*)$/);
    if (m) out[m[1]] = m[2];
  }
  return out;
}

const ENV = parseEnvFile(path.join(AG_DIR, 'out/modeltaps.env'));
const AGENV = parseEnvFile(path.join(AG_DIR, 'out/authgear.env'));
const MODELTAPS = ENV.MODELTAPS_BASE;
const DB = path.join(ENV.MODELTAPS_DATA_DIR, 'modeltaps.db');
const PASSWORD = 'Modeltaps-IT-Passw0rd!';

const argv = process.argv.slice(2);
const argOf = (name) => { const i = argv.indexOf(name); return i >= 0 ? argv[i + 1] : null; };
const ONLY = (argOf('--only') || '').split(',').filter(Boolean);
const OUT = argOf('--out') || path.join(AG_DIR, 'out/baseline.json');
const HEADED = argv.includes('--headed');
const GAP_MS = Number(argOf('--gap') ?? process.env.BL_GAP_MS ?? 90000);

const STAMP = process.env.BL_STAMP || Date.now().toString(36).slice(-6);
const uid = (s) => `bl${s}${STAMP}`;
const SIGNED_OUT = `${MODELTAPS}/signed-out`;

function sql(query) {
  return execFileSync('sqlite3', ['-readonly', '-json', DB, query], { encoding: 'utf8' }).trim();
}
function sqlRows(query) {
  const s = sql(query);
  return s ? JSON.parse(s) : [];
}

// modeltapsSnapshot 读当前登录用户（/api/user/self）+ 直接查库补 password / 身份行。
async function modeltapsSnapshot(ctx) {
  const res = await ctx.request.get(`${MODELTAPS}/api/user/self`);
  const body = await res.json();
  if (!body.success) throw new Error(`/api/user/self 失败: ${body.message}`);
  const u = body.data;
  const row = sqlRows(`select id, username, email, display_name, avatar_url, phone_number,
      length(coalesce(password,'')) as pw_len, role, status from users where id=${u.id}`)[0] || {};
  const identities = sqlRows(`select provider_id, subject, created_time
      from user_oidc_identities where user_id=${u.id} order by id`);
  return { id: u.id, username: row.username, email: row.email, display_name: row.display_name,
    avatar_url: row.avatar_url, phone_number: row.phone_number, password_len: row.pw_len,
    role: row.role, status: row.status, identity_count: identities.length, identities };
}

const subjDigest = (s) => (s ? `${s.slice(0, 8)}…${s.slice(-4)} (len ${s.length})` : '');

// captureIdToken 复用浏览器里已建立的 AuthGear 会话，另跑一次授权码流程并在回调打到
// Modeltaps 之前拦下来，自己拿 code 换 id_token。纯旁路观测：Modeltaps 不参与，也不改任何源码。
async function captureIdToken(ctx) {
  const disc = await (await ctx.request.get(`${AGENV.AUTHGEAR_ORIGIN}/.well-known/openid-configuration`)).json();
  const redirectUri = ENV.OIDC_REDIRECT_URI;
  const page = await ctx.newPage();
  let captured = null;
  page.on('request', (req) => { if (!captured && req.url().startsWith(redirectUri)) captured = req.url(); });
  await page.route(`${redirectUri}*`, (route) => route.abort());
  const url = new URL(disc.authorization_endpoint);
  url.searchParams.set('client_id', AGENV.OIDC_CLIENT_ID);
  url.searchParams.set('redirect_uri', redirectUri);
  url.searchParams.set('response_type', 'code');
  url.searchParams.set('scope', (ENV.OIDC_SCOPES || 'openid,email,profile').split(',').join(' '));
  url.searchParams.set('state', 'probe' + Math.random().toString(36).slice(2));
  await page.goto(url.toString()).catch(() => {});
  // 会话已存在时 AuthGear 会先停在 select_account 屏，要点一下 Continue 才发回调。
  for (let i = 0; i < 60 && !captured; i++) {
    const go = page.locator('button[name="x_action"][value="continue"]');
    if (await go.count().catch(() => 0)) await go.first().click().catch(() => {});
    await page.waitForTimeout(200);
  }
  await page.close();
  if (!captured) return { error: '未截获授权回调' };
  const code = new URL(captured).searchParams.get('code');
  if (!code) return { error: `回调无 code：${captured}` };
  const res = await ctx.request.post(disc.token_endpoint, {
    form: {
      grant_type: 'authorization_code', code, redirect_uri: redirectUri,
      client_id: AGENV.OIDC_CLIENT_ID, client_secret: AGENV.OIDC_CLIENT_SECRET,
    },
  });
  const body = await res.json();
  if (!body.id_token) return { error: `换取 id_token 失败：${JSON.stringify(body).slice(0, 200)}` };
  const payload = JSON.parse(Buffer.from(body.id_token.split('.')[1], 'base64url').toString());
  return {
    sub: payload.sub, preferred_username: payload.preferred_username, email: payload.email,
    email_verified: payload.email_verified, phone_number: payload.phone_number,
    phone_number_verified: payload.phone_number_verified, name: payload.name,
    all_claims: Object.keys(payload).sort(),
  };
}

// afterLogin 可选，在同一个页面（AuthGear 会话仍在）上再做点事，比如去设置页加绑标识。
// 账号体系是 external：浏览器导航到 /login 由服务端直接 302 到 AuthGear 授权页，
// 本站不再渲染登录页，也就没有「使用 Authgear 登录」按钮可点。
async function loginToModeltaps(ctx, opts, afterLogin) {
  const page = await ctx.newPage();
  const trace = [];
  try {
    await page.goto(`${MODELTAPS}/login`, { waitUntil: 'domcontentloaded' });
    await page.waitForURL(/localhost:\d+\//, { timeout: 30000 });
    await driveAuthgear(page, MODELTAPS, { ...opts, trace });
    await page.waitForURL((u) => u.href.startsWith(MODELTAPS), { timeout: 30000 });
    await page.waitForLoadState('networkidle').catch(() => {});
    const finalUrl = page.url();
    const bodyText = (await page.locator('body').innerText()).slice(0, 300);
    if (afterLogin) await afterLogin(page);
    return { trace, finalUrl, bodyText };
  } catch (err) {
    if (err.screen) console.error('未识别屏幕:', JSON.stringify(err.screen, null, 2));
    else console.error('当前屏幕:', JSON.stringify(await dumpScreen(page).catch(() => ({})), null, 2));
    throw err;
  } finally {
    await page.close();
  }
}

// logout 走本站登出接口：经提供方登录的会话会拿到 IdP 的结束会话地址（RP-Initiated Logout），
// browser=true 时整页跳过去，断言 IdP 结束会话后回落本站 /signed-out。
// 跳转完成后清掉两边 Cookie，让下一次登录从零开始。
async function logout(ctx, { browser = false } = {}) {
  const res = await ctx.request.get(`${MODELTAPS}/api/user/logout`);
  const body = await res.json().catch(() => ({}));
  const redirectUrl = body?.data?.redirect_url || '';
  const rec = { has_redirect_url: !!redirectUrl, landed: '' };
  if (browser && redirectUrl) {
    const page = await ctx.newPage();
    await page.goto(redirectUrl, { waitUntil: 'domcontentloaded' }).catch(() => {});
    await page.waitForLoadState('networkidle').catch(() => {});
    rec.landed = page.url();
    await page.close();
  }
  await ctx.clearCookies();
  return rec;
}

const CASES = [
  { id: 'email_otp', label: '邮箱+验证码',
    loginId: () => `${uid('eo')}@bltest.local`, password: null },
  { id: 'email_password', label: '邮箱+密码',
    loginId: () => `${uid('ep')}@bltest.local`, password: PASSWORD },
];

// runMultiIdentity 构造「同一个 AuthGear 账号绑两个邮箱」，然后用两个邮箱分别登录 Modeltaps，
// 断言两次都落在同一个 Modeltaps 账号上、且只建了一个号。
async function runMultiIdentity(browser) {
  const email1 = `${uid('m1')}@bltest.local`;
  const email2 = `${uid('m2')}@bltest.local`;
  const rec = { id: 'multi_identity', label: '同一账号两个邮箱',
    login_id: `${email1} / ${email2}` };

  // 先用第一个邮箱+密码建号，再在 AuthGear 设置页给同一账号加绑第二个邮箱。
  const setup = await browser.newContext();
  const first = await loginToModeltaps(setup, { loginId: email1, otpTarget: email1, password: PASSWORD });
  rec.signup_trace = first.trace;
  const snapBase = await modeltapsSnapshot(setup);
  rec.modeltaps = snapBase;
  rec.subject_digest = subjDigest(snapBase.identities[0]?.subject);
  const bindPage = await setup.newPage();
  try {
    await bindIdentity(bindPage, AGENV.AUTHGEAR_ORIGIN, email2);
  } finally {
    await bindPage.close();
  }
  rec.id_token = await captureIdToken(setup).catch((e) => ({ error: e.message }));
  rec.logout = await logout(setup, { browser: true });
  rec.signed_out_ok = rec.logout.landed.startsWith(SIGNED_OUT);
  await setup.close();

  // 两个邮箱各开一个全新上下文登录，逐一比对 Modeltaps user id。
  const attempts = [
    { via: 'email1', opts: { loginId: email1, otpTarget: email1, password: PASSWORD } },
    { via: 'email2', opts: { loginId: email2, otpTarget: email2, password: PASSWORD } },
  ];
  rec.logins = [];
  for (const a of attempts) {
    const ctx = await browser.newContext();
    try {
      const r = await loginToModeltaps(ctx, a.opts);
      const snap = await modeltapsSnapshot(ctx);
      rec.logins.push({ via: a.via, modeltaps_id: snap.id, username: snap.username,
        email: snap.email, identity_count: snap.identity_count, trace: r.trace });
      await logout(ctx);
    } finally {
      await ctx.close();
    }
  }

  const subject = snapBase.identities[0]?.subject;
  const owned = subject
    ? sqlRows(`select count(*) as n, count(distinct user_id) as u
        from user_oidc_identities where subject='${subject}'`)[0]
    : { n: 0, u: 0 };
  const ids = [...new Set(rec.logins.map((l) => l.modeltaps_id))];
  rec.distinct_modeltaps_ids = ids;
  rec.relogin_same_account = ids.length === 1 && ids[0] === snapBase.id;
  rec.identity_rows_for_subject = owned.n;
  rec.users_created = owned.u;
  rec.ok = rec.relogin_same_account && owned.u === 1 && owned.n === 1 && rec.signed_out_ok;
  return rec;
}

async function runCase(browser, def) {
  const loginId = def.loginId();
  const opts = { loginId, otpTarget: loginId, password: def.password };
  const rec = { id: def.id, label: def.label, login_id: loginId };

  const ctx1 = await browser.newContext();
  const first = await loginToModeltaps(ctx1, opts);
  rec.signup_trace = first.trace;
  const snap1 = await modeltapsSnapshot(ctx1);
  rec.modeltaps = snap1;
  rec.subject_digest = subjDigest(snap1.identities[0]?.subject);
  rec.id_token = await captureIdToken(ctx1).catch((e) => ({ error: e.message }));
  rec.logout = await logout(ctx1, { browser: true });
  rec.signed_out_ok = rec.logout.landed.startsWith(SIGNED_OUT);
  await ctx1.close();

  const ctx2 = await browser.newContext();
  const second = await loginToModeltaps(ctx2, opts);
  rec.relogin_trace = second.trace;
  const snap2 = await modeltapsSnapshot(ctx2);
  await logout(ctx2);
  await ctx2.close();

  // 「新增用户」按 AuthGear subject 归因，而不是全库 users 计数差：
  // 其他代理可能同时在同一套环境里建号，总数差会被污染。
  const subject = snap1.identities[0]?.subject;
  const owned = subject
    ? sqlRows(`select count(*) as n, count(distinct user_id) as u
        from user_oidc_identities where subject='${subject}'`)[0]
    : { n: 0, u: 0 };
  rec.relogin_same_account = snap2.id === snap1.id;
  rec.identity_rows_for_subject = owned.n;
  rec.users_created = owned.u;
  // 项目里没有手机号标识，IdP 不会下发 phone_number claim，users.phone_number 应保持为空。
  rec.phone_number_empty = !snap1.phone_number && !snap2.phone_number;
  rec.ok = rec.relogin_same_account && owned.u === 1 && owned.n === 1
    && rec.phone_number_empty && rec.signed_out_ok;
  return rec;
}

function table(records) {
  const cols = ['组合', 'Modeltaps id', 'username', 'email', 'display_name', '密码长度', '身份行', '再登录同账号', '新增用户', '退出落 /signed-out'];
  const rows = records.map((r) => [
    r.label + (r.error ? ' (失败)' : ''),
    r.modeltaps?.id ?? '-',
    r.modeltaps?.username ?? '-',
    r.modeltaps?.email || '(空)',
    r.modeltaps?.display_name || '(空)',
    r.modeltaps?.password_len ?? '-',
    r.modeltaps?.identity_count ?? '-',
    r.relogin_same_account === undefined ? '-' : (r.relogin_same_account ? '是' : '否'),
    r.users_created ?? '-',
    r.signed_out_ok === undefined ? '-' : (r.signed_out_ok ? '是' : '否'),
  ].map(String));
  const w = cols.map((c, i) => Math.max([...c].length, ...rows.map((r) => [...r[i]].length)));
  const line = (cells) => '| ' + cells.map((c, i) => c.padEnd(w[i] - ([...c].length - c.length))).join(' | ') + ' |';
  return [line(cols), '|' + w.map((n) => '-'.repeat(n + 2)).join('|') + '|', ...rows.map(line)].join('\n');
}

async function main() {
  const browser = await chromium.launch({ headless: !HEADED });
  const records = [];
  try {
    const plan = [
      ...CASES.map((def) => ({ id: def.id, label: def.label, run: () => runCase(browser, def) })),
      { id: 'multi_identity', label: '同一账号两个邮箱', run: () => runMultiIdentity(browser) },
    ];
    let ran = 0;
    for (const item of plan) {
      if (ONLY.length && !ONLY.includes(item.id)) continue;
      // AuthGear 对验证码发送有冷却窗口（"Please wait for a moment before retrying"），
      // 全量连跑时相邻用例会撞上，用例之间退避一下。
      if (ran++) await new Promise((r) => setTimeout(r, GAP_MS));
      process.stderr.write(`\n=== ${item.id} (${item.label}) ===\n`);
      try {
        const rec = await item.run();
        records.push(rec);
        process.stderr.write(`${rec.ok ? 'OK' : 'MISMATCH'} ${JSON.stringify(rec.modeltaps)}\n`);
      } catch (err) {
        records.push({ id: item.id, label: item.label, error: err.message, ok: false });
        process.stderr.write(`FAIL ${err.message}\n`);
      }
    }
  } finally {
    await browser.close();
  }
  const result = { stamp: STAMP, modeltaps: MODELTAPS, generated_at: new Date().toISOString(), cases: records };
  fs.mkdirSync(path.dirname(OUT), { recursive: true });
  fs.writeFileSync(OUT, JSON.stringify(result, null, 2));
  console.log(table(records));
  console.log(`\nJSON: ${OUT}`);
  process.exit(records.every((r) => r.ok) ? 0 : 1);
}

await main();
