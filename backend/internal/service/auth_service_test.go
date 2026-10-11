package service_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/kava2108/booking/backend/internal/auth"
	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/mocks"
	"github.com/kava2108/booking/backend/internal/service"
)

// IMP 8-user-auth-auth-service-register-login TASK-0001 / IMP 9-user-auth-auth-service-refresh-reset TASK-0001

const (
	jwtSecret = "0123456789abcdef0123456789abcdef"
	goodPass  = "SecurePass123!" // 14 文字
)

var (
	now    = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	userID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
)

// fakeMailer は送信内容を記録する。トークンの中身は検証用に保持する。
type fakeMailer struct {
	mu            sync.Mutex
	registrations []string
	resets        []resetMail
	err           error
}
type resetMail struct{ to, name, url string }

func (f *fakeMailer) SendRegistrationConfirmation(_ context.Context, to, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registrations = append(f.registrations, to)
	return f.err
}
func (f *fakeMailer) SendPasswordReset(_ context.Context, to, name, url string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resets = append(f.resets, resetMail{to, name, url})
	return f.err
}

type env struct {
	svc    service.AuthService
	users  *mocks.MockUserRepository
	tokens *mocks.MockTokenRepository
	mail   *fakeMailer
	tm     *auth.TokenManager
	clock  *time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	clock := now
	tm, err := auth.NewTokenManager(jwtSecret, 15*time.Minute, auth.WithClock(func() time.Time { return clock }))
	require.NoError(t, err)
	e := &env{users: mocks.NewMockUserRepository(t), tokens: mocks.NewMockTokenRepository(t), mail: &fakeMailer{}, tm: tm, clock: &clock}
	e.svc = service.NewAuthService(e.users, e.tokens, tm, e.mail,
		service.AuthConfig{RefreshTTL: 7 * 24 * time.Hour, ResetTTL: 30 * time.Minute, BcryptCost: bcrypt.MinCost, ResetURLBase: "https://app.example.com",
			LoginMaxFailures: 5, LoginLockWindow: 15 * time.Minute, ResetMaxRequests: 3, ResetWindow: time.Hour},
		service.WithClock(func() time.Time { return clock }),
		service.WithAsync(func(f func()) { f() }), // テストでは同期実行
		service.WithLogger(zerolog.Nop()),
	)
	return e
}

func userWithPassword(t *testing.T, pw string) *domain.User {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	require.NoError(t, err)
	return &domain.User{ID: userID, Email: "user@example.com", PasswordHash: string(h), Name: "山田 太郎", Role: domain.RolePatient}
}

// ---------- Register (REQ-001) ----------

func TestRegister_Success(t *testing.T) {
	e := newEnv(t)
	var saved repository.CreateUserParams
	e.users.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p repository.CreateUserParams) (*domain.User, error) {
			saved = p
			return &domain.User{ID: userID, Email: p.Email, Name: p.Name, Role: domain.RolePatient}, nil
		})

	u, err := e.svc.Register(context.Background(), service.RegisterInput{Email: "  User@Example.COM ", Password: goodPass, Name: "山田 太郎"})
	require.NoError(t, err)

	assert.Equal(t, "user@example.com", saved.Email, "trim + 小文字化して保存")
	assert.NotEqual(t, goodPass, saved.PasswordHash)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(saved.PasswordHash), []byte(goodPass)), "bcrypt で保存")
	assert.Equal(t, userID, u.ID)
	assert.Equal(t, []string{"user@example.com"}, e.mail.registrations, "確認メールを送る")
}

// REQ-001-AC-2
func TestRegister_DuplicateEmail(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, repository.ErrEmailTaken)

	_, err := e.svc.Register(context.Background(), service.RegisterInput{Email: "dup@example.com", Password: goodPass, Name: "n"})
	assert.ErrorIs(t, err, service.ErrEmailAlreadyExists)
	assert.Empty(t, e.mail.registrations, "登録失敗時はメールを送らない")
}

// REQ-001-AC-3 / 境界値: 12 文字未満は拒否、12 文字は可（文字数は rune 数）
func TestRegister_PasswordLengthBoundary(t *testing.T) {
	cases := map[string]struct {
		pw      string
		wantErr bool
	}{
		"11 文字":     {"abcdefghijk", true},
		"12 文字":     {"abcdefghijkl", false},
		"日本語 11 文字": {strings.Repeat("あ", 11), true},
		"日本語 12 文字": {strings.Repeat("あ", 12), false}, // 36 bytes
		"72 bytes":  {strings.Repeat("a", 72), false},
		"73 bytes":  {strings.Repeat("a", 73), true}, // bcrypt の上限超過
		"空":         {"", true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			if !c.wantErr {
				e.users.EXPECT().Create(mock.Anything, mock.Anything).Return(&domain.User{ID: userID}, nil)
			}
			_, err := e.svc.Register(context.Background(), service.RegisterInput{Email: "a@example.com", Password: c.pw, Name: "n"})
			if c.wantErr {
				assert.ErrorIs(t, err, service.ErrWeakPassword)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// メール送信に失敗しても登録は成功する（非同期・可用性優先）
func TestRegister_MailFailureDoesNotFailRegistration(t *testing.T) {
	e := newEnv(t)
	e.mail.err = errors.New("smtp down")
	e.users.EXPECT().Create(mock.Anything, mock.Anything).Return(&domain.User{ID: userID, Email: "a@example.com"}, nil)

	_, err := e.svc.Register(context.Background(), service.RegisterInput{Email: "a@example.com", Password: goodPass, Name: "n"})
	assert.NoError(t, err)
}

func TestRegister_RepositoryErrorIsNotMaskedAsDuplicate(t *testing.T) {
	e := newEnv(t)
	boom := errors.New("db down")
	e.users.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, boom)

	_, err := e.svc.Register(context.Background(), service.RegisterInput{Email: "a@example.com", Password: goodPass, Name: "n"})
	assert.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, service.ErrEmailAlreadyExists)
}

// NFR: 既定の bcrypt cost は 12
func TestRegister_DefaultBcryptCostIs12(t *testing.T) {
	clock := now
	tm, _ := auth.NewTokenManager(jwtSecret, time.Minute)
	users := mocks.NewMockUserRepository(t)
	var hash string
	users.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, p repository.CreateUserParams) (*domain.User, error) {
		hash = p.PasswordHash
		return &domain.User{ID: userID}, nil
	})
	svc := service.NewAuthService(users, mocks.NewMockTokenRepository(t), tm, &fakeMailer{}, service.AuthConfig{RefreshTTL: time.Hour, ResetTTL: time.Hour},
		service.WithClock(func() time.Time { return clock }), service.WithAsync(func(f func()) { f() }), service.WithLogger(zerolog.Nop()))

	_, err := svc.Register(context.Background(), service.RegisterInput{Email: "a@example.com", Password: goodPass, Name: "n"})
	require.NoError(t, err)
	cost, err := bcrypt.Cost([]byte(hash))
	require.NoError(t, err)
	assert.Equal(t, 12, cost)
}

// ---------- Login (REQ-002) ----------

func TestLogin_Success(t *testing.T) {
	e := newEnv(t)
	u := userWithPassword(t, goodPass)
	u.Role = domain.RoleStaff
	e.users.EXPECT().FindByEmail(mock.Anything, "user@example.com").Return(u, nil)
	var savedHash string
	var savedExp time.Time
	e.tokens.EXPECT().SaveRefreshToken(mock.Anything, userID, mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, _ uuid.UUID, h string, exp time.Time) (*domain.RefreshToken, error) {
			savedHash, savedExp = h, exp
			return &domain.RefreshToken{}, nil
		})

	pair, err := e.svc.Login(context.Background(), " User@Example.com", goodPass)
	require.NoError(t, err)

	// アクセストークン: 15 分・ロール入り
	id, err := e.tm.ParseAccess(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, userID, id.UserID)
	assert.Equal(t, domain.RoleStaff, id.Role)
	assert.Equal(t, 900, pair.ExpiresIn)

	// リフレッシュトークン: 生値は返し、保存されるのはハッシュのみ・有効期間 7 日
	assert.NotEmpty(t, pair.RefreshToken)
	assert.Equal(t, auth.HashToken(pair.RefreshToken), savedHash)
	assert.NotEqual(t, pair.RefreshToken, savedHash)
	assert.Equal(t, now.Add(7*24*time.Hour), savedExp)
	assert.Equal(t, savedExp, pair.RefreshExpiresAt)
}

// REQ-002-AC-2: 誤パスワードと未登録は区別できない
func TestLogin_WrongPasswordAndUnknownUserAreIndistinguishable(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, "user@example.com").Return(userWithPassword(t, goodPass), nil)
	e.users.EXPECT().FindByEmail(mock.Anything, "nobody@example.com").Return(nil, repository.ErrNotFound)

	_, errWrong := e.svc.Login(context.Background(), "user@example.com", "WrongPassword!!")
	_, errUnknown := e.svc.Login(context.Background(), "nobody@example.com", goodPass)

	assert.ErrorIs(t, errWrong, service.ErrInvalidCredentials)
	assert.ErrorIs(t, errUnknown, service.ErrInvalidCredentials)
	assert.Equal(t, errWrong.Error(), errUnknown.Error())
}

// タイミング差で存在を推測されないよう、未登録でも bcrypt 比較を実行する（時間で検証）
func TestLogin_UnknownUserStillPaysBcryptCost(t *testing.T) {
	clock := now
	tm, _ := auth.NewTokenManager(jwtSecret, time.Minute)
	users := mocks.NewMockUserRepository(t)
	users.EXPECT().FindByEmail(mock.Anything, mock.Anything).Return(nil, repository.ErrNotFound)
	svc := service.NewAuthService(users, mocks.NewMockTokenRepository(t), tm, &fakeMailer{},
		service.AuthConfig{RefreshTTL: time.Hour, ResetTTL: time.Hour, BcryptCost: 10},
		service.WithClock(func() time.Time { return clock }), service.WithLogger(zerolog.Nop()))

	start := time.Now()
	_, err := svc.Login(context.Background(), "nobody@example.com", goodPass)
	elapsed := time.Since(start)

	assert.ErrorIs(t, err, service.ErrInvalidCredentials)
	assert.Greater(t, elapsed, 20*time.Millisecond, "cost 10 の bcrypt 比較が実行されている（未実行なら ~0ms）")
}

func TestLogin_RepositoryErrorIsNotInvalidCredentials(t *testing.T) {
	e := newEnv(t)
	boom := errors.New("db down")
	e.users.EXPECT().FindByEmail(mock.Anything, mock.Anything).Return(nil, boom)

	_, err := e.svc.Login(context.Background(), "a@example.com", goodPass)
	assert.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, service.ErrInvalidCredentials)
}

func TestLogin_SaveTokenFailure(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, mock.Anything).Return(userWithPassword(t, goodPass), nil)
	boom := errors.New("db down")
	e.tokens.EXPECT().SaveRefreshToken(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, boom)

	pair, err := e.svc.Login(context.Background(), "user@example.com", goodPass)
	assert.ErrorIs(t, err, boom)
	assert.Nil(t, pair, "保存できなかったトークンを返さない")
}

// ---------- RefreshToken (REQ-003) ----------

func usableRefresh(hash string) *domain.RefreshToken {
	return &domain.RefreshToken{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), UserID: userID, TokenHash: hash, ExpiresAt: now.Add(time.Hour)}
}

func TestRefreshToken_RotatesToken(t *testing.T) {
	e := newEnv(t)
	old := usableRefresh(auth.HashToken("old-raw"))
	staff := &domain.User{ID: userID, Role: domain.RoleStaff}
	e.tokens.EXPECT().FindRefreshToken(mock.Anything, auth.HashToken("old-raw")).Return(old, nil)
	e.users.EXPECT().FindByID(mock.Anything, userID).Return(staff, nil)
	e.tokens.EXPECT().RevokeRefreshToken(mock.Anything, old.ID).Return(true, nil)
	var newHash string
	e.tokens.EXPECT().SaveRefreshToken(mock.Anything, userID, mock.Anything, now.Add(7*24*time.Hour)).RunAndReturn(
		func(_ context.Context, _ uuid.UUID, h string, _ time.Time) (*domain.RefreshToken, error) {
			newHash = h
			return &domain.RefreshToken{}, nil
		})

	pair, err := e.svc.RefreshToken(context.Background(), "old-raw")
	require.NoError(t, err)

	assert.NotEqual(t, "old-raw", pair.RefreshToken, "ローテーションで新しい値になる")
	assert.Equal(t, auth.HashToken(pair.RefreshToken), newHash)
	id, err := e.tm.ParseAccess(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, domain.RoleStaff, id.Role, "ロールは最新のユーザー情報から")
}

// REQ-003-AC-2: 不明・失効済み・期限切れはすべて ErrInvalidToken（区別しない）
func TestRefreshToken_InvalidCases(t *testing.T) {
	revokedAt := now.Add(-time.Second) // 失効直後(grace 内)の再送は競合として拒否するだけ。再利用検知は別テスト
	cases := map[string]func(e *env){
		"不明": func(e *env) {
			e.tokens.EXPECT().FindRefreshToken(mock.Anything, mock.Anything).Return(nil, repository.ErrNotFound)
		},
		"失効済み": func(e *env) {
			tok := usableRefresh("h")
			tok.RevokedAt = &revokedAt
			e.tokens.EXPECT().FindRefreshToken(mock.Anything, mock.Anything).Return(tok, nil)
		},
		"期限切れ": func(e *env) {
			tok := usableRefresh("h")
			tok.ExpiresAt = now.Add(-time.Second)
			e.tokens.EXPECT().FindRefreshToken(mock.Anything, mock.Anything).Return(tok, nil)
		},
		"期限ちょうど": func(e *env) {
			tok := usableRefresh("h")
			tok.ExpiresAt = now
			e.tokens.EXPECT().FindRefreshToken(mock.Anything, mock.Anything).Return(tok, nil)
		},
		"ユーザーが削除済み": func(e *env) {
			e.tokens.EXPECT().FindRefreshToken(mock.Anything, mock.Anything).Return(usableRefresh("h"), nil)
			e.users.EXPECT().FindByID(mock.Anything, userID).Return(nil, repository.ErrNotFound)
		},
		"競合に負けた(失効が 0 件)": func(e *env) {
			tok := usableRefresh("h")
			e.tokens.EXPECT().FindRefreshToken(mock.Anything, mock.Anything).Return(tok, nil)
			e.users.EXPECT().FindByID(mock.Anything, userID).Return(&domain.User{ID: userID, Role: domain.RolePatient}, nil)
			e.tokens.EXPECT().RevokeRefreshToken(mock.Anything, tok.ID).Return(false, nil)
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t) // SaveRefreshToken は期待していない → 呼ばれたら失敗する
			setup(e)
			pair, err := e.svc.RefreshToken(context.Background(), "raw")
			assert.ErrorIs(t, err, service.ErrInvalidToken)
			assert.Nil(t, pair)
		})
	}
}

func TestRefreshToken_EmptyTokenRejectedWithoutDB(t *testing.T) {
	e := newEnv(t) // repository は一切呼ばれない
	_, err := e.svc.RefreshToken(context.Background(), "")
	assert.ErrorIs(t, err, service.ErrInvalidToken)
}

func TestRefreshToken_RepositoryErrorIsNotInvalidToken(t *testing.T) {
	e := newEnv(t)
	boom := errors.New("db down")
	e.tokens.EXPECT().FindRefreshToken(mock.Anything, mock.Anything).Return(nil, boom)

	_, err := e.svc.RefreshToken(context.Background(), "raw")
	assert.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, service.ErrInvalidToken)
}

// ---------- Logout (REQ-005) ----------

func TestLogout_RevokesToken(t *testing.T) {
	e := newEnv(t)
	tok := usableRefresh(auth.HashToken("raw"))
	e.tokens.EXPECT().FindRefreshToken(mock.Anything, auth.HashToken("raw")).Return(tok, nil)
	e.tokens.EXPECT().RevokeRefreshToken(mock.Anything, tok.ID).Return(true, nil)

	assert.NoError(t, e.svc.Logout(context.Background(), "raw"))
}

// 冪等: 不明・失効済み・空でもエラーにしない（Cookie は常に削除できる）
func TestLogout_IsIdempotent(t *testing.T) {
	t.Run("不明", func(t *testing.T) {
		e := newEnv(t)
		e.tokens.EXPECT().FindRefreshToken(mock.Anything, mock.Anything).Return(nil, repository.ErrNotFound)
		assert.NoError(t, e.svc.Logout(context.Background(), "raw"))
	})
	t.Run("失効済み", func(t *testing.T) {
		e := newEnv(t)
		revoked := now
		tok := usableRefresh("h")
		tok.RevokedAt = &revoked
		e.tokens.EXPECT().FindRefreshToken(mock.Anything, mock.Anything).Return(tok, nil) // Revoke は呼ばれない
		assert.NoError(t, e.svc.Logout(context.Background(), "raw"))
	})
	t.Run("空", func(t *testing.T) {
		e := newEnv(t)
		assert.NoError(t, e.svc.Logout(context.Background(), ""))
	})
}

func TestLogout_RepositoryErrorIsReturned(t *testing.T) {
	e := newEnv(t)
	boom := errors.New("db down")
	e.tokens.EXPECT().FindRefreshToken(mock.Anything, mock.Anything).Return(nil, boom)

	assert.ErrorIs(t, e.svc.Logout(context.Background(), "raw"), boom)
}

// ---------- RequestPasswordReset (REQ-004-AC-1/2) ----------

func TestRequestPasswordReset_KnownUser(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, "user@example.com").Return(&domain.User{ID: userID, Email: "user@example.com", Name: "山田 太郎"}, nil)
	var hash string
	e.tokens.EXPECT().SavePasswordResetToken(mock.Anything, userID, mock.Anything, now.Add(30*time.Minute)).RunAndReturn(
		func(_ context.Context, _ uuid.UUID, h string, _ time.Time) (*domain.PasswordResetToken, error) {
			hash = h
			return &domain.PasswordResetToken{}, nil
		})

	require.NoError(t, e.svc.RequestPasswordReset(context.Background(), " User@Example.com "))

	require.Len(t, e.mail.resets, 1)
	m := e.mail.resets[0]
	assert.Equal(t, "user@example.com", m.to)
	assert.True(t, strings.HasPrefix(m.url, "https://app.example.com/password-reset/confirm?token="))
	raw := strings.TrimPrefix(m.url, "https://app.example.com/password-reset/confirm?token=")
	assert.Equal(t, auth.HashToken(raw), hash, "メールの raw トークンのハッシュだけが保存される")
	assert.NotContains(t, hash, raw)
}

// REQ-004-AC-2: 未登録でも同一の結果(nil)。トークンもメールも作らない
func TestRequestPasswordReset_UnknownUserLooksIdentical(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, "nobody@example.com").Return(nil, repository.ErrNotFound)

	assert.NoError(t, e.svc.RequestPasswordReset(context.Background(), "nobody@example.com"))
	assert.Empty(t, e.mail.resets)
}

// 内部エラーでも応答は同じ(存在の推測防止)。エラーはログに出る
func TestRequestPasswordReset_InternalErrorsAreSwallowed(t *testing.T) {
	t.Run("ユーザー取得失敗", func(t *testing.T) {
		e := newEnv(t)
		e.users.EXPECT().FindByEmail(mock.Anything, mock.Anything).Return(nil, errors.New("db down"))
		assert.NoError(t, e.svc.RequestPasswordReset(context.Background(), "a@example.com"))
	})
	t.Run("トークン保存失敗", func(t *testing.T) {
		e := newEnv(t)
		e.users.EXPECT().FindByEmail(mock.Anything, mock.Anything).Return(&domain.User{ID: userID, Email: "a@example.com"}, nil)
		e.tokens.EXPECT().SavePasswordResetToken(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("db down"))
		assert.NoError(t, e.svc.RequestPasswordReset(context.Background(), "a@example.com"))
		assert.Empty(t, e.mail.resets, "保存できなかったトークンのメールは送らない")
	})
}

func TestRequestPasswordReset_MailFailureIsSwallowed(t *testing.T) {
	e := newEnv(t)
	e.mail.err = errors.New("smtp down")
	e.users.EXPECT().FindByEmail(mock.Anything, mock.Anything).Return(&domain.User{ID: userID, Email: "a@example.com"}, nil)
	e.tokens.EXPECT().SavePasswordResetToken(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&domain.PasswordResetToken{}, nil)

	assert.NoError(t, e.svc.RequestPasswordReset(context.Background(), "a@example.com"))
}

// ---------- ConfirmPasswordReset (REQ-004-AC-3/4) ----------

const newPass = "NewSecurePass456!"

func usableReset() *domain.PasswordResetToken {
	return &domain.PasswordResetToken{ID: uuid.MustParse("44444444-4444-4444-4444-444444444444"), UserID: userID, ExpiresAt: now.Add(10 * time.Minute)}
}

func TestConfirmPasswordReset_Success(t *testing.T) {
	e := newEnv(t)
	tok := usableReset()
	e.tokens.EXPECT().FindPasswordResetToken(mock.Anything, auth.HashToken("raw")).Return(tok, nil)
	e.tokens.EXPECT().MarkPasswordResetTokenUsed(mock.Anything, tok.ID).Return(true, nil)
	e.tokens.EXPECT().RevokeAllRefreshTokens(mock.Anything, userID).Return(int64(2), nil)
	var newHash string
	e.users.EXPECT().UpdatePassword(mock.Anything, userID, mock.Anything).RunAndReturn(
		func(_ context.Context, _ uuid.UUID, h string) error { newHash = h; return nil })
	e.users.EXPECT().FindByID(mock.Anything, userID).Return(&domain.User{ID: userID, Email: "user@example.com"}, nil)

	require.NoError(t, e.svc.ConfirmPasswordReset(context.Background(), "raw", newPass))
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(newHash), []byte(newPass)))
}

// REQ-004-AC-4: 期限切れ・使用済み・不明は同じエラー。副作用なし
func TestConfirmPasswordReset_InvalidTokens(t *testing.T) {
	used := now.Add(-time.Minute)
	cases := map[string]func(e *env){
		"不明": func(e *env) {
			e.tokens.EXPECT().FindPasswordResetToken(mock.Anything, mock.Anything).Return(nil, repository.ErrNotFound)
		},
		"使用済み": func(e *env) {
			tok := usableReset()
			tok.UsedAt = &used
			e.tokens.EXPECT().FindPasswordResetToken(mock.Anything, mock.Anything).Return(tok, nil)
		},
		"期限切れ": func(e *env) {
			tok := usableReset()
			tok.ExpiresAt = now.Add(-time.Second)
			e.tokens.EXPECT().FindPasswordResetToken(mock.Anything, mock.Anything).Return(tok, nil)
		},
		"期限ちょうど": func(e *env) {
			tok := usableReset()
			tok.ExpiresAt = now
			e.tokens.EXPECT().FindPasswordResetToken(mock.Anything, mock.Anything).Return(tok, nil)
		},
		"競合に負けた(使用済み化が 0 件)": func(e *env) {
			tok := usableReset()
			e.tokens.EXPECT().FindPasswordResetToken(mock.Anything, mock.Anything).Return(tok, nil)
			e.tokens.EXPECT().MarkPasswordResetTokenUsed(mock.Anything, tok.ID).Return(false, nil)
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t) // UpdatePassword / RevokeAll は期待していない → 呼ばれたら失敗
			setup(e)
			err := e.svc.ConfirmPasswordReset(context.Background(), "raw", newPass)
			assert.ErrorIs(t, err, service.ErrInvalidResetToken)
		})
	}
}

// 弱いパスワードではトークンを消費しない（やり直せる）
func TestConfirmPasswordReset_WeakPasswordDoesNotConsumeToken(t *testing.T) {
	e := newEnv(t) // Find / Mark / Update はいずれも呼ばれない
	err := e.svc.ConfirmPasswordReset(context.Background(), "raw", "short")
	assert.ErrorIs(t, err, service.ErrWeakPassword)
}

func TestConfirmPasswordReset_EmptyTokenRejectedWithoutDB(t *testing.T) {
	e := newEnv(t)
	assert.ErrorIs(t, e.svc.ConfirmPasswordReset(context.Background(), "", newPass), service.ErrInvalidResetToken)
}

// セッション失効に失敗したらパスワードは変更しない（旧パスワードで盗まれたセッションが残る状態を作らない）
func TestConfirmPasswordReset_RevokeFailureLeavesPasswordUnchanged(t *testing.T) {
	e := newEnv(t) // UpdatePassword は期待していない
	tok := usableReset()
	boom := errors.New("db down")
	e.tokens.EXPECT().FindPasswordResetToken(mock.Anything, mock.Anything).Return(tok, nil)
	e.tokens.EXPECT().MarkPasswordResetTokenUsed(mock.Anything, tok.ID).Return(true, nil)
	e.tokens.EXPECT().RevokeAllRefreshTokens(mock.Anything, userID).Return(int64(0), boom)

	assert.ErrorIs(t, e.svc.ConfirmPasswordReset(context.Background(), "raw", newPass), boom)
}

func TestConfirmPasswordReset_UpdateFailureIsReturned(t *testing.T) {
	e := newEnv(t)
	tok := usableReset()
	boom := errors.New("db down")
	e.tokens.EXPECT().FindPasswordResetToken(mock.Anything, mock.Anything).Return(tok, nil)
	e.tokens.EXPECT().MarkPasswordResetTokenUsed(mock.Anything, tok.ID).Return(true, nil)
	e.tokens.EXPECT().RevokeAllRefreshTokens(mock.Anything, userID).Return(int64(1), nil)
	e.users.EXPECT().UpdatePassword(mock.Anything, userID, mock.Anything).Return(boom)

	assert.ErrorIs(t, e.svc.ConfirmPasswordReset(context.Background(), "raw", newPass), boom)
}

// ---------- アカウント単位のログイン試行制限 (REQ-002-AC-3) ----------

func wrongLogins(t *testing.T, e *env, email string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		_, err := e.svc.Login(context.Background(), email, "WrongPassword!!")
		require.ErrorIs(t, err, service.ErrInvalidCredentials, "試行 %d", i+1)
	}
}

func TestLogin_AccountLockedAfterFiveFailures(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, "user@example.com").Return(userWithPassword(t, goodPass), nil).Times(5)

	wrongLogins(t, e, "user@example.com", 5)

	// 6 回目は正しいパスワードでも拒否。DB（FindByEmail）にも触れない
	pair, err := e.svc.Login(context.Background(), "user@example.com", goodPass)
	assert.Nil(t, pair)
	var tooMany *service.TooManyAttemptsError
	require.ErrorAs(t, err, &tooMany)
	assert.Equal(t, 15*time.Minute, tooMany.RetryAfter)
	assert.ErrorIs(t, err, service.ErrTooManyAttempts)
}

// 4 回失敗 → 成功でリセット → さらに 4 回失敗してもロックされない
func TestLogin_SuccessResetsFailureCount(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, "user@example.com").Return(userWithPassword(t, goodPass), nil)
	e.tokens.EXPECT().SaveRefreshToken(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&domain.RefreshToken{}, nil)

	wrongLogins(t, e, "user@example.com", 4)
	_, err := e.svc.Login(context.Background(), "user@example.com", goodPass)
	require.NoError(t, err)
	wrongLogins(t, e, "user@example.com", 4)

	_, err = e.svc.Login(context.Background(), "user@example.com", goodPass)
	assert.NoError(t, err, "リセット後の 4 回失敗ではロックされない")
}

// 未登録のメールも同じ回数でロックされる（ロックの有無で存在を推測できない）
func TestLogin_UnknownAccountIsLockedIdentically(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, "nobody@example.com").Return(nil, repository.ErrNotFound).Times(5)

	wrongLogins(t, e, "nobody@example.com", 5)

	_, err := e.svc.Login(context.Background(), "nobody@example.com", goodPass)
	assert.ErrorIs(t, err, service.ErrTooManyAttempts)
}

// アカウントごとに独立（共有 IP の別患者を巻き込まない）
func TestLogin_LockIsPerAccount(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, "alice@example.com").Return(userWithPassword(t, goodPass), nil).Times(5)
	bob := userWithPassword(t, goodPass)
	bob.Email = "bob@example.com"
	e.users.EXPECT().FindByEmail(mock.Anything, "bob@example.com").Return(bob, nil)
	e.tokens.EXPECT().SaveRefreshToken(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&domain.RefreshToken{}, nil)

	wrongLogins(t, e, "alice@example.com", 5)

	_, err := e.svc.Login(context.Background(), "bob@example.com", goodPass)
	assert.NoError(t, err)
}

// メールの大文字小文字・前後の空白を変えても同じアカウントとして数える
func TestLogin_LockKeyIsNormalizedEmail(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, "user@example.com").Return(userWithPassword(t, goodPass), nil).Times(5)

	for _, variant := range []string{"user@example.com", "USER@example.com", " User@Example.com", "user@EXAMPLE.COM ", "uSeR@example.com"} {
		_, err := e.svc.Login(context.Background(), variant, "WrongPassword!!")
		require.ErrorIs(t, err, service.ErrInvalidCredentials)
	}

	_, err := e.svc.Login(context.Background(), "  USER@EXAMPLE.COM", goodPass)
	assert.ErrorIs(t, err, service.ErrTooManyAttempts)
}

// 時間が経てば解除される
func TestLogin_LockExpires(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, "user@example.com").Return(userWithPassword(t, goodPass), nil)
	e.tokens.EXPECT().SaveRefreshToken(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&domain.RefreshToken{}, nil)
	wrongLogins(t, e, "user@example.com", 5)

	*e.clock = now.Add(15*time.Minute - time.Second)
	_, err := e.svc.Login(context.Background(), "user@example.com", goodPass)
	assert.ErrorIs(t, err, service.ErrTooManyAttempts)

	*e.clock = now.Add(15 * time.Minute)
	_, err = e.svc.Login(context.Background(), "user@example.com", goodPass)
	assert.NoError(t, err)
}

// DB 障害による失敗はロック対象にしない（利用者の責任ではない）
func TestLogin_InfrastructureErrorsDoNotCountAsFailures(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, "user@example.com").Return(nil, errors.New("db down")).Times(10)

	for i := 0; i < 10; i++ {
		_, err := e.svc.Login(context.Background(), "user@example.com", goodPass)
		require.Error(t, err)
		require.NotErrorIs(t, err, service.ErrTooManyAttempts)
	}
}

// ---------- パスワードリセット申請の宛先単位の制限（メール爆撃対策） ----------

func TestRequestPasswordReset_ThrottledPerAccount(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, "user@example.com").Return(&domain.User{ID: userID, Email: "user@example.com", Name: "n"}, nil).Times(3)
	e.tokens.EXPECT().SavePasswordResetToken(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&domain.PasswordResetToken{}, nil).Times(3)

	for i := 0; i < 5; i++ {
		assert.NoError(t, e.svc.RequestPasswordReset(context.Background(), "user@example.com"), "応答は常に同じ（成功）")
	}
	assert.Len(t, e.mail.resets, 3, "1 時間に 3 通まで。4 通目以降は送らない")
}

func TestRequestPasswordReset_ThrottleIsPerAccountAndExpires(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, email string) (*domain.User, error) {
			return &domain.User{ID: userID, Email: email, Name: "n"}, nil
		})
	e.tokens.EXPECT().SavePasswordResetToken(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&domain.PasswordResetToken{}, nil)

	for i := 0; i < 4; i++ {
		_ = e.svc.RequestPasswordReset(context.Background(), "a@example.com")
	}
	_ = e.svc.RequestPasswordReset(context.Background(), "b@example.com")
	assert.Len(t, e.mail.resets, 4, "a は 3 通、b は別枠で 1 通")

	*e.clock = now.Add(time.Hour)
	_ = e.svc.RequestPasswordReset(context.Background(), "a@example.com")
	assert.Len(t, e.mail.resets, 5, "1 時間後は再び送れる")
}

// 未登録メールでも同じ回数で制限し、応答は変わらない（存在の推測防止）
func TestRequestPasswordReset_UnknownAccountThrottledSilently(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, "nobody@example.com").Return(nil, repository.ErrNotFound)

	for i := 0; i < 5; i++ {
		assert.NoError(t, e.svc.RequestPasswordReset(context.Background(), "nobody@example.com"))
	}
	assert.Empty(t, e.mail.resets)
}

// パスワード変更に成功したらそのアカウントのログイン失敗カウントを解除する
func TestConfirmPasswordReset_UnlocksLoginLock(t *testing.T) {
	e := newEnv(t)
	e.users.EXPECT().FindByEmail(mock.Anything, "user@example.com").Return(userWithPassword(t, goodPass), nil).Times(5)
	wrongLogins(t, e, "user@example.com", 5)
	_, err := e.svc.Login(context.Background(), "user@example.com", goodPass)
	require.ErrorIs(t, err, service.ErrTooManyAttempts)

	tok := usableReset()
	e.tokens.EXPECT().FindPasswordResetToken(mock.Anything, mock.Anything).Return(tok, nil)
	e.tokens.EXPECT().MarkPasswordResetTokenUsed(mock.Anything, tok.ID).Return(true, nil)
	e.tokens.EXPECT().RevokeAllRefreshTokens(mock.Anything, userID).Return(int64(0), nil)
	e.users.EXPECT().FindByID(mock.Anything, userID).Return(&domain.User{ID: userID, Email: "user@example.com"}, nil)
	e.users.EXPECT().UpdatePassword(mock.Anything, userID, mock.Anything).Return(nil)
	require.NoError(t, e.svc.ConfirmPasswordReset(context.Background(), "raw", newPass))

	e.users.EXPECT().FindByEmail(mock.Anything, "user@example.com").Return(userWithPassword(t, newPass), nil)
	e.tokens.EXPECT().SaveRefreshToken(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&domain.RefreshToken{}, nil)
	_, err = e.svc.Login(context.Background(), "user@example.com", newPass)
	assert.NoError(t, err, "本人確認済みのパスワード再設定でロックが解除される")
}

// S5: 失効済みトークンを grace 超過後に再利用したら盗用とみなし、全セッションを失効させる
func TestRefreshToken_ReuseOfRevokedTokenRevokesAllSessions(t *testing.T) {
	e := newEnv(t)
	revokedAt := now.Add(-time.Minute)
	tok := usableRefresh("h")
	tok.RevokedAt = &revokedAt
	e.tokens.EXPECT().FindRefreshToken(mock.Anything, mock.Anything).Return(tok, nil)
	e.tokens.EXPECT().RevokeAllRefreshTokens(mock.Anything, userID).Return(int64(2), nil).Once()

	pair, err := e.svc.RefreshToken(context.Background(), "raw")

	assert.ErrorIs(t, err, service.ErrInvalidToken)
	assert.Nil(t, pair)
}

// 全失効に失敗したら成功扱いにせずエラーを返す(盗用側を残さない)
func TestRefreshToken_ReuseDetectionFailureIsReported(t *testing.T) {
	e := newEnv(t)
	revokedAt := now.Add(-time.Minute)
	tok := usableRefresh("h")
	tok.RevokedAt = &revokedAt
	e.tokens.EXPECT().FindRefreshToken(mock.Anything, mock.Anything).Return(tok, nil)
	boom := errors.New("db down")
	e.tokens.EXPECT().RevokeAllRefreshTokens(mock.Anything, userID).Return(int64(0), boom)

	_, err := e.svc.RefreshToken(context.Background(), "raw")

	assert.ErrorIs(t, err, boom)
}

// S3: ロックはメール+IP 単位。攻撃者の IP が失敗を重ねても、本人の別 IP からのログインは通る
func TestLogin_LockIsPerAccountAndIP(t *testing.T) {
	e := newEnv(t)
	e.tokens.EXPECT().SaveRefreshToken(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&domain.RefreshToken{}, nil)
	attacker := service.WithClientIP(context.Background(), "203.0.113.9")
	owner := service.WithClientIP(context.Background(), "198.51.100.7")
	e.users.EXPECT().FindByEmail(mock.Anything, "user@example.com").Return(userWithPassword(t, goodPass), nil).Times(6)

	for i := 0; i < 5; i++ {
		_, err := e.svc.Login(attacker, "user@example.com", "WrongPassword!!")
		require.ErrorIs(t, err, service.ErrInvalidCredentials)
	}
	_, err := e.svc.Login(attacker, "user@example.com", goodPass)
	assert.ErrorIs(t, err, service.ErrTooManyAttempts, "攻撃元 IP はロックされる")

	_, err = e.svc.Login(owner, "user@example.com", goodPass)
	assert.NoError(t, err, "本人の別 IP は締め出されない")
}
