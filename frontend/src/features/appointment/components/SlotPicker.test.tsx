import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { describe, expect, it, vi } from 'vitest';
import type { Doctor } from '@/api/types';
import { doctor, renderWithProviders, slot } from '@/test/helpers';
import { server } from '@/test/server';
import { SlotPicker } from './SlotPicker';

const SLOTS_URL = '*/api/v1/slots';
const doctors = [doctor(7, '山田'), doctor(8, '佐藤')] as Doctor[];

describe('SlotPicker（REQ-001 空き枠）', () => {
  it('空き枠を時刻順に表示し、選択すると通知する', async () => {
    server.use(http.get(SLOTS_URL, () => HttpResponse.json([slot('s1', '2026-10-19T09:30:00+09:00'), slot('s2', '2026-10-19T10:00:00+09:00')])));
    const onSelect = vi.fn();
    renderWithProviders(<SlotPicker departmentId={1} date="2026-10-19" doctors={doctors} onSelect={onSelect} />);

    const buttons = await screen.findAllByRole('button', { name: /山田/ });
    expect(buttons).toHaveLength(2);
    expect(buttons[0]).toHaveTextContent('09:30');
    await userEvent.setup().click(buttons[1]!);

    expect(onSelect).toHaveBeenCalledWith(expect.objectContaining({ id: 's2' }));
  });

  // REQ-001-AC-3: 満席・休診・締切後は選択不可として理由つきで表示する
  it.each([
    ['full', '満席'],
    ['closed', '休診'],
    ['past_cutoff', '受付終了'],
    ['outside_window', '予約受付期間外'],
  ])('%s の枠は選択できず、理由「%s」を表示する', async (reason, label) => {
    server.use(http.get(SLOTS_URL, () => HttpResponse.json([slot('s1', '2026-10-19T09:30:00+09:00', { available: false, unavailable_reason: reason })])));
    const onSelect = vi.fn();
    renderWithProviders(<SlotPicker departmentId={1} date="2026-10-19" doctors={doctors} onSelect={onSelect} />);

    const btn = await screen.findByRole('button', { name: new RegExp(`09:30.*${label}`) });
    expect(btn).toBeDisabled();
    await userEvent.setup().click(btn);
    expect(onSelect).not.toHaveBeenCalled();
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it('医師ごとにまとめて表示する', async () => {
    server.use(http.get(SLOTS_URL, () => HttpResponse.json([
      slot('s1', '2026-10-19T09:30:00+09:00', { doctor_id: 7 }),
      slot('s2', '2026-10-19T09:30:00+09:00', { doctor_id: 8 }),
    ])));
    renderWithProviders(<SlotPicker departmentId={1} date="2026-10-19" doctors={doctors} onSelect={() => {}} />);

    expect(await screen.findByRole('heading', { name: '山田 医師' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '佐藤 医師' })).toBeInTheDocument();
  });

  it('日付・診療科・医師の条件を API に渡す', async () => {
    let query = '';
    server.use(http.get(SLOTS_URL, ({ request }) => { query = new URL(request.url).search; return HttpResponse.json([]); }));
    renderWithProviders(<SlotPicker departmentId={3} doctorId={8} date="2026-10-19" doctors={doctors} onSelect={() => {}} />);

    await screen.findByText(/予約できる枠がありません/);
    const q = new URLSearchParams(query);
    expect(q.get('department_id')).toBe('3');
    expect(q.get('doctor_id')).toBe('8');
    expect(q.get('date')).toBe('2026-10-19');
  });

  it('枠が無い日は案内を表示する', async () => {
    server.use(http.get(SLOTS_URL, () => HttpResponse.json([])));
    renderWithProviders(<SlotPicker departmentId={1} date="2026-10-19" doctors={doctors} onSelect={() => {}} />);

    expect(await screen.findByText('この日は予約できる枠がありません')).toBeInTheDocument();
  });

  it('すべて選択不可の日は、別の日を選ぶよう案内する', async () => {
    server.use(http.get(SLOTS_URL, () => HttpResponse.json([slot('s1', '2026-10-19T09:30:00+09:00', { available: false, unavailable_reason: 'full' })])));
    renderWithProviders(<SlotPicker departmentId={1} date="2026-10-19" doctors={doctors} onSelect={() => {}} />);

    expect(await screen.findByText(/別の日をお選びください/)).toBeInTheDocument();
  });

  it('取得に失敗したらエラーと再試行を表示する', async () => {
    let n = 0;
    server.use(http.get(SLOTS_URL, () => (n++ === 0 ? HttpResponse.json({ error: 'internal_error', request_id: 'req-1' }, { status: 500 }) : HttpResponse.json([slot('s1', '2026-10-19T09:30:00+09:00')]))));
    renderWithProviders(<SlotPicker departmentId={1} date="2026-10-19" doctors={doctors} onSelect={() => {}} />);

    expect(await screen.findByRole('alert')).toHaveTextContent('req-1');
    await userEvent.setup().click(screen.getByRole('button', { name: '再試行' }));

    expect(await screen.findByRole('button', { name: /09:30/ })).toBeInTheDocument();
  });

  it('時刻は JST で表示する（ブラウザのタイムゾーンに依存しない）', async () => {
    server.use(http.get(SLOTS_URL, () => HttpResponse.json([slot('s1', '2026-10-19T00:30:00Z')])));
    renderWithProviders(<SlotPicker departmentId={1} date="2026-10-19" doctors={doctors} onSelect={() => {}} />);

    expect(await screen.findByRole('button', { name: /09:30/ })).toBeInTheDocument();
  });
});
