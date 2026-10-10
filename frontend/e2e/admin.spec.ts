import { expect, test } from '@playwright/test';
import { ADMIN, loginViaUi } from './helpers';

test.describe('クリニック管理者', () => {
  test('基本設定を保存すると、再読み込みしても反映されている', async ({ page }) => {
    await loginViaUi(page, ADMIN);
    await page.goto('/admin');
    await page.getByRole('tab', { name: '基本設定' }).click();

    const deadline = page.getByLabel(/キャンセル期限/);
    await deadline.fill('90');
    await page.getByLabel('内科の当日受付の定員（人）').fill('50');
    await page.getByRole('button', { name: '保存する' }).click();
    await expect(page.getByRole('status')).toContainText('保存しました');

    await page.reload();
    await page.getByRole('tab', { name: '基本設定' }).click();
    await expect(page.getByLabel(/キャンセル期限/)).toHaveValue('90');
    await expect(page.getByLabel('内科の当日受付の定員（人）')).toHaveValue('50');
  });

  test('範囲外の値は保存前にエラーになる', async ({ page }) => {
    await loginViaUi(page, ADMIN);
    await page.goto('/admin');
    await page.getByRole('tab', { name: '基本設定' }).click();

    await page.getByLabel(/予約受付期間/).fill('0');
    await page.getByRole('button', { name: '保存する' }).click();

    await expect(page.getByText('1以上365以下で入力してください')).toBeVisible();
  });

  test('診療スケジュールを表示し、時間帯を追加して保存できる', async ({ page }) => {
    await loginViaUi(page, ADMIN);
    await page.goto('/admin');

    await page.getByLabel('診療科').selectOption({ label: '内科' });
    await page.getByLabel('医師').selectOption({ label: '山田' });
    await expect(page.getByLabel('曜日（7行目）')).toBeVisible(); // seed: 全曜日 7 行
    await page.getByRole('button', { name: '保存する' }).click();

    await expect(page.getByRole('status')).toContainText('保存しました');
  });

  test('診療時間が重なる入力は保存前にエラーになる', async ({ page }) => {
    await loginViaUi(page, ADMIN);
    await page.goto('/admin');
    await page.getByLabel('診療科').selectOption({ label: '内科' });
    await page.getByLabel('医師').selectOption({ label: '山田' });
    await page.getByRole('button', { name: '時間帯を追加' }).click();

    // 追加した 8 行目を月曜 09:00〜12:00 にする → 1 行目(月曜 00:00-23:30 の行)があるため重複
    await page.getByLabel('曜日（8行目）').selectOption('1');
    await page.getByRole('button', { name: '保存する' }).click();

    await expect(page.getByText('同じ曜日の他の時間帯と重なっています')).toBeVisible();
  });
});
