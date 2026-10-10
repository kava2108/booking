import { expect, test } from '@playwright/test';

test.describe('公開画面（未ログイン）', () => {
  test('トップに診療科ごとの待ち時間が表示される', async ({ page }) => {
    await page.goto('/');

    await expect(page.getByRole('heading', { level: 1, name: '現在の待ち時間' })).toBeVisible();
    await expect(page.getByRole('heading', { name: '内科' })).toBeVisible();
    await expect(page.getByText('待ち時間は目安です').first()).toBeVisible();
    // 個人を特定できる情報は載らない
    await expect(page.getByText(/受付番号|E2E患者/)).toHaveCount(0);
  });

  test('予約画面で診療科を選ぶと、空き枠が表示される', async ({ page }) => {
    await page.goto('/reserve');
    await expect(page.getByText('診療科を選ぶと、空き枠が表示されます')).toBeVisible();

    await page.getByLabel('診療科').selectOption({ label: '内科' });
    await page.getByRole('button', { name: '次の日' }).click();

    await expect(page.getByRole('button', { name: /^\d\d:\d\d 山田$/ }).first()).toBeEnabled();
  });

  test('存在しないページは 404 の案内を表示する', async ({ page }) => {
    await page.goto('/no/such/page');

    await expect(page.getByRole('heading', { level: 1, name: 'ページが見つかりません' })).toBeVisible();
    await page.getByRole('link', { name: 'トップへ戻る' }).click();
    await expect(page).toHaveURL('/');
  });

  test('パスワード再設定の申請は、登録の有無にかかわらず同じ案内になる', async ({ page }) => {
    await page.goto('/password-reset');
    await page.getByLabel('メールアドレス').fill('nobody-at-all@e2e.test');
    await page.getByRole('button', { name: '再設定メールを送る' }).click();

    await expect(page.getByRole('status')).toContainText('登録されている場合');
  });

  test('スキップリンクとランドマークがある（アクセシビリティ）', async ({ page }) => {
    await page.goto('/');

    await expect(page.getByRole('banner')).toBeVisible();
    await expect(page.getByRole('main')).toBeVisible();
    await expect(page.getByRole('contentinfo')).toBeVisible();
    await page.keyboard.press('Tab');
    await expect(page.getByRole('link', { name: '本文へスキップ' })).toBeFocused();
  });
});
