import { expect, type APIRequestContext, type Page } from '@playwright/test';

/** scripts/e2e.sh が用意する固定アカウントと、テストごとに作る患者アカウントのパスワード。 */
export const PASSWORD = 'E2e-Password-123!';
export const ADMIN = 'admin@e2e.test';
export const STAFF = 'staff@e2e.test';

let seq = 0;
/** テストごとに別の患者アカウントを作る（共有 DB で互いに干渉しないように）。 */
export function newPatient(label: string): { email: string; name: string } {
  const id = `${Date.now()}-${++seq}-${Math.floor(Math.random() * 1e6)}`;
  return { email: `${label}-${id}@e2e.test`, name: `E2E患者${label}${seq}${Math.floor(Math.random() * 1000)}` };
}

export async function registerViaApi(request: APIRequestContext, p: { email: string; name: string }): Promise<void> {
  const res = await request.post('/api/v1/auth/register', { data: { email: p.email, password: PASSWORD, name: p.name } });
  expect(res.status(), await res.text()).toBe(201);
}

export async function tokenViaApi(request: APIRequestContext, email: string): Promise<string> {
  const res = await request.post('/api/v1/auth/login', { data: { email, password: PASSWORD } });
  expect(res.status(), await res.text()).toBe(200);
  return ((await res.json()) as { access_token: string }).access_token;
}

/** `expectSuccess: false` は、ログインに失敗する場合用（画面遷移を待たない）。 */
export async function loginViaUi(page: Page, email: string, password = PASSWORD, expectSuccess = true): Promise<void> {
  await page.goto('/login');
  await page.getByLabel('メールアドレス').fill(email);
  await page.getByLabel('パスワード', { exact: true }).fill(password);
  await page.getByRole('button', { name: 'ログイン' }).click();
  if (!expectSuccess) return;
  // ログイン完了（画面遷移）を待つ。待たずに次の goto をすると、ログイン要求が中断される
  await page.waitForURL((url) => url.pathname !== '/login');
}

/** JST の "YYYY-MM-DD"。 */
export function ymdJst(offsetDays = 0): string {
  const d = new Date(Date.now() + offsetDays * 86_400_000);
  return new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Tokyo', year: 'numeric', month: '2-digit', day: '2-digit' }).format(d);
}

interface Slot { id: string; start_at: string; available: boolean }

/** 今日の空いている枠のうち最後のものを、API で予約する。今日の枠が残っていなければ null（深夜帯）。 */
export async function bookTodaysLastSlot(request: APIRequestContext, token: string): Promise<{ id: string; appointment_no: string } | null> {
  const slots = (await (await request.get(`/api/v1/slots?department_id=1&date=${ymdJst()}`)).json()) as Slot[];
  const free = slots.filter((s) => s.available);
  const last = free.at(-1);
  if (!last) return null;
  const res = await request.post('/api/v1/appointments', {
    headers: { Authorization: `Bearer ${token}` },
    data: { slot_id: last.id, visit_type: 'first', symptom_note: 'E2E テスト' },
  });
  expect(res.status(), await res.text()).toBe(201);
  return (await res.json()) as { id: string; appointment_no: string };
}
