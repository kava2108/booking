import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it } from 'vitest';
import { authStore } from '@/store/auth';
import { dept, loginAs, queueEntry, renderWithProviders, staffAppt } from '@/test/helpers';
import { server } from '@/test/server';
import { StaffPage } from './StaffPage';

const BASE = '*/api/v1';
afterEach(() => authStore.reset());

function setup() {
  loginAs('staff');
  const queries: string[] = [];
  server.use(
    http.get(`${BASE}/departments`, () => HttpResponse.json([dept(1, '内科'), dept(2, '小児科')])),
    http.get(`${BASE}/doctors`, () => HttpResponse.json([])),
    http.get(`${BASE}/staff/appointments`, ({ request }) => { queries.push(`appt:${new URL(request.url).searchParams.get('department_id')}`); return HttpResponse.json([staffAppt('a1')]); }),
    http.get(`${BASE}/staff/queue`, ({ request }) => { queries.push(`queue:${new URL(request.url).searchParams.get('department_id')}`); return HttpResponse.json([queueEntry('q1')]); }),
  );
  renderWithProviders(<StaffPage />);
  return queries;
}

describe('StaffPage', () => {
  it('最初の診療科の「当日の予約」を表示する', async () => {
    const queries = setup();

    expect(await screen.findByRole('tab', { name: '当日の予約', selected: true })).toBeInTheDocument();
    expect(await screen.findByText('山田 太郎')).toBeInTheDocument();
    expect(queries).toContain('appt:1');
  });

  it('タブで待機列・当日受付・休診設定に切り替えられる', async () => {
    setup();
    const user = userEvent.setup();
    await screen.findByText('山田 太郎');

    await user.click(screen.getByRole('tab', { name: '待機列' }));
    expect(await screen.findByRole('button', { name: /呼出/ })).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: '当日受付（代行）' }));
    expect(screen.getByLabelText('氏名')).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: '休診設定' }));
    expect(screen.getByRole('button', { name: '休診を設定する' })).toBeInTheDocument();
  });

  it('診療科を切り替えると、その診療科のデータを取得する', async () => {
    const queries = setup();
    const user = userEvent.setup();
    await screen.findByText('山田 太郎');

    await user.selectOptions(screen.getByLabelText('診療科'), '小児科');

    await screen.findByText('山田 太郎');
    expect(queries).toContain('appt:2');
  });

  it('矢印キーでタブを切り替えられる（キーボード操作）', async () => {
    setup();
    const user = userEvent.setup();
    const first = await screen.findByRole('tab', { name: '当日の予約' });

    first.focus();
    await user.keyboard('{ArrowRight}');

    expect(screen.getByRole('tab', { name: '待機列', selected: true })).toHaveFocus();
  });
});
