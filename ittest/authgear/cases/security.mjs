// 安全边界用例 S1–S7：只判定行为，不修改 Modeltaps 源码。
//   node cases/security.mjs --out out/security.json [--only S1,S7]
// 前置：./up.sh && ./init-project.sh && ./modeltaps-up.sh 已跑过。
// AuthGear 项目只有 email 一种 login_id，因此依赖用户名（S3）与手机号（S4）的两条用例
// 在本项目形态下不成立，按 SKIP 记录原因，不静默跳过。
// 用例会临时改动 AuthGear 的邮箱验证开关与 Modeltaps 提供方的 link_by_verified_email /
// disable_auto_register，结束时恢复为「邮箱验证开启 + 两个开关均关闭」。
import { chromium } from 'playwright';
import { writeFileSync, mkdirSync } from 'node:fs';
import path from 'node:path';
import * as L from './seclib.mjs';

const args = process.argv.slice(2);
const argOf = (n, d) => { const i = args.indexOf(n); return i >= 0 ? args[i + 1] : d; };
const OUT = path.resolve(L.AG_DIR, argOf('--out', 'out/security.json'));
const ONLY = argOf('--only', '') ? argOf('--only', '').split(',') : null;
// 本地 username 有 12 字符上限，前缀留短一点，后缀才够用。
const RUN = 'sc' + Date.now().toString().slice(-5);
const PW = 'Modeltaps-Sec-Pw-1!';
const results = [];

const name = (s) => RUN + s;
const mail = (s) => RUN + s + '@sectest.local';

// 新建一个隔离的浏览器上下文（AuthGear 与 Modeltaps 会话都是干净的）。
async function fresh(browser) {
  const ctx = await browser.newContext();
  return { ctx, page: await ctx.newPage() };
}

// 在 AuthGear 注册一个新账号（邮箱标识）并一路走到 Modeltaps。返回流程轨迹与落地 URL。
async function signupAndLogin(ctx, page, { loginId, otpTarget, password }) {
  await L.openAuthgearFromModeltaps(page);
  const flow = await L.runFlow(page, { loginId, password, otpTarget: otpTarget ?? loginId });
  return { flow, url: page.url() };
}

// 读当前 Modeltaps 会话对应的账号（未登录返回 null）。
async function modeltapsSelf(page) {
  // fetch 必须同源发起，页面可能还停在 AuthGear 上（流程卡住时）。
  if (!page.url().startsWith(L.MODELTAPS)) {
    await page.goto(L.MODELTAPS + '/login', { waitUntil: 'domcontentloaded' }).catch(() => {});
  }
  return page.evaluate(async (base) => {
    const r = await fetch(base + '/api/user/self', { credentials: 'include' });
    return r.json();
  }, L.MODELTAPS);
}

// 以 root 建一个本地 Modeltaps 账号（有密码，可选邮箱）。
async function createLocalUser(username, email) {
  const r = await L.admin('POST', '/api/user/', { username, password: PW, display_name: username });
  if (!r.success) throw new Error('建本地用户失败: ' + r.message);
  const u = L.userByName(username);
  if (email) {
    const full = await L.admin('GET', '/api/user/' + u.id);
    const up = await L.admin('PUT', '/api/user/', { ...full.data, email });
    if (!up.success) throw new Error('回填邮箱失败: ' + up.message);
  }
  return L.userByName(username);
}

function record(id, title, expected, actual, verdict, risk, note) {
  results.push({ id, title, expected, actual, verdict, risk, note });
  console.log('[' + id + '] ' + verdict + ' / 风险 ' + risk + ' — ' + title);
}

// ---------------------------------------------------------------- S1
// 未验证邮箱：AuthGear 关掉邮箱验证后注册邮箱账号，Modeltaps 不得写邮箱、不得并入同邮箱老账号。
async function S1(browser) {
  await L.setEmailVerification(false);
  await L.setProvider({ link_by_verified_email: true });
  const victimName = name('v1');
  const sharedMail = mail('shared1');
  const victim = await createLocalUser(victimName, sharedMail);

  const { ctx, page } = await fresh(browser);
  const before = L.userCount();
  const r = await signupAndLogin(ctx, page, { loginId: sharedMail, password: PW });
  const claims = await L.captureIdToken(ctx).catch((e) => ({ error: e.message }));
  const self = await modeltapsSelf(page);
  const after = L.userCount();
  const newUser = self.success ? L.userById(self.data.id) : null;
  await ctx.close();

  const merged = newUser && newUser.id === victim.id;
  const emailWritten = !!(newUser && newUser.email);
  const ok = !merged && !emailWritten && claims.email_verified !== true;
  record('S1', '未验证邮箱不得写入、不得关联',
    'email_verified=false；新账号 email 为空；不并入同邮箱本地账号 #' + victim.id,
    {
      flow: r.flow, id_token: pick(claims), modeltaps_user: newUser,
      victim: { id: victim.id, email: victim.email }, users_before: before, users_after: after,
      merged, email_written: emailWritten,
    },
    ok ? '符合' : '不符', ok ? '无' : '高',
    ok ? 'oidcTrustedEmail 在 email_verified 非 true 时返回空串，关联与回填两条路径同时被掐断（controller/oidc.go:113-124、167、374）。'
      : '未验证邮箱被采信，存在账号接管路径。');
}

const pick = (c) => c && ({
  sub: c.sub, email: c.email, email_verified: c.email_verified,
  preferred_username: c.preferred_username, phone_number: c.phone_number,
  phone_number_verified: c.phone_number_verified, error: c.error,
});

// ---------------------------------------------------------------- S2
// 已验证邮箱关联：这是设计内行为，本用例把「接管面」量化——IdP 侧任意账号只要验证了
// 同一邮箱，就能直接进入 Modeltaps 已有账号。
async function S2(browser) {
  await L.setEmailVerification(true);
  await L.setProvider({ link_by_verified_email: true });
  const victimName = name('v2');
  const sharedMail = mail('shared2');
  const victim = await createLocalUser(victimName, sharedMail);

  const { ctx, page } = await fresh(browser);
  const before = L.userCount();
  const r = await signupAndLogin(ctx, page, { loginId: sharedMail, password: PW });
  const claims = await L.captureIdToken(ctx).catch((e) => ({ error: e.message }));
  const self = await modeltapsSelf(page);
  const after = L.userCount();
  await ctx.close();

  const landedOnVictim = self.success && self.data.id === victim.id;
  const ids = victim ? L.identitiesOf(victim.id) : [];
  record('S2', '按已验证邮箱关联 = 设计内的接管面',
    '进入本地账号 #' + victim.id + '（设计内），并补写一行身份行；不新增用户',
    {
      flow: r.flow, id_token: pick(claims), landed_user: self.success ? self.data.id : null,
      victim_id: victim.id, victim_identities: ids, users_before: before, users_after: after,
    },
    landedOnVictim ? '符合（设计内）' : '不符', landedOnVictim ? '中' : '中',
    '前提必须写清：只有当 IdP 侧邮箱唯一且必经验证时才安全。IdP 一旦允许邮箱被释放/转移，'
    + '接手方即可无声进入 Modeltaps 原账号（controller/oidc.go:163-181）。默认关闭该开关是对的。');
}

// ---------------------------------------------------------------- S2b
// 邮箱易主：AuthGear 上把 X 从账号 B 改走，再由账号 C 验证 X 后登录 Modeltaps。
async function S2b(browser) {
  await L.setEmailVerification(true);
  await L.setProvider({ link_by_verified_email: true });
  const victimName = name('v2b');
  const sharedMail = mail('shared2b');
  const movedMail = mail('moved2b');
  const victim = await createLocalUser(victimName, sharedMail);

  // B：先用 X 登录并占住 victim 账号
  const b1 = await fresh(browser);
  await signupAndLogin(b1.ctx, b1.page, { loginId: sharedMail, password: PW });
  const selfB = await modeltapsSelf(b1.page);
  const claimsB = await L.captureIdToken(b1.ctx).catch((e) => ({ error: e.message }));
  // B 在 IdP 侧把邮箱换走，释放 X
  const changed = await L.settingsChangeEmail(b1.ctx, movedMail);
  const claimsBAfter = await L.captureIdToken(b1.ctx).catch((e) => ({ error: e.message }));
  await b1.ctx.close();

  // C：新 AuthGear 账号接手 X
  const c1 = await fresh(browser);
  const before = L.userCount();
  const r = await signupAndLogin(c1.ctx, c1.page, { loginId: sharedMail, password: PW });
  const claims = await L.captureIdToken(c1.ctx).catch((e) => ({ error: e.message }));
  // 冲突时回调页会停在原地展示专属提示，modeltapsSelf 会离开该页，先把文案抓下来。
  const calloutText = await c1.page
    .evaluate(() => (document.body ? document.body.innerText : '').slice(0, 400))
    .catch(() => '');
  const selfC = await modeltapsSelf(c1.page);
  const after = L.userCount();
  await c1.ctx.close();

  // 只有「C 确实是另一个 IdP 主体」时才算构造成功；否则说明 IdP 没放行邮箱易主，
  // C 只是以 B 的身份重新登录了一次。
  const distinctSubject = claims && claimsB && claims.sub && claimsB.sub && claims.sub !== claimsB.sub;
  const constructed = !!(changed.ok && distinctSubject);
  const landedOnVictim = !!(selfC.success && selfC.data.id === victim.id);
  const rejected = !!(constructed && !landedOnVictim && after === before);
  record('S2b', 'IdP 侧邮箱易主后接手方能否进入原账号',
    '换手成功时 C 应被拒绝：不进入 #' + victim.id + '、不新增用户，回调页展示关联冲突提示',
    {
      victim_id: victim.id, b_landed: selfB.success ? selfB.data.id : null,
      b_id_token: pick(claimsB), b_id_token_after_change: pick(claimsBAfter),
      change_email: changed, c_flow: r.flow, c_id_token: pick(claims),
      c_landed: selfC.success ? selfC.data.id : null,
      c_callback_text: calloutText,
      users_before: before, users_after: after,
      distinct_idp_subject: distinctSubject, scenario_constructed: constructed,
      landed_on_victim: landedOnVictim, rejected,
      victim_identities: L.identitiesOf(victim.id),
    },
    constructed ? (rejected ? '符合（已拒绝）' : '不符（可接管）') : '未能构造',
    constructed ? (rejected ? '低' : '高') : '待决策',
    !constructed
      ? 'IdP 侧未放行邮箱易主，本条接管路径在该 IdP 上不成立；换一个允许释放邮箱的 IdP 仍需重新评估。'
      : rejected
        ? 'SEC-24 修复生效：resolveOidcUser 在按已验证邮箱关联前检查目标账号在同一提供方下是否已有身份行，'
        + '已有则拒绝自动关联，回调返回 OIDC_LINK_CONFLICT: 前缀消息，要求用户先登录后手动绑定。'
        : 'B 与 C 是两个不同 IdP 主体，却都落到同一个 Modeltaps 账号：身份行按 (provider, subject) 唯一，'
        + '但按邮箱二次关联未拦住同提供方的另一个 subject，SEC-24 修复未生效。');
}

// ---------------------------------------------------------------- S3（跳过）
// preferred_username 边界依赖「IdP 侧存在用户名标识」：本项目只有 email 一种 login_id，
// AuthGear 不会下发 preferred_username，这条边界在该项目形态下无从构造。
async function S3() {
  record('S3', 'preferred_username 边界与 IdP 侧改名', '—', {
    skip_reason: 'AuthGear 项目只有 email 一种 login_id（init-project.sh），不存在用户名标识，'
      + 'id_token 里也就没有 preferred_username；要复跑需临时把 username 加回 identity.login_id.keys。',
  }, 'SKIP', '待决策',
  '用户名回落规则（sanitizeOAuthUsername / oauthUsername，controller/common.go:267-283）'
  + '仍由单测覆盖；本套 E2E 在邮箱项目上不做判定。');
}

// ---------------------------------------------------------------- S4
// 孤岛账号：经提供方建出来的账号本地没有密码，OIDC 是唯一登录方式，解绑必须被拒。
// 原手机号版本（纯手机号账号既无邮箱也无密码）在邮箱项目上不成立，改为等价的邮箱账号版本。
async function S4(browser) {
  await L.setEmailVerification(true);
  await L.setProvider({ link_by_verified_email: false });
  const login = mail('iso4');
  const { ctx, page } = await fresh(browser);
  const r = await signupAndLogin(ctx, page, { loginId: login, password: PW });
  const claims = await L.captureIdToken(ctx).catch((e) => ({ error: e.message }));
  const self = await modeltapsSelf(page);
  const u = self.success ? L.userById(self.data.id) : null;
  const unbind = await page.evaluate(async (base) => {
    const r2 = await fetch(base + '/api/user/unbind', {
      method: 'POST', credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ type: 'oidc', provider: 'authgear' }),
    });
    return r2.json();
  }, L.MODELTAPS);
  await ctx.close();

  // 本地无密码：AuthGear 侧的密码不是 Modeltaps 的凭据，users.password 必须为空。
  const isolated = !!u && u.pw_len === 0;
  const blocked = unbind && unbind.success === false;
  const ok = isolated && blocked;
  record('S4', '唯一登录方式的账号解绑必须被拒',
    '账号本地无密码；/api/user/unbind(oidc/authgear) 被 SEC-23 拒绝',
    {
      flow: r.flow, id_token: pick(claims), modeltaps_user: u,
      identities: u ? L.identitiesOf(u.id) : [], unbind_response: unbind,
      isolated, unbind_blocked: blocked,
      phone_variant_skipped: '项目无手机号标识，原「纯手机号孤岛账号」变体不适用',
    },
    ok ? '符合' : '不符', ok ? '中' : '高',
    'SEC-23 拦住了「自己把自己锁死」，但账号本身仍是孤岛：本地不存密码，'
    + '一旦 IdP 侧账号丢失或提供方被停用，用户没有任何本地找回路径，只能走人工。'
    + '风险定「中」而非「低」：这是可运营性缺口，不是可利用漏洞。');
}

// ---------------------------------------------------------------- S5
// 邮箱变更同步：IdP 侧把邮箱从 X 改成 Y，本站身份提供方（first_party）下 Modeltaps 应跟着改。
async function S5(browser) {
  await L.setEmailVerification(true);
  await L.setProvider({ link_by_verified_email: false });
  const oldMail = mail('old5');
  const newMail = mail('new5');
  const { ctx, page } = await fresh(browser);
  await signupAndLogin(ctx, page, { loginId: oldMail, password: PW });
  const self1 = await modeltapsSelf(page);
  const u1 = self1.success ? L.userById(self1.data.id) : null;

  const changed = await L.settingsChangeEmail(ctx, newMail);
  const claims = await L.captureIdToken(ctx).catch((e) => ({ error: e.message }));

  const usersBefore = L.userCount();
  await L.openAuthgearFromModeltaps(page, { logoutFirst: true });
  const again = await L.runFlow(page, { loginId: newMail, password: PW, otpTarget: newMail });
  const self2 = await modeltapsSelf(page);
  const u2 = self2.success ? L.userById(self2.data.id) : null;
  await ctx.close();

  const sameAccount = u1 && u2 && u1.id === u2.id;
  const emailSynced = !!(u2 && u2.email === newMail);
  const noNewUser = usersBefore === L.userCount();
  const ok = sameAccount && emailSynced && noNewUser;
  record('S5', 'IdP 侧改邮箱后 Modeltaps 跟随同步',
    '仍进原账号 #' + (u1 && u1.id) + '，Modeltaps email 更新为 ' + newMail + '，不新增用户',
    {
      change_email: changed, id_token_after_change: pick(claims), relogin_flow: again,
      user_before: u1, user_after: u2, users_before: usersBefore, users_after: L.userCount(),
      same_account: sameAccount, modeltaps_email_synced: emailSynced, no_new_user: noNewUser,
    },
    ok ? '符合' : '不符', ok ? '低' : '中',
    '按 (provider, subject) 命中身份行即登录，随后 syncOidcEmail 按提供方权威程度决定是否写邮箱：'
    + 'first_party 提供方即本站账号体系，邮箱以 IdP 为准（本地为空或与 IdP 不同都覆盖）；'
    + '第三方提供方只在本地为空时回填。两档都不抢已归属其它账号的邮箱。'
    + '本环境的 authgear 提供方标记为 first_party，因此改邮箱后再登录，本站邮箱跟随更新，'
    + '不再留下用户已不再控制的陈旧邮箱。');
}

// ---------------------------------------------------------------- S6
// SEC-29 回归：登录态用户被后台删除后再走一次 OIDC 登录，绑定分支不得写出孤儿身份行。
// 会话表落地后 DeleteUser 会连带删掉该用户的全部会话行（model/user.go Delete），
// 「残留会话」这个前置在当前版本已不复存在：登录态随删号即失效，回调走的是注册分支。
// 因此本例的判定改为：删号即会话失效（前置被消除）或绑定分支明确拒绝，两者都不得留孤儿行。
async function S6(browser) {
  await L.setEmailVerification(true);
  await L.setProvider({ link_by_verified_email: false });
  const loginA = mail('o1');
  const loginB = mail('o2');

  // A：正常注册登录，拿到 Modeltaps 账号
  const { ctx, page } = await fresh(browser);
  await signupAndLogin(ctx, page, { loginId: loginA, password: PW });
  const self = await modeltapsSelf(page);
  const victimId = self.success ? self.data.id : null;
  const orphansBefore = L.orphanIdentities();

  // 后台删号：会话行随之删除，浏览器里的 cookie 变成死 cookie
  const del = await L.admin('DELETE', '/api/user/' + victimId);
  const orphansAfterDelete = L.orphanIdentities();
  const stillLoggedIn = await modeltapsSelf(page);

  // 带着残留会话，再走一次 /login（服务端 302 到 IdP）→ 回调进 oidcBind 分支。
  // 回调页只显示通用失败文案（OAuthCallback.jsx 对 success=false 统一提示后回登录页），
  // 后端判定文案只在 /api/oauth/oidc 的响应体里，所以在导航前就挂上响应监听。
  await L.agLogout(ctx);
  const bindRespP = page.waitForResponse((r) => r.url().includes('/api/oauth/oidc'), { timeout: 90000 })
    .then((r) => r.text()).catch(() => '');
  await L.openAuthgearFromModeltaps(page);
  // 归因更正：实测到达 oidcBind 的路径并不是「用 loginB 新建了一个 IdP 账号」——
  // agLogout 后 AuthGear 侧会话未必真的清干净，流程停在账号选择页，runFlow 点
  // Continue 复用了受害者自身的 IdP 账号回到 Modeltaps 回调，带着残留的 Modeltaps 会话
  // 进入绑定分支。判定只看「回调是否返回拒绝文案 + 是否新增孤儿身份行」，
  // 与新旧 IdP 主体无关，故该路径同样有效。
  const again = await L.runFlow(page, { loginId: loginB, password: PW, otpTarget: loginB });
  const bindRespText = (await bindRespP).replace(/\s+/g, ' ').slice(0, 300);
  const bodyText = await page.evaluate(() => document.body.innerText).catch(() => '');
  const orphansAfter = L.orphanIdentities();
  await ctx.close();

  const newOrphan = orphansAfter.filter((i) => !orphansBefore.some((j) => j.id === i.id));
  const bindWroteOrphan = orphansAfter.length > orphansAfterDelete.length;
  const DENY = '用户已被封禁或不存在';
  const reachedBind = bindRespText.includes(DENY) || bodyText.includes(DENY);
  // 删号后会话是否还活着：活着才谈得上「带残留会话进绑定分支」。
  const sessionAlive = !!(stillLoggedIn && stillLoggedIn.success);

  // 附带观察：软删用户的身份行是否被清理（SEC-26）。
  const leftover = orphansAfterDelete.length > orphansBefore.length;

  const ok = !bindWroteOrphan && (reachedBind || !sessionAlive);
  record('S6', '已删除账号不得经 OIDC 回调留下孤儿身份行（SEC-29 回归）',
    '删除用户后再走一次 OIDC 登录：会话应已随删号失效（或绑定分支判「' + DENY + '」拒绝），'
    + '任一路径都不得写出指向已删除用户的 user_oidc_identity 行',
    {
      victim_id: victimId, delete_response: del,
      session_alive_after_delete: sessionAlive,
      orphans_before: orphansBefore.length, orphans_after_delete: orphansAfterDelete.length,
      orphans_after_bind: orphansAfter.length, new_orphan_rows: newOrphan,
      relogin_flow: again, page_text: bodyText.replace(/\s+/g, ' ').slice(0, 200),
      bind_response_text: bindRespText, reached_bind_branch: reachedBind,
      bind_wrote_orphan: bindWroteOrphan, delete_left_identity_row: leftover,
      orphan_rows_sample: orphansAfter.slice(-3),
    },
    bindWroteOrphan ? '不符（已复现）' : (ok ? '符合' : '不符'),
    bindWroteOrphan ? '中' : (ok ? '无' : '待决策'),
    (bindWroteOrphan
      ? 'OIDCAuth 只看 session.Get("id") 非空就走 oidcBind，oidcBind 若不校验会话用户存在性，'
      + '就会拿着已删除账号的 userId 写出孤儿身份行。SEC-29 的修复未生效或被回退，需立即复核。'
      : sessionAlive
        ? 'oidcBind 绑定前先用 model.IsUserEnabled 校验会话用户仍存在（软删在默认 scope 下查不到）'
        + '且未被封禁，不过则直接拒绝，既不查身份行也不写行。本例确认：孤儿身份行未新增，'
        + '响应为「' + DENY + '」。'
        : '会话表落地后 DeleteUser 与软删同事务删掉该用户的全部会话行（model/user.go Delete），'
        + '删号后浏览器里的 cookie 立刻失效，「带残留会话进 oidcBind」这个前置已不成立——'
        + '本次回调走的是注册分支，另建了一个全新账号。SEC-29 的代码层判定仍由单测覆盖'
        + '（controller/oidc_test.go），E2E 在此只确认：删号后没有任何孤儿身份行产生。')
    + ' 附带：DeleteUser 是软删（users.deleted_at 置位、username 改写成 _del_*），'
    + '并在同一事务里删除该用户的 user_oidc_identities 行（SEC-26），'
    + '原 IdP 主体因此仍能重新建号（见 S6b）。');
}

// ---------------------------------------------------------------- S6b
// 删号后残留的身份行是否让原 IdP 账号无法重新建号。
async function S6b(browser) {
  await L.setEmailVerification(true);
  await L.setProvider({ link_by_verified_email: false });
  const login = mail('o3');
  const { ctx, page } = await fresh(browser);
  await signupAndLogin(ctx, page, { loginId: login, password: PW });
  const self = await modeltapsSelf(page);
  const victimId = self.success ? self.data.id : null;
  const del = await L.admin('DELETE', '/api/user/' + victimId);
  const leftover = L.sql('select id,user_id,subject from user_oidc_identities where user_id=' + victimId);

  const usersBefore = L.userCount();
  await L.openAuthgearFromModeltaps(page, { logoutFirst: true });
  const again = await L.runFlow(page, { loginId: login, password: PW });
  const bodyText = await page.evaluate(() => document.body.innerText).catch(() => '');
  const self2 = await modeltapsSelf(page);
  await ctx.close();

  const recovered = self2 && self2.success;
  record('S6b', '账号被删后原 IdP 主体能否重新建号',
    '删号后清掉会话再登录，应能建一个全新的 Modeltaps 账号',
    {
      victim_id: victimId, delete_response: del, leftover_identity_rows: leftover,
      relogin_flow: again, page_text: bodyText.replace(/\s+/g, ' ').slice(0, 250),
      users_before: usersBefore, users_after: L.userCount(),
      logged_in_after: recovered ? self2.data.id : null,
    },
    recovered ? '符合' : '不符', recovered ? '无' : '中',
    recovered
      ? '残留身份行未阻塞重新建号。'
      : 'DeleteUser 软删用户却保留 user_oidc_identities 行，(provider_id, subject) 唯一索引'
      + '使 oidcRegister 的事务在插入身份行时必然冲突（controller/oidc.go:377-394），'
      + '同一个 IdP 账号从此再也建不出 Modeltaps 账号，必须人工清库。');
}

// ---------------------------------------------------------------- S7
// SEC-30 回归：提供方开「禁止首登自动建号」后，未关联到已有账号的首登必须被拒——
// 不建用户、不写身份行，回调响应带 OIDC_REGISTER_DISABLED: 前缀；关掉开关后恢复建号。
async function S7(browser) {
  await L.setEmailVerification(true);
  await L.setProvider({ link_by_verified_email: false, disable_auto_register: true });
  const login = mail('r1');
  const PREFIX = 'OIDC_REGISTER_DISABLED:';

  // 开关开：全新 AuthGear 账号首登。回调页只显示通用失败文案，判定文案在
  // /api/oauth/oidc 的响应体里，所以导航前就挂上响应监听（同 S6）。
  const { ctx, page } = await fresh(browser);
  const usersBefore = L.userCount();
  const identitiesBefore = L.allIdentities().length;
  const onRespP = page.waitForResponse((r) => r.url().includes('/api/oauth/oidc'), { timeout: 90000 })
    .then((r) => r.text()).catch(() => '');
  await L.openAuthgearFromModeltaps(page);
  const onFlow = await L.runFlow(page, { loginId: login, password: PW, otpTarget: login });
  const onRespText = (await onRespP).replace(/\s+/g, ' ').slice(0, 300);
  const onBody = await page.evaluate(() => document.body.innerText).catch(() => '');
  const onSelf = await modeltapsSelf(page);
  const usersAfterOn = L.userCount();
  const identitiesAfterOn = L.allIdentities().length;

  const rejected = onRespText.includes(PREFIX);
  const noWrite = usersAfterOn === usersBefore && identitiesAfterOn === identitiesBefore;
  const notLoggedIn = !(onSelf && onSelf.success);

  // 开关关：同一个 AuthGear 账号再登一次，应正常建号。
  await L.setProvider({ link_by_verified_email: false, disable_auto_register: false });
  await L.openAuthgearFromModeltaps(page, { logoutFirst: true });
  const offFlow = await L.runFlow(page, { loginId: login, password: PW, otpTarget: login });
  const offSelf = await modeltapsSelf(page);
  const usersAfterOff = L.userCount();
  const identitiesAfterOff = L.allIdentities().length;
  const created = offSelf.success ? L.userById(offSelf.data.id) : null;
  await ctx.close();

  const restored = !!created
    && usersAfterOff === usersAfterOn + 1
    && identitiesAfterOff === identitiesAfterOn + 1;
  const ok = rejected && noWrite && notLoggedIn && restored;
  record('S7', '禁止首登自动建号（SEC-30 回归）',
    '开关开：首登被拒、users 与身份行计数不变、响应含 ' + PREFIX + ' 前缀；开关关：恢复建号',
    {
      idp_login_id: login,
      on: {
        flow: onFlow, response_text: onRespText,
        page_text: onBody.replace(/\s+/g, ' ').slice(0, 200),
        logged_in: !notLoggedIn,
        users_before: usersBefore, users_after: usersAfterOn,
        identities_before: identitiesBefore, identities_after: identitiesAfterOn,
        rejected, no_write: noWrite,
      },
      off: {
        flow: offFlow, modeltaps_user: created,
        users_after: usersAfterOff, identities_after: identitiesAfterOff,
        identities: created ? L.identitiesOf(created.id) : [], restored,
      },
    },
    ok ? '符合' : '不符', ok ? '无' : '高',
    ok
      ? 'oidcRegisterOrReject 在 provider.DisableAutoRegister 时直接返回 OIDC_REGISTER_DISABLED: 前缀消息，'
      + '不进 oidcRegister，因此既不建用户也不写身份行（controller/oidc.go:378-394）；'
      + '关掉开关后同一 IdP 主体照常建号，开关无残留副作用。'
      : '开关未按预期生效：迁移期会继续产生无人认领的孤岛账号，或关掉开关后无法恢复建号，需立即复核。');
}

// ---------------------------------------------------------------- 主流程
const CASES = { S1, S2, S2b, S3, S4, S5, S6, S6b, S7 };

const browser = await chromium.launch({ headless: true });
try {
  await L.ensureSinkRedirect();
  for (const [id, fn] of Object.entries(CASES)) {
    if (ONLY && !ONLY.includes(id)) continue;
    console.log('\n=== ' + id + ' ===');
    try {
      await fn(browser);
    } catch (e) {
      record(id, '(执行失败)', '-', { error: e.message, stack: (e.stack || '').split('\n').slice(0, 4) },
        '执行失败', '待决策', '用例本身跑挂，需人工复核。');
    }
  }
} finally {
  await browser.close();
  await L.setEmailVerification(true).catch(() => {});
  await L.removeSinkRedirect().catch(() => {});
  await L.setProvider({ link_by_verified_email: false, disable_auto_register: false }).catch(() => {});
}

mkdirSync(path.dirname(OUT), { recursive: true });
writeFileSync(OUT, JSON.stringify({ run: RUN, at: new Date().toISOString(), results }, null, 2));
console.log('\n结果已写入 ' + OUT);
console.log(results.map((r) => [r.id, r.verdict, '风险' + r.risk, r.title].join('\t')).join('\n'));
