// 紹介サイトの画面集を撮影する。開発用の画面(BASE)に、scripts/demo-seed.sh のデモデータが入っている前提。
//   scripts/screenshots.sh から Playwright のコンテナ内で実行する（cwd は frontend/）。
import { createRequire } from 'node:module';
const { chromium } = createRequire(`${process.cwd()}/`)('@playwright/test');

const BASE = process.env.SHOT_BASE ?? 'http://127.0.0.1:5173';
const OUT = process.env.SHOT_OUT ?? '../site/img';
const PASSWORD = 'Dev-Password-123!';

const browser = await chromium.launch({ args: ['--lang=ja-JP'] });

async function session(email, viewport = { width: 1280, height: 800 }) {
  const ctx = await browser.newContext({ viewport, locale: 'ja-JP', timezoneId: 'Asia/Tokyo', deviceScaleFactor: 2 });
  const page = await ctx.newPage();
  if (email) {
    await page.goto(`${BASE}/login`);
    await page.getByLabel('メールアドレス').fill(email);
    await page.getByLabel('パスワード', { exact: true }).fill(PASSWORD);
    await page.getByRole('button', { name: 'ログイン' }).click();
    await page.waitForURL((u) => u.pathname !== '/login');
  }
  return { ctx, page };
}

const shot = async (page, name, opts = {}) => {
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(400);
  await page.screenshot({ path: `${OUT}/${name}.png`, ...opts });
  console.log('shot', name);
};

// 1. 公開の待ち時間ボード（未ログイン）とログイン画面
{
  const { ctx, page } = await session(null);
  await page.goto(`${BASE}/`);
  await page.getByRole('heading', { level: 1, name: '現在の待ち時間' }).waitFor();
  await shot(page, 'home');
  await page.goto(`${BASE}/login`);
  await shot(page, 'login');
  await ctx.close();
}

// 2. 患者: 予約 → ダイアログ → マイページ
{
  const { ctx, page } = await session('demo3@dev.test');
  await page.goto(`${BASE}/reserve`);
  await page.getByLabel('診療科').selectOption({ label: '内科' });
  await page.getByRole('button', { name: '次の日' }).click();
  await page.getByRole('button', { name: /^09:00 / }).waitFor();
  await shot(page, 'reserve', { fullPage: true });
  await page.getByRole('button', { name: /^11:00 / }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel('再診').check();
  await dialog.getByLabel(/症状など/).fill('高血圧の薬をもらいたい');
  await shot(page, 'booking-dialog');
  await ctx.close();
}
{
  const { ctx, page } = await session('demo9@dev.test'); // 今日の内科で待機中
  await page.goto(`${BASE}/mypage`);
  await page.getByRole('heading', { level: 1, name: 'マイページ' }).waitFor();
  await shot(page, 'mypage', { fullPage: true });
  await ctx.close();
}

// 3. スタッフ
{
  const { ctx, page } = await session('staff@dev.test');
  await page.getByRole('heading', { level: 1, name: 'スタッフ画面' }).waitFor();
  await page.getByRole('tab', { name: '当日の予約' }).click();
  await shot(page, 'staff-appointments', { fullPage: true });
  await page.getByRole('tab', { name: '待機列' }).click();
  await shot(page, 'staff-queue', { fullPage: true });
  await page.getByRole('tab', { name: '当日受付（代行）' }).click();
  await shot(page, 'staff-walkin');
  await page.getByRole('tab', { name: '休診設定' }).click();
  await shot(page, 'staff-closure');
  await ctx.close();
}

// 4. 管理者
{
  const { ctx, page } = await session('admin@dev.test', { width: 1280, height: 1100 });
  await page.goto(`${BASE}/admin`);
  await page.getByLabel('診療科').selectOption({ label: '内科' });
  await page.getByLabel('医師').selectOption({ label: '山田 健一' });
  await page.getByLabel('曜日（1行目）').waitFor();
  await shot(page, 'admin-schedule');
  await page.getByRole('tab', { name: '基本設定' }).click();
  await shot(page, 'admin-settings', { fullPage: true });
  await ctx.close();
}

// 5. スマートフォン幅
{
  const { ctx, page } = await session(null, { width: 390, height: 844 });
  await page.goto(`${BASE}/`);
  await page.getByRole('heading', { level: 1, name: '現在の待ち時間' }).waitFor();
  await shot(page, 'mobile-home', { fullPage: true });
  await ctx.close();
}
{
  const { ctx, page } = await session('demo9@dev.test', { width: 390, height: 844 });
  await page.goto(`${BASE}/mypage`);
  await page.getByRole('heading', { level: 1, name: 'マイページ' }).waitFor();
  await shot(page, 'mobile-mypage', { fullPage: true });
  await ctx.close();
}

await browser.close();
