// Package service は業務ロジック。repository は interface のみに依存する。
package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/kava2108/booking/backend/internal/auth"
	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
)

const (
	// refreshReuseGrace は、失効直後の再送を競合として許容する時間。これを超えた再利用は盗用とみなす。
	refreshReuseGrace = 10 * time.Second
	// DefaultBcryptCost は NFR(bcrypt cost >= 12)に従う既定値。
	DefaultBcryptCost = 12
	minPasswordRunes  = 12
	maxPasswordBytes  = 72 // bcrypt の入力上限
)

var (
	ErrEmailAlreadyExists = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrWeakPassword       = errors.New("password must be 12 to 72 bytes and at least 12 characters")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrInvalidResetToken  = errors.New("invalid or expired reset token")
	// ErrTooManyAttempts はアカウントのログイン連続失敗によるロック中。errors.As で TooManyAttemptsError を取り出せる。
	ErrTooManyAttempts = errors.New("too many login attempts")
)

// TooManyAttemptsError はロック中であることと、解除までの残り時間（Retry-After）を運ぶ。
type TooManyAttemptsError struct{ RetryAfter time.Duration }

func (e *TooManyAttemptsError) Error() string        { return ErrTooManyAttempts.Error() }
func (e *TooManyAttemptsError) Is(target error) bool { return target == ErrTooManyAttempts }

// RegisterInput は登録リクエスト。
type RegisterInput struct {
	Email    string
	Password string
	Name     string
}

// TokenPair はログイン/リフレッシュの結果。RefreshToken は生の値(Cookie に載せるだけ。保存しない)。
type TokenPair struct {
	AccessToken      string
	ExpiresIn        int // アクセストークンの有効秒数
	RefreshToken     string
	RefreshExpiresAt time.Time
}

// AuthService は認証の業務ロジック。
type AuthService interface {
	Register(ctx context.Context, in RegisterInput) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*TokenPair, error)
	RefreshToken(ctx context.Context, rawRefreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, rawRefreshToken string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ConfirmPasswordReset(ctx context.Context, rawToken, newPassword string) error
}

// AuthConfig は service の設定。
type AuthConfig struct {
	RefreshTTL   time.Duration
	ResetTTL     time.Duration
	BcryptCost   int    // 0 のとき DefaultBcryptCost
	ResetURLBase string // フロントの URL(例: https://app.example.com)

	// ログイン: アカウント(正規化したメール)単位で、連続 LoginMaxFailures 回の失敗で LoginLockWindow ロック（REQ-002-AC-3）
	LoginMaxFailures int           // 0 のとき 5
	LoginLockWindow  time.Duration // 0 のとき 15 分
	// パスワードリセット申請: 宛先単位で ResetWindow 内に ResetMaxRequests 通まで（メール爆撃対策）
	ResetMaxRequests int           // 0 のとき 3
	ResetWindow      time.Duration // 0 のとき 1 時間
}

// Option は AuthService の差し替え可能な依存。
type Option func(*authService)

func WithClock(now func() time.Time) Option { return func(s *authService) { s.now = now } }
func WithAsync(run func(func())) Option     { return func(s *authService) { s.async = run } }
func WithLogger(l zerolog.Logger) Option    { return func(s *authService) { s.log = l } }

type authService struct {
	users  repository.UserRepository
	tokens repository.TokenRepository
	tm     *auth.TokenManager
	mailer Mailer
	cfg    AuthConfig

	now        func() time.Time
	async      func(func())
	log        zerolog.Logger
	loginGuard AttemptGuard
	resetGuard AttemptGuard
	// dummyHash は未登録ユーザーのログインでも同じ時間だけ bcrypt 比較するためのハッシュ。
	dummyHash []byte
}

func NewAuthService(users repository.UserRepository, tokens repository.TokenRepository, tm *auth.TokenManager, mailer Mailer, cfg AuthConfig, opts ...Option) AuthService {
	if cfg.BcryptCost == 0 {
		cfg.BcryptCost = DefaultBcryptCost
	}
	s := &authService{
		users: users, tokens: tokens, tm: tm, mailer: mailer, cfg: cfg,
		now: time.Now, async: func(f func()) { go f() }, log: zerolog.Nop(),
	}
	for _, o := range opts {
		o(s)
	}
	if cfg.LoginMaxFailures == 0 {
		cfg.LoginMaxFailures = 5
	}
	if cfg.LoginLockWindow == 0 {
		cfg.LoginLockWindow = 15 * time.Minute
	}
	if cfg.ResetMaxRequests == 0 {
		cfg.ResetMaxRequests = 3
	}
	if cfg.ResetWindow == 0 {
		cfg.ResetWindow = time.Hour
	}
	s.cfg = cfg
	if s.loginGuard == nil {
		s.loginGuard = NewMemoryGuard(cfg.LoginMaxFailures, cfg.LoginLockWindow, WithGuardClock(s.now))
	}
	if s.resetGuard == nil {
		s.resetGuard = NewMemoryGuard(cfg.ResetMaxRequests, cfg.ResetWindow, WithGuardClock(s.now))
	}
	s.dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), cfg.BcryptCost)
	return s
}

type clientIPKey struct{}

// WithClientIP はログイン試行の制限に使うクライアント IP を ctx に載せる。
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey{}, ip)
}

// loginKey はログイン試行の制限キー。メール単位に IP を足し、他人のメールで失敗を重ねても
// 本人を締め出せないようにする(別の IP からのログインはロックされない)。IP が無ければメールのみ。
func loginKey(ctx context.Context, email string) string {
	key := normalizeEmail(email)
	if ip, _ := ctx.Value(clientIPKey{}).(string); ip != "" {
		key += "|" + ip
	}
	return key
}

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func validatePassword(pw string) error {
	if utf8.RuneCountInString(pw) < minPasswordRunes || len(pw) > maxPasswordBytes {
		return ErrWeakPassword
	}
	return nil
}

// Register: 正規化 → パスワード検証 → bcrypt → 保存(重複は一意制約で検出) → 確認メール(非同期)
func (s *authService) Register(ctx context.Context, in RegisterInput) (*domain.User, error) {
	email := normalizeEmail(in.Email)
	if err := validatePassword(in.Password); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), s.cfg.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	u, err := s.users.Create(ctx, repository.CreateUserParams{Email: email, PasswordHash: string(hash), Name: strings.TrimSpace(in.Name)})
	if err != nil {
		if errors.Is(err, repository.ErrEmailTaken) {
			return nil, ErrEmailAlreadyExists
		}
		return nil, err
	}
	s.sendAsync("registration_confirmation", func(ctx context.Context) error {
		return s.mailer.SendRegistrationConfirmation(ctx, email, u.Name)
	})
	return u, nil
}

// Login: アカウント(正規化したメール)+クライアント IP 単位の試行制限つき。
//   - 連続失敗でロック中は、正しいパスワードでも拒否し、DB にも触れない。
//   - 未登録メールも同じ回数・同じ挙動でロックし、応答時間も揃える(存在を推測させない)。
//   - インフラ障害による失敗は数えない。成功でカウントをリセットする。
func (s *authService) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	addr := normalizeEmail(email)
	key := loginKey(ctx, email)
	if blocked, retry := s.loginGuard.Blocked(key); blocked {
		return nil, &TooManyAttemptsError{RetryAfter: retry}
	}
	u, err := s.users.FindByEmail(ctx, addr)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		s.loginGuard.Fail(key)
		return nil, ErrInvalidCredentials
	case err != nil:
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		s.loginGuard.Fail(key)
		return nil, ErrInvalidCredentials
	}
	s.loginGuard.Reset(key)
	return s.issuePair(ctx, u)
}

// issuePair はアクセストークンとリフレッシュトークンを発行し、リフレッシュのハッシュだけを保存する。
func (s *authService) issuePair(ctx context.Context, u *domain.User) (*TokenPair, error) {
	access, ttl, err := s.tm.IssueAccess(u.ID, u.Role)
	if err != nil {
		return nil, err
	}
	raw, hash, err := auth.GenerateOpaqueToken()
	if err != nil {
		return nil, err
	}
	expires := s.now().Add(s.cfg.RefreshTTL)
	if _, err := s.tokens.SaveRefreshToken(ctx, u.ID, hash, expires); err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, ExpiresIn: int(ttl.Seconds()), RefreshToken: raw, RefreshExpiresAt: expires}, nil
}

// sendAsync はメール送信をリクエストから切り離して実行する。失敗は error ログのみ(宛先・トークンは出さない)。
func (s *authService) sendAsync(kind string, send func(ctx context.Context) error) {
	s.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := send(ctx); err != nil {
			s.log.Error().Err(err).Str("mail_kind", kind).Msg("failed to send mail")
		}
	})
}

// RefreshToken: リフレッシュトークンをローテーションする。
// 失効の更新件数で「同時に使われたか」を判定し、先着 1 件だけを成功させる。
func (s *authService) RefreshToken(ctx context.Context, raw string) (*TokenPair, error) {
	if raw == "" {
		return nil, ErrInvalidToken
	}
	tok, err := s.tokens.FindRefreshToken(ctx, auth.HashToken(raw))
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return nil, ErrInvalidToken
	case err != nil:
		return nil, err
	}
	if tok.RevokedAt != nil && s.now().Sub(*tok.RevokedAt) > refreshReuseGrace {
		// 失効済みトークンの再利用 = 盗用の疑い。そのユーザーの全セッションを失効させる。
		// 直後(grace 内)の再送は、複数タブの同時更新などの正規の競合として扱い、失効だけ拒否する。
		if _, err := s.tokens.RevokeAllRefreshTokens(ctx, tok.UserID); err != nil {
			return nil, err
		}
		s.log.Warn().Str("user_id", tok.UserID.String()).Msg("refresh token reuse detected; all sessions revoked")
		return nil, ErrInvalidToken
	}
	if !tok.IsUsable(s.now()) {
		return nil, ErrInvalidToken
	}
	u, err := s.users.FindByID(ctx, tok.UserID)
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return nil, ErrInvalidToken
	case err != nil:
		return nil, err
	}
	revoked, err := s.tokens.RevokeRefreshToken(ctx, tok.ID)
	if err != nil {
		return nil, err
	}
	if !revoked {
		return nil, ErrInvalidToken
	}
	return s.issuePair(ctx, u)
}

// Logout: トークンを失効させる。不明・失効済み・空は成功扱い(冪等。Cookie は常に削除できる)。
func (s *authService) Logout(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	tok, err := s.tokens.FindRefreshToken(ctx, auth.HashToken(raw))
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return nil
	case err != nil:
		return err
	}
	if tok.RevokedAt != nil {
		return nil
	}
	_, err = s.tokens.RevokeRefreshToken(ctx, tok.ID)
	return err
}

// RequestPasswordReset: 登録の有無にかかわらず常に nil を返す(存在の推測防止)。
// 内部エラーも応答には出さず error ログのみ。メールの宛先・トークンはログに出さない。
func (s *authService) RequestPasswordReset(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	// 宛先単位の頻度制限。登録の有無にかかわらず同じ数え方・同じ応答(メール爆撃と存在推測の防止)。
	if blocked, _ := s.resetGuard.Blocked(email); blocked {
		return nil
	}
	s.resetGuard.Fail(email)
	u, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			s.log.Error().Err(err).Msg("password reset: find user failed")
		}
		return nil
	}
	raw, hash, err := auth.GenerateOpaqueToken()
	if err != nil {
		s.log.Error().Err(err).Msg("password reset: generate token failed")
		return nil
	}
	if _, err := s.tokens.SavePasswordResetToken(ctx, u.ID, hash, s.now().Add(s.cfg.ResetTTL)); err != nil {
		s.log.Error().Err(err).Msg("password reset: save token failed")
		return nil
	}
	resetURL := strings.TrimRight(s.cfg.ResetURLBase, "/") + "/password-reset/confirm?token=" + url.QueryEscape(raw)
	s.sendAsync("password_reset", func(ctx context.Context) error {
		return s.mailer.SendPasswordReset(ctx, u.Email, u.Name, resetURL)
	})
	return nil
}

// ConfirmPasswordReset: パスワードを更新する。
// 順序: 入力検証(弱いパスワードでトークンを消費しない) → トークン検証 → 使用済み化(先着 1 件)
//
//	→ 全リフレッシュトークン失効 → パスワード更新。
//
// セッション失効に失敗した場合はパスワードを変更しない。
func (s *authService) ConfirmPasswordReset(ctx context.Context, raw, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	if raw == "" {
		return ErrInvalidResetToken
	}
	tok, err := s.tokens.FindPasswordResetToken(ctx, auth.HashToken(raw))
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrInvalidResetToken
	case err != nil:
		return err
	}
	if !tok.IsUsable(s.now()) {
		return ErrInvalidResetToken
	}
	marked, err := s.tokens.MarkPasswordResetTokenUsed(ctx, tok.ID)
	if err != nil {
		return err
	}
	if !marked {
		return ErrInvalidResetToken
	}
	if _, err := s.tokens.RevokeAllRefreshTokens(ctx, tok.UserID); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.cfg.BcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := s.users.UpdatePassword(ctx, tok.UserID, string(hash)); err != nil {
		return err
	}
	// 本人確認済みのパスワード再設定なので、ログイン失敗によるロックを解除する
	if u, err := s.users.FindByID(ctx, tok.UserID); err == nil {
		s.loginGuard.ResetAccount(normalizeEmail(u.Email))
	}
	return nil
}
