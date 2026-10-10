import { expect, test } from '@playwright/test';
import { loginViaUi, newPatient, PASSWORD, registerViaApi } from './helpers';

test.describe('患者', () => {
  test('新規登録 → 予約 → マイページで確認 → キャンセル', async ({ page }) => {
    const p = newPatient('flow');

    // 新規登録（登録後は自動でログインしてマイページへ）
    await page.goto('/register');
    await page.getByLabel('お名前').fill(p.name);
    await page.getByLabel('メールアドレス').fill(p.email);
    await page.getByLabel('パスワード（12文字以上）').fill(PASSWORD);
    await page.getByLabel('パスワード（確認）').fill(PASSWORD);
    await page.getByRole('button', { name: '登録する' }).click();
    await expect(page.getByRole('heading', { level: 1, name: 'マイページ' })).toBeVisible();
    await expect(page.getByText('ご予約はありません')).toBeVisible();

    // 予約（明日の枠。深夜でも必ず枠がある）
    await page.getByRole('link', { name: '新しく予約する' }).click();
    await page.getByLabel('診療科').selectOption({ label: '内科' });
    await page.getByRole('button', { name: '次の日' }).click();
    await page.getByRole('button', { name: /^09:00 山田$/ }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toContainText('内科');
    await dialog.getByLabel('再診').check();
    await dialog.getByLabel(/症状など/).fill('E2E: 頭痛');
    await dialog.getByRole('button', { name: '予約を確定する' }).click();
    await expect(dialog.getByText('予約を受け付けました')).toBeVisible();
    const no = (await dialog.locator('strong').last().textContent())?.trim() ?? '';
    expect(no).toMatch(/^A-\d{8}-\d{4}$/);

    // マイページで確認
    await dialog.getByRole('link', { name: 'マイページで確認する' }).click();
    const card = page.getByRole('article').filter({ hasText: no });
    await expect(card).toContainText('予約済み');
    await expect(card).toContainText('09:00');
    await expect(card).toContainText('E2E: 頭痛');

    // キャンセル（確認ダイアログを経る）
    await card.getByRole('button', { name: 'キャンセルする' }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'キャンセルを確定する' }).click();
    await expect(page.getByRole('region', { name: 'これまでのご予約' }).getByText('キャンセル済み')).toBeVisible();
  });

  test('同じ時間帯の二重予約は拒否され、別の枠を選べる', async ({ page, request }) => {
    const p = newPatient('dup');
    await registerViaApi(request, p);
    await loginViaUi(page, p.email);
    await expect(page.getByRole('heading', { level: 1, name: 'マイページ' })).toBeVisible();

    const book = async () => {
      await page.goto('/reserve');
      await page.getByLabel('診療科').selectOption({ label: '内科' });
      await page.getByRole('button', { name: '次の日' }).click();
      await page.getByRole('button', { name: /^10:00 山田$/ }).click();
      await page.getByRole('dialog').getByLabel('初診').check();
      await page.getByRole('dialog').getByRole('button', { name: '予約を確定する' }).click();
    };
    await book();
    await expect(page.getByText('予約を受け付けました')).toBeVisible();
    await book();

    await expect(page.getByRole('dialog').getByRole('alert')).toContainText('同じ時間帯');
  });

  test('未ログインでマイページを開くとログイン画面へ。ログインすると元のページへ戻る', async ({ page, request }) => {
    const p = newPatient('redirect');
    await registerViaApi(request, p);

    await page.goto('/mypage');
    await expect(page.getByRole('heading', { level: 1, name: 'ログイン' })).toBeVisible();
    await page.getByLabel('メールアドレス').fill(p.email);
    await page.getByLabel('パスワード', { exact: true }).fill(PASSWORD);
    await page.getByRole('button', { name: 'ログイン' }).click();

    await expect(page).toHaveURL('/mypage');
    await expect(page.getByRole('heading', { level: 1, name: 'マイページ' })).toBeVisible();
  });

  test('パスワードが違うとエラーを表示し、ログインしない', async ({ page, request }) => {
    const p = newPatient('wrongpw');
    await registerViaApi(request, p);

    await loginViaUi(page, p.email, 'Wrong-Password-999!', false);

    await expect(page.getByRole('alert')).toContainText('メールアドレスまたはパスワードが正しくありません');
    await expect(page.getByRole('heading', { level: 1, name: 'ログイン' })).toBeVisible();
  });

  test('ページを再読み込みしてもログイン状態が保たれる（Cookie からのセッション復元）', async ({ page, request }) => {
    const p = newPatient('reload');
    await registerViaApi(request, p);
    await loginViaUi(page, p.email);
    await expect(page.getByRole('heading', { level: 1, name: 'マイページ' })).toBeVisible();

    await page.reload();

    await expect(page.getByRole('heading', { level: 1, name: 'マイページ' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'ログアウト' })).toBeVisible();
  });

  test('ログアウトすると、再読み込みしてもログインしていない', async ({ page, request }) => {
    const p = newPatient('logout');
    await registerViaApi(request, p);
    await loginViaUi(page, p.email);
    await page.getByRole('button', { name: 'ログアウト' }).click();
    await expect(page.getByRole('link', { name: 'ログイン' })).toBeVisible();

    await page.goto('/mypage');

    await expect(page.getByRole('heading', { level: 1, name: 'ログイン' })).toBeVisible();
  });

  test('患者はスタッフ画面・管理画面を開けない', async ({ page, request }) => {
    const p = newPatient('authz');
    await registerViaApi(request, p);
    await loginViaUi(page, p.email);
    await expect(page.getByRole('heading', { level: 1, name: 'マイページ' })).toBeVisible();

    for (const path of ['/staff', '/admin']) {
      await page.goto(path);
      await expect(page.getByRole('alert')).toContainText('権限がありません');
    }
    await expect(page.getByRole('link', { name: 'スタッフ' })).toHaveCount(0);
  });

  test('他人の予約の順番は API でも見られない（IDOR）', async ({ request }) => {
    const a = newPatient('idor-a');
    const b = newPatient('idor-b');
    await registerViaApi(request, a);
    await registerViaApi(request, b);
    const tokenB = (await (await request.post('/api/v1/auth/login', { data: { email: b.email, password: PASSWORD } })).json()) as { access_token: string };

    const res = await request.get('/api/v1/appointments/00000000-0000-0000-0000-000000000000/position', { headers: { Authorization: `Bearer ${tokenB.access_token}` } });

    expect(res.status()).toBe(404);
  });
});
