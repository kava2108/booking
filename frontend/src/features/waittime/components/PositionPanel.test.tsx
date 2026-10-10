import { screen } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it } from 'vitest';
import { authStore } from '@/store/auth';
import { loginAs, renderWithProviders } from '@/test/helpers';
import { server } from '@/test/server';
import { PositionPanel } from './PositionPanel';

afterEach(() => authStore.reset());
const URL = '*/api/v1/appointments/a1/position';

describe('PositionPanel（REQ-004-AC-4 自分の順番）', () => {
  it('待機中は前の人数と呼出見込み時刻を表示する', async () => {
    loginAs('patient');
    server.use(http.get(URL, () => HttpResponse.json({ status: 'waiting', ahead_count: 4, estimated_call_at: '2026-10-12T09:50:00+09:00', basis: 'actual' })));
    renderWithProviders(<PositionPanel appointmentId="a1" />);

    expect(await screen.findByText(/あと4人/)).toBeInTheDocument();
    expect(screen.getByText(/09:50ごろ/)).toBeInTheDocument();
  });

  it('見込みを算出できないときは人数だけ伝え、時刻を推測しない', async () => {
    loginAs('patient');
    server.use(http.get(URL, () => HttpResponse.json({ status: 'waiting', ahead_count: 2, estimated_call_at: null })));
    renderWithProviders(<PositionPanel appointmentId="a1" />);

    expect(await screen.findByText(/あと2人/)).toBeInTheDocument();
    expect(screen.queryByText(/ごろ/)).not.toBeInTheDocument();
    expect(screen.getByText(/見込み時刻は算出できません/)).toBeInTheDocument();
  });

  it.each([
    ['not_checked_in', '来院されましたら受付で'],
    ['called', 'お呼びしています'],
    ['in_consultation', '診察中'],
    ['done', '診察は終了'],
    ['canceled', '取り消'],
  ])('状態 %s の案内', async (status, expected) => {
    loginAs('patient');
    server.use(http.get(URL, () => HttpResponse.json({ status, ahead_count: 0, estimated_call_at: null })));
    renderWithProviders(<PositionPanel appointmentId="a1" />);

    expect(await screen.findByText(new RegExp(expected))).toBeInTheDocument();
  });

  it('取得に失敗しても画面全体を壊さず、静かに案内する', async () => {
    loginAs('patient');
    server.use(http.get(URL, () => HttpResponse.json({ error: 'internal_error' }, { status: 500 })));
    renderWithProviders(<PositionPanel appointmentId="a1" />);

    expect(await screen.findByText(/順番を取得できませんでした/)).toBeInTheDocument();
  });
});
