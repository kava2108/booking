import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { DepartmentWaitTime } from '@/api/types';
import { available, unavailable } from '@/test/helpers';
import { WaitTimeCard } from './WaitTimeCard';

const show = (d: unknown) => render(<WaitTimeCard dept={d as DepartmentWaitTime} />);

describe('WaitTimeCard', () => {
  // REQ-004-AC-1: 待ち人数と推定待ち時間
  it('待ち時間（分）と待ち人数を表示する', () => {
    show(available({ waiting_count: 6, wait_minutes: 35 }));

    expect(screen.getByRole('heading', { name: '内科' })).toBeInTheDocument();
    expect(screen.getByText('約35分')).toBeInTheDocument();
    expect(screen.getByText('6人')).toBeInTheDocument();
  });

  it('根拠: 実績ベースと標準時間ベースを区別して伝える', () => {
    const { unmount } = show(available({ basis: 'actual' }));
    expect(screen.getByText(/直近の診察実績/)).toBeInTheDocument();
    unmount();

    show(available({ basis: 'standard' }));
    expect(screen.getByText(/標準的な診察時間/)).toBeInTheDocument();
  });

  it('待っている人がいなければ「お待ちの方はいません」', () => {
    show(available({ waiting_count: 0, wait_minutes: 0, basis: undefined }));

    expect(screen.getByText('現在お待ちの方はいません')).toBeInTheDocument();
    expect(screen.queryByText(/約0分/)).not.toBeInTheDocument();
  });

  // REQ-004-AC-5: 算出不可は理由を表示し、数値を出さない
  it.each([
    ['before_hours', '診療開始前'],
    ['closed', '休診または診療時間外'],
    ['no_data', '算出できません'],
  ])('算出不可（%s）は理由を表示し、数値を一切出さない', (reason, expected) => {
    show(unavailable(reason));

    expect(screen.getByText(new RegExp(expected))).toBeInTheDocument();
    expect(screen.queryByText(/約\d+分/)).not.toBeInTheDocument();
    expect(screen.queryByText(/\d+人/)).not.toBeInTheDocument();
  });

  it('未知の理由でも落ちず、数値は出さない', () => {
    show(unavailable('something_new'));

    expect(screen.getByText(/現在はご利用いただけません|算出できません/)).toBeInTheDocument();
    expect(screen.queryByText(/約\d+分/)).not.toBeInTheDocument();
  });

  it('1 時間以上は「約1時間30分」のように読みやすく表示する', () => {
    show(available({ wait_minutes: 90 }));

    expect(screen.getByText('約1時間30分')).toBeInTheDocument();
  });

  it('ちょうど 60 分は「約1時間」', () => {
    show(available({ wait_minutes: 60 }));

    expect(screen.getByText('約1時間')).toBeInTheDocument();
  });
});
