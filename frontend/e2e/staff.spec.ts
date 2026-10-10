import { expect, test } from '@playwright/test';
import { bookTodaysLastSlot, loginViaUi, newPatient, registerViaApi, STAFF, tokenViaApi, ymdJst } from './helpers';

test.describe('受付スタッフ', () => {
  test('来院受付 → 呼出 → 診察開始 → 診察完了（予約患者）', async ({ page, request }) => {
    const p = newPatient('visit');
    await registerViaApi(request, p);
    const appt = await bookTodaysLastSlot(request, await tokenViaApi(request, p.email));
    test.skip(!appt, '本日の空き枠が無い時間帯（深夜）のためスキップ');

    await loginViaUi(page, STAFF);
    await expect(page.getByRole('heading', { level: 1, name: 'スタッフ画面' })).toBeVisible();

    // 当日の予約 → 来院受付
    await page.getByRole('button', { name: `${p.name}さんの来院受付` }).click();
    const row = page.getByRole('row').filter({ hasText: p.name });
    await expect(row.getByText('受付済み')).toBeVisible();

    // 待機列 → 呼出 → 診察開始 → 診察完了
    await page.getByRole('tab', { name: '待機列' }).click();
    const queueRow = page.getByRole('row').filter({ hasText: p.name });
    await expect(queueRow).toContainText('E2E テスト'); // 症状はスタッフにのみ見える
    await page.getByRole('button', { name: `${p.name}さんの呼出` }).click();
    await page.getByRole('button', { name: `${p.name}さんの診察開始` }).click();
    await page.getByRole('button', { name: `${p.name}さんの診察完了` }).click();
    await expect(queueRow).toHaveCount(0);

    // 当日の予約では「診察済み」になる
    await page.getByRole('tab', { name: '当日の予約' }).click();
    await expect(page.getByRole('row').filter({ hasText: p.name }).getByText('診察済み')).toBeVisible();
  });

  test('当日受付の代行 → 待機列に追加され、公開の待ち時間ボードに人数が反映される', async ({ page, browser }) => {
    await loginViaUi(page, STAFF);
    await page.getByRole('tab', { name: '当日受付（代行）' }).click();
    const name = `E2E電話${Date.now()}`;
    await page.getByLabel('氏名').fill(name);
    await page.getByRole('button', { name: '受付する' }).click();
    await expect(page.getByRole('status')).toContainText('受付番号');

    await page.getByRole('tab', { name: '待機列' }).click();
    const row = page.getByRole('row').filter({ hasText: name });
    await expect(row).toContainText('当日受付');
    await expect(row).toContainText('待機中');

    // 未ログインの別ブラウザから見ると、待っている人数が出るが氏名は出ない
    const anon = await browser.newPage();
    await anon.goto('/');
    await expect(async () => {
      await anon.reload();
      await expect(anon.getByText(/約\d+分/).first()).toBeVisible();
      await expect(anon.getByText(name)).toHaveCount(0);
    }).toPass({ timeout: 20_000 });
    await anon.close();

    // 後片付け: 呼出〜完了して列から外す
    for (const act of ['呼出', '診察開始', '診察完了']) await page.getByRole('button', { name: `${name}さんの${act}` }).click();
    await expect(row).toHaveCount(0);
  });

  test('患者の当日受付 → スタッフの待機列に表示される', async ({ page, browser, request }) => {
    const p = newPatient('walkin');
    await registerViaApi(request, p);
    const ctx = await browser.newContext();
    const patientPage = await ctx.newPage();
    await loginViaUi(patientPage, p.email);
    await patientPage.goto('/');
    await patientPage.getByRole('button', { name: '内科の当日受付をする' }).click();
    await patientPage.getByRole('dialog').getByRole('button', { name: '受付する' }).click();
    await expect(patientPage.getByRole('dialog')).toContainText('受付番号');
    await ctx.close();

    await loginViaUi(page, STAFF);
    await page.getByRole('tab', { name: '待機列' }).click();
    const row = page.getByRole('row').filter({ hasText: p.name });
    await expect(row).toContainText('当日受付');
    for (const act of ['呼出', '診察開始', '診察完了']) await page.getByRole('button', { name: `${p.name}さんの${act}` }).click();
  });

  test('休診を設定すると、その時間の枠は予約画面で「休診」と表示される', async ({ page }) => {
    await loginViaUi(page, STAFF);
    await page.getByRole('tab', { name: '休診設定' }).click();
    const day = ymdJst(2); // 明後日の 03:00〜04:00（他のテストが使わない時間帯）
    await page.getByLabel('開始').fill(`${day}T03:00`);
    await page.getByLabel('終了').fill(`${day}T04:00`);
    await page.getByRole('button', { name: '休診を設定する' }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toContainText('予約できなくなります');
    await dialog.getByRole('button', { name: '設定する' }).click();
    await expect(page.getByRole('status')).toContainText('枠を休診にし');

    await page.goto('/reserve');
    await page.getByLabel('診療科').selectOption({ label: '内科' });
    await page.getByLabel('日付').fill(day);
    await expect(page.getByRole('button', { name: /^03:00 山田 休診$/ })).toBeDisabled();
    await expect(page.getByRole('button', { name: /^04:00 山田$/ })).toBeEnabled();
  });

  test('スタッフは管理画面を開けない', async ({ page }) => {
    await loginViaUi(page, STAFF);
    await expect(page.getByRole('heading', { level: 1, name: 'スタッフ画面' })).toBeVisible();

    await page.goto('/admin');

    await expect(page.getByRole('alert')).toContainText('権限がありません');
  });
});
