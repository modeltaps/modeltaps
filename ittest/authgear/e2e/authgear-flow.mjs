// AuthGear authflow v2 的屏幕驱动：不假设固定步数，每轮识别当前屏幕并执行对应动作，
// 直到浏览器被重定向回 Modeltaps。识别不了的屏幕直接 dump 结构后抛错，方便补分支。
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const execFileAsync = promisify(execFile);
const E2E_DIR = path.dirname(fileURLToPath(import.meta.url));
const AG_DIR = path.resolve(E2E_DIR, '..');

// otp.sh 从 AuthGear 容器日志（DEV_MODE）取最近一条发给该邮箱的验证码。
export async function fetchOtp(target) {
  const { stdout } = await execFileAsync(path.join(AG_DIR, 'otp.sh'), [target], {
    cwd: AG_DIR,
    env: { ...process.env, OTP_TIMEOUT: process.env.OTP_TIMEOUT || '25' },
  });
  const code = stdout.trim();
  if (!/^[0-9]{4,8}$/.test(code)) throw new Error(`otp.sh 返回异常: ${JSON.stringify(stdout)}`);
  return code;
}

export async function dumpScreen(page) {
  return page.evaluate(() => {
    const q = (s) => Array.from(document.querySelectorAll(s));
    return {
      url: location.href,
      inputs: q('input').map((e) => ({ name: e.name, type: e.type, placeholder: e.placeholder })),
      buttons: q('button,a[href]').map((e) => ({
        name: e.name, value: e.value, type: e.type,
        text: (e.textContent || '').replace(/\s+/g, ' ').trim().slice(0, 48),
      })).filter((b) => b.text),
      text: document.body.innerText.replace(/\n{2,}/g, '\n').slice(0, 600),
    };
  });
}

// settle 等待一次导航后的稳定态：authflow 每步都是整页提交。
async function settle(page) {
  await page.waitForLoadState('domcontentloaded');
  await page.waitForLoadState('networkidle').catch(() => {});
}

// act 执行一次会触发整页跳转的动作，并等到新文档就绪。
// 不等的话下一轮循环会读到旧 DOM，把同一屏重复提交一次（按钮此时已 disabled，必然超时）。
async function act(page, fn) {
  const nav = page.waitForNavigation({ waitUntil: 'domcontentloaded', timeout: 30000 }).catch(() => {});
  await fn();
  await nav;
}

// bindIdentity 在 AuthGear 的终端用户设置页给「当前已登录账号」再加绑一个邮箱，
// 走 /settings/identity/add_email → 填邮箱 → 收验证码 → 回列表页。
// 项目只有 email 一种 login_id，所以「同一账号多标识」就是同一账号多个邮箱。
export async function bindIdentity(page, authgearOrigin, loginId) {
  const listUrl = `${authgearOrigin}/settings/identity/email?q_login_id_key=email`;
  await page.goto(`${authgearOrigin}/settings/identity/add_email?q_login_id_key=email`,
    { waitUntil: 'domcontentloaded' });
  await settle(page);
  await page.locator('input[name="x_login_id"]').fill(loginId);
  await act(page, () => page.locator('button[name="x_action"][type="submit"]').first().click());
  await settle(page);

  for (let step = 0; step < 10; step++) {
    if (page.url().startsWith(listUrl.split('?')[0])) break;
    if (await page.locator('input[name="x_code"]').count()) {
      const code = await fetchOtp(loginId);
      await act(page, async () => {
        await page.locator('input[name="x_code"]').fill(code);
        await page.waitForTimeout(800);
        const submit = page.locator('button[name="x_action"][value="submit"]');
        if ((await submit.count()) && (await submit.first().isEnabled().catch(() => false))) {
          await submit.first().click().catch(() => {});
        }
      });
      await settle(page);
      continue;
    }
    break;
  }

  // 回列表页核对标识确实挂上了，避免「流程跑完但没绑成」被当成通过。
  await page.goto(listUrl, { waitUntil: 'domcontentloaded' });
  await settle(page);
  const shown = await page.locator('body').innerText();
  if (!shown.includes(loginId)) {
    throw Object.assign(new Error(`加绑邮箱后列表页未出现 ${loginId}`),
      { screen: await dumpScreen(page) });
  }
}

// driveAuthgear 走完 AuthGear 侧的注册或登录，直到回到 modeltapsBase。
//   opts: { loginId, otpTarget, password, trace }
// loginId 一律是邮箱（项目只有 email 一种 login_id）。
// password 为 null 表示这个账号只有 OTP 认证器：遇到需要创建密码的屏幕会抛错。
export async function driveAuthgear(page, modeltapsBase, opts) {
  const trace = opts.trace || [];
  let cooldownAt = null;
  for (let step = 0; step < 30; step++) {
    if (page.url().startsWith(modeltapsBase)) return trace;
    await settle(page);
    if (page.url().startsWith(modeltapsBase)) return trace;
    const url = new URL(page.url());
    const p = url.pathname;

    // 撞上 AuthGear 的验证码发送冷却时，页面会退回上一屏并挂一条提示，继续循环只会耗尽步数。
    // 只在同一屏重复出现时才判定：横幅可能是上一步的残留，首次见到不算。
    if ((await page.locator('body').innerText().catch(() => ''))
      .includes('Please wait for a moment before retrying')) {
      if (cooldownAt === p) {
        throw Object.assign(new Error('撞上 AuthGear 验证码发送冷却，请拉开用例间隔（--gap 毫秒）后重试'),
          { screen: await dumpScreen(page) });
      }
      cooldownAt = p;
    }

    // 入口屏：signup / login / signup_login 都是同一个「输入邮箱」表单，
    // 项目只有 email 一种 login_id，没有切换入口的链接。
    if (await page.locator('input[name="q_login_id"]').count()) {
      trace.push(`entry:${p}`);
      await page.locator('input[name="q_login_id"]').fill(opts.loginId);
      await act(page, () => page.locator('button[name="x_action"][value="login_id"]').click());
      continue;
    }

    if (p.endsWith('/create_password') || p.endsWith('/change_password')) {
      trace.push(`create_password:${p}`);
      if (!opts.password) {
        // 邮箱注册走到这一屏时，AuthGear 会附一个「下次用一次性验证码登录」的
        // 备选按钮；纯 OTP 用例走它，账号就不会有密码认证器。
        const otpOnly = page.locator('button:has-text("one-time password")');
        if (!(await otpOnly.count())) {
          throw Object.assign(new Error(`该用例要求无密码，但这一屏没有纯 OTP 备选：${p}`),
            { screen: await dumpScreen(page) });
        }
        trace.push('skip_password');
        await act(page, () => otpOnly.first().click());
        continue;
      }
      await page.locator('input[name="x_password"]').fill(opts.password);
      const confirm = page.locator('input[name="x_confirm_password"]');
      if (await confirm.count()) await confirm.fill(opts.password);
      await act(page, () => page.locator('button[name="x_action"][type="submit"]').first().click());
      continue;
    }

    if (p.endsWith('/enter_password')) {
      trace.push('enter_password');
      if (!opts.password) {
        // 纯 OTP 用例：这一屏底部有「Send code via Email」备选，切过去走验证码。
        const alt = page.locator('button:has-text("Send code")');
        if (!(await alt.count())) {
          throw Object.assign(new Error('该用例要求无密码，但这一屏没有验证码备选'),
            { screen: await dumpScreen(page) });
        }
        trace.push('switch_to_otp');
        await act(page, () => alt.first().click());
        continue;
      }
      await page.locator('input[name="x_password"]').fill(opts.password);
      await act(page, () => page.locator('button[name="x_action"][type="submit"]').first().click());
      continue;
    }

    if (p.endsWith('/enter_oob_otp') || p.endsWith('/verify_login_link')) {
      trace.push('enter_oob_otp');
      const code = await fetchOtp(opts.otpTarget);
      // 验证码输入框填满 6 位会由页面脚本自动提交，此时按钮已 disabled，不能再点。
      await act(page, async () => {
        await page.locator('input[name="x_code"]').fill(code);
        await page.waitForTimeout(800);
        const submit = page.locator('button[name="x_action"][value="submit"]');
        if ((await submit.count()) && (await submit.first().isEnabled().catch(() => false))) {
          await submit.first().click().catch(() => {});
        }
      });
      continue;
    }

    // 认证器 / 验证方式选择屏：按用例意图选密码或 OTP。
    const choice = page.locator('a[href*="create_authenticator"], button[name="x_action"][value="choose"]');
    if (p.includes('create_authenticator') || p.includes('select') || (await choice.count())) {
      trace.push(`choose:${p}`);
      const want = opts.password ? /password/i : /code|otp|verification/i;
      const link = page.getByRole('link').filter({ hasText: want }).first();
      if (await link.count()) { await act(page, () => link.click()); continue; }
    }

    // 授权同意屏。
    const allow = page.locator('button[name="x_action"][value="consent"], button:has-text("Allow")');
    if (await allow.count()) {
      trace.push('consent');
      await act(page, () => allow.first().click());
      continue;
    }

    throw Object.assign(new Error(`未识别的 AuthGear 屏幕: ${p}`), { screen: await dumpScreen(page) });
  }
  throw new Error('AuthGear 流程步数超限');
}
