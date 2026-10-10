import { screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { Route, Routes, MemoryRouter } from 'react-router-dom';
import { render } from '@testing-library/react';
import { authStore } from '@/store/auth';
import { loginAs } from '@/test/helpers';
import { RequireAuth, RequireRole } from './guards';

afterEach(() => authStore.reset());

function app(initial: string) {
  return render(
    <MemoryRouter initialEntries={[initial]}>
      <Routes>
        <Route path="/login" element={<div>ログイン画面</div>} />
        <Route element={<RequireAuth />}>
          <Route path="/mypage" element={<div>マイページ</div>} />
        </Route>
        <Route element={<RequireRole roles={['staff', 'clinic_admin']} />}>
          <Route path="/staff" element={<div>スタッフ画面</div>} />
        </Route>
        <Route element={<RequireRole roles={['clinic_admin']} />}>
          <Route path="/admin" element={<div>管理画面</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

describe('RequireAuth', () => {
  it('セッション復元中は何も表示せず待つ（誤ってログイン画面へ飛ばさない）', () => {
    app('/mypage');

    expect(screen.getByRole('status')).toHaveTextContent('読み込み中');
    expect(screen.queryByText('ログイン画面')).not.toBeInTheDocument();
  });

  it('未ログインならログイン画面へ', () => {
    authStore.clear();
    app('/mypage');

    expect(screen.getByText('ログイン画面')).toBeInTheDocument();
  });

  it('ログイン済みなら表示する', () => {
    loginAs('patient');
    app('/mypage');

    expect(screen.getByText('マイページ')).toBeInTheDocument();
  });
});

describe('RequireRole（画面の出し分け。認可の本体はサーバー）', () => {
  it.each([
    ['staff', '/staff', 'スタッフ画面'],
    ['clinic_admin', '/staff', 'スタッフ画面'],
    ['clinic_admin', '/admin', '管理画面'],
  ])('%s は %s を表示できる', (role, path, text) => {
    loginAs(role);
    app(path);

    expect(screen.getByText(text)).toBeInTheDocument();
  });

  it.each([
    ['patient', '/staff'],
    ['patient', '/admin'],
    ['staff', '/admin'],
    ['system_admin', '/staff'],
    ['system_admin', '/admin'],
  ])('%s は %s を開けず、権限なしの案内を表示する', (role, path) => {
    loginAs(role);
    app(path);

    expect(screen.getByRole('alert')).toHaveTextContent('権限がありません');
    expect(screen.queryByText('スタッフ画面')).not.toBeInTheDocument();
    expect(screen.queryByText('管理画面')).not.toBeInTheDocument();
  });

  it('未ログインならログイン画面へ', () => {
    authStore.clear();
    app('/staff');

    expect(screen.getByText('ログイン画面')).toBeInTheDocument();
  });
});
