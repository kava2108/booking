package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/routers"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/kava2108/booking/backend/internal/app"
	"github.com/kava2108/booking/backend/internal/auth"
	"github.com/kava2108/booking/backend/internal/config"
	"github.com/kava2108/booking/backend/internal/middleware"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/service"
	"github.com/kava2108/booking/backend/internal/testutil/pgtest"
)

// IMP 10-user-auth-auth-handler TASK-0001: 実 DB を使った全エンドポイントの統合テスト

const (
	pass    = "SecurePass123!"
	newPass = "NewSecurePass456!"
	secret  = "0123456789abcdef0123456789abcdef"
)

type testApp struct {
	t      *testing.T
	e      *echo.Echo
	q      *db.Queries
	pool   *pgxpool.Pool
	logs   *bytes.Buffer
	mail   *recordingMailer
	tm     *auth.TokenManager
	users  repository.UserRepository
	router routers.Router
}

func newApp(t *testing.T, mutate func(*config.Config)) *testApp {
	t.Helper()
	pool := pgtest.NewPool(t)
	q := db.New(pool)
	users, tokens := repository.NewUserRepository(q), repository.NewTokenRepository(q)
	tm, err := auth.NewTokenManager(secret, 15*time.Minute)
	require.NoError(t, err)
	mail := &recordingMailer{}
	logs := &bytes.Buffer{}
	logger := zerolog.New(logs)

	cfg := &config.Config{
		JWTRefreshTTL: 7 * 24 * time.Hour, RateLimitLoginMax: 1000, RateLimitLoginIPMax: 1000, RateLimitSensitiveMax: 1000, RateLimitLoginWindow: time.Minute,
		RateLimitPublicMax: 1000, RateLimitPublicWindow: time.Minute, CookieSecure: true, AppBaseURL: "https://app.example.com",
	}
	if mutate != nil {
		mutate(cfg)
	}
	svc := service.NewAuthService(users, tokens, tm, mail,
		service.AuthConfig{RefreshTTL: cfg.JWTRefreshTTL, ResetTTL: 30 * time.Minute, BcryptCost: bcrypt.MinCost, ResetURLBase: cfg.AppBaseURL,
			LoginMaxFailures: cfg.RateLimitLoginMax, LoginLockWindow: cfg.RateLimitLoginWindow, ResetMaxRequests: 3, ResetWindow: time.Hour},
		service.WithAsync(func(f func()) { f() }), service.WithLogger(logger))

	e := app.New(app.Deps{Config: cfg, Logger: logger, Auth: svc, Tokens: tm})
	return &testApp{t: t, router: loadRouter(t), e: e, q: q, pool: pool, logs: logs, mail: mail, tm: tm, users: users}
}

type recordingMailer struct {
	mu     sync.Mutex
	resets []string
	regs   []string
}

func (m *recordingMailer) SendRegistrationConfirmation(_ context.Context, to, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.regs = append(m.regs, to)
	return nil
}
func (m *recordingMailer) SendPasswordReset(_ context.Context, _, _, resetURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resets = append(m.resets, resetURL)
	return nil
}

func (a *testApp) do(method, path string, body any, mods ...func(*http.Request)) *httptest.ResponseRecorder {
	var rd *bytes.Reader
	switch b := body.(type) {
	case nil:
		rd = bytes.NewReader(nil)
	case string:
		rd = bytes.NewReader([]byte(b))
	default:
		j, _ := json.Marshal(b)
		rd = bytes.NewReader(j)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:5555"
	for _, m := range mods {
		m(req)
	}
	rec := httptest.NewRecorder()
	a.e.ServeHTTP(rec, req)
	conformsToOpenAPI(a.t, a.router, req, rec) // 認証 API の応答も OpenAPI 定義と突き合わせる
	return rec
}

func withCookie(v string) func(*http.Request) {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "refresh_token", Value: v}) }
}
func withBearer(v string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+v) }
}
func withIP(ip string) func(*http.Request) { return func(r *http.Request) { r.RemoteAddr = ip + ":1" } }

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m), rec.Body.String())
	return m
}

func refreshCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "refresh_token" {
			return c
		}
	}
	return nil
}

func (a *testApp) register(t *testing.T, email string) {
	t.Helper()
	rec := a.do(http.MethodPost, "/api/v1/auth/register", map[string]string{"email": email, "password": pass, "name": "山田 太郎"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func (a *testApp) login(t *testing.T, email, password string) *httptest.ResponseRecorder {
	return a.do(http.MethodPost, "/api/v1/auth/login", map[string]string{"email": email, "password": password})
}

// ---------- 全フロー (REQ-001〜003, 005) ----------

func TestAuthFlow_RegisterLoginRefreshLogout(t *testing.T) {
	a := newApp(t, nil)

	// 登録: 201。パスワード・ハッシュを返さない
	rec := a.do(http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "User@Example.com", "password": pass, "name": "山田 太郎"})
	require.Equal(t, http.StatusCreated, rec.Code)
	body := decode(t, rec)
	assert.Equal(t, "user@example.com", body["email"])
	assert.NotEmpty(t, body["user_id"])
	assert.NotContains(t, rec.Body.String(), "password")
	assert.Equal(t, []string{"user@example.com"}, a.mail.regs)

	// ログイン: アクセストークン + HttpOnly Cookie
	rec = a.login(t, "user@example.com", pass)
	require.Equal(t, http.StatusOK, rec.Code)
	login := decode(t, rec)
	assert.Equal(t, "Bearer", login["token_type"])
	assert.EqualValues(t, 900, login["expires_in"])
	access := login["access_token"].(string)
	id, err := a.tm.ParseAccess(access)
	require.NoError(t, err)
	assert.Equal(t, "patient", string(id.Role))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	c1 := refreshCookie(rec)
	require.NotNil(t, c1)
	assert.True(t, c1.HttpOnly)
	assert.True(t, c1.Secure)
	assert.Equal(t, http.SameSiteStrictMode, c1.SameSite)
	assert.Equal(t, "/api/v1/auth", c1.Path)
	assert.Greater(t, c1.MaxAge, 6*24*3600)

	// リフレッシュ: 新しいトークンに交換され、旧トークンは使えない
	rec = a.do(http.MethodPost, "/api/v1/auth/refresh", nil, withCookie(c1.Value))
	require.Equal(t, http.StatusOK, rec.Code)
	c2 := refreshCookie(rec)
	require.NotNil(t, c2)
	assert.NotEqual(t, c1.Value, c2.Value)
	assert.NotEmpty(t, decode(t, rec)["access_token"])
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, http.StatusUnauthorized, a.do(http.MethodPost, "/api/v1/auth/refresh", nil, withCookie(c1.Value)).Code, "ローテーション済みの旧トークンは無効")

	// ログアウト: Cookie 削除。以後そのリフレッシュトークンは 401 (REQ-005-AC-2)
	rec = a.do(http.MethodPost, "/api/v1/auth/logout", nil, withBearer(access), withCookie(c2.Value))
	require.Equal(t, http.StatusOK, rec.Code)
	cleared := refreshCookie(rec)
	require.NotNil(t, cleared)
	assert.Less(t, cleared.MaxAge, 0)
	assert.Equal(t, "/api/v1/auth", cleared.Path)
	assert.Equal(t, http.StatusUnauthorized, a.do(http.MethodPost, "/api/v1/auth/refresh", nil, withCookie(c2.Value)).Code)
}

func TestCookieSecureFlagFollowsConfig(t *testing.T) {
	a := newApp(t, func(c *config.Config) { c.CookieSecure = false })
	a.register(t, "dev@example.com")

	c := refreshCookie(a.login(t, "dev@example.com", pass))
	require.NotNil(t, c)
	assert.False(t, c.Secure, "開発(http)用に無効化できる")
	assert.True(t, c.HttpOnly, "HttpOnly は常に有効")
	assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
}

// ---------- 登録 (REQ-001) ----------

func TestRegister_Errors(t *testing.T) {
	a := newApp(t, nil)
	a.register(t, "dup@example.com")

	t.Run("重複 409", func(t *testing.T) {
		rec := a.do(http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "dup@example.com", "password": pass, "name": "n"})
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "email_already_registered", decode(t, rec)["error"])
	})
	t.Run("大文字違いも重複", func(t *testing.T) {
		rec := a.do(http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "DUP@EXAMPLE.COM", "password": pass, "name": "n"})
		assert.Equal(t, http.StatusConflict, rec.Code)
	})
	t.Run("バリデーション 422 とフィールド別詳細", func(t *testing.T) {
		rec := a.do(http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "not-an-email", "password": "short", "name": ""})
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		body := decode(t, rec)
		assert.Equal(t, "validation_failed", body["error"])
		fields := map[string]bool{}
		for _, d := range body["details"].([]any) {
			fields[d.(map[string]any)["field"].(string)] = true
		}
		assert.True(t, fields["email"] && fields["password"] && fields["name"], "%v", fields)
	})
	t.Run("12 文字ちょうどは可・11 文字は不可", func(t *testing.T) {
		assert.Equal(t, http.StatusUnprocessableEntity, a.do(http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "p11@example.com", "password": "abcdefghijk", "name": "n"}).Code)
		assert.Equal(t, http.StatusCreated, a.do(http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "p12@example.com", "password": "abcdefghijkl", "name": "n"}).Code)
	})
	t.Run("72 bytes 超は 422", func(t *testing.T) {
		long := strings.Repeat("あ", 25) // 25 文字 = 75 bytes
		rec := a.do(http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "long@example.com", "password": long, "name": "n"})
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Equal(t, "validation_failed", decode(t, rec)["error"])
	})
	t.Run("壊れた JSON は 400", func(t *testing.T) {
		rec := a.do(http.MethodPost, "/api/v1/auth/register", "{not json")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, "invalid_request", decode(t, rec)["error"])
	})
	t.Run("巨大なボディは 413", func(t *testing.T) {
		rec := a.do(http.MethodPost, "/api/v1/auth/register", `{"name":"`+strings.Repeat("a", 2<<20)+`"}`)
		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	})
}

// ---------- ログイン (REQ-002) ----------

func TestLogin_Failures(t *testing.T) {
	a := newApp(t, nil)
	a.register(t, "user@example.com")

	wrong := a.login(t, "user@example.com", "WrongPassword!!")
	unknown := a.login(t, "nobody@example.com", pass)

	assert.Equal(t, http.StatusUnauthorized, wrong.Code)
	assert.Equal(t, http.StatusUnauthorized, unknown.Code)
	w, u := decode(t, wrong), decode(t, unknown)
	assert.Equal(t, "invalid_credentials", w["error"])
	assert.Equal(t, w["error"], u["error"])
	assert.Equal(t, w["message"], u["message"], "誤パスワードと未登録でメッセージを変えない")
	assert.Nil(t, refreshCookie(wrong), "失敗時に Cookie を設定しない")

	assert.Equal(t, http.StatusUnprocessableEntity, a.do(http.MethodPost, "/api/v1/auth/login", map[string]string{"email": "", "password": ""}).Code)
}

// REQ-002-AC-3（アカウント単位）: 同一アカウントの連続 5 回失敗で 15 分ロック
func TestLogin_AccountLockout(t *testing.T) {
	a := newApp(t, func(c *config.Config) { c.RateLimitLoginMax = 5; c.RateLimitLoginWindow = 15 * time.Minute })
	a.register(t, "user@example.com")
	a.register(t, "other@example.com")

	for i := 1; i <= 5; i++ {
		assert.Equal(t, http.StatusUnauthorized, a.login(t, "user@example.com", "WrongPassword!!").Code, "試行 %d", i)
	}

	// 6 回目は正しいパスワードでもロック。Retry-After と統一エラー形式
	rec := a.login(t, "user@example.com", pass)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "900", rec.Header().Get("Retry-After"))
	body := decode(t, rec)
	assert.Equal(t, "too_many_requests", body["error"])
	assert.NotEmpty(t, body["request_id"])
	assert.Nil(t, refreshCookie(rec))

	// 同じ IP でも別アカウントは影響を受けない（共有 Wi-Fi の別患者を巻き込まない）
	assert.Equal(t, http.StatusOK, a.login(t, "other@example.com", pass).Code)

	// 別 IP や X-Forwarded-For を変えても、そのアカウントはロックされたまま
	assert.Equal(t, http.StatusTooManyRequests, a.do(http.MethodPost, "/api/v1/auth/login", map[string]string{"email": "user@example.com", "password": pass}, withIP("198.51.100.77")).Code)
	spoof := a.do(http.MethodPost, "/api/v1/auth/login", map[string]string{"email": "USER@example.com", "password": pass}, func(r *http.Request) { r.Header.Set("X-Forwarded-For", "192.0.2.1") })
	assert.Equal(t, http.StatusTooManyRequests, spoof.Code, "大文字違いのメールでも同じアカウント")
}

// 未登録メールも同じ回数でロックされ、レスポンスで存在を区別できない
func TestLogin_UnknownAccountLockoutLooksIdentical(t *testing.T) {
	a := newApp(t, func(c *config.Config) { c.RateLimitLoginMax = 5; c.RateLimitLoginWindow = 15 * time.Minute })
	a.register(t, "user@example.com")

	for i := 0; i < 5; i++ {
		a.login(t, "user@example.com", "WrongPassword!!")
		a.login(t, "nobody@example.com", "WrongPassword!!")
	}
	known := a.login(t, "user@example.com", pass)
	unknown := a.login(t, "nobody@example.com", pass)

	assert.Equal(t, http.StatusTooManyRequests, known.Code)
	assert.Equal(t, known.Code, unknown.Code)
	assert.Equal(t, known.Header().Get("Retry-After"), unknown.Header().Get("Retry-After"))
	assert.Equal(t, decode(t, known)["error"], decode(t, unknown)["error"])
}

// 成功するとカウントはリセットされる（連続失敗のみが対象）
func TestLogin_SuccessResetsLockout(t *testing.T) {
	a := newApp(t, func(c *config.Config) { c.RateLimitLoginMax = 5 })
	a.register(t, "user@example.com")

	for round := 0; round < 3; round++ {
		for i := 0; i < 4; i++ {
			require.Equal(t, http.StatusUnauthorized, a.login(t, "user@example.com", "WrongPassword!!").Code)
		}
		require.Equal(t, http.StatusOK, a.login(t, "user@example.com", pass).Code, "round %d", round)
	}
}

// IP 単位は全アカウントをまたぐ緩い天井（パスワードスプレー対策）
func TestLogin_IPCeilingAcrossAccounts(t *testing.T) {
	a := newApp(t, func(c *config.Config) { c.RateLimitLoginIPMax = 3; c.RateLimitLoginWindow = 15 * time.Minute })

	for i := 0; i < 3; i++ {
		assert.Equal(t, http.StatusUnauthorized, a.login(t, "user"+string(rune('a'+i))+"@example.com", "x").Code)
	}
	rec := a.login(t, "userz@example.com", "x")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))

	assert.Equal(t, http.StatusUnauthorized, a.do(http.MethodPost, "/api/v1/auth/login", map[string]string{"email": "usera@example.com", "password": "x"}, withIP("198.51.100.5")).Code, "別 IP は影響なし")
}

// 登録・リセットは IP 単位の天井（メール送信の濫用防止）
func TestSensitiveEndpoints_IPCeiling(t *testing.T) {
	a := newApp(t, func(c *config.Config) { c.RateLimitSensitiveMax = 2 })

	for i := 0; i < 2; i++ {
		assert.Equal(t, http.StatusCreated, a.do(http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "n" + string(rune('a'+i)) + "@example.com", "password": pass, "name": "n"}).Code)
	}
	assert.Equal(t, http.StatusTooManyRequests, a.do(http.MethodPost, "/api/v1/auth/register", map[string]string{"email": "nc@example.com", "password": pass, "name": "n"}).Code)
}

// 同じ宛先へのリセット申請は 1 時間に 3 通まで。応答は変わらない（メール爆撃・存在推測の防止）
func TestPasswordReset_PerAccountThrottle(t *testing.T) {
	a := newApp(t, nil)
	a.register(t, "user@example.com")

	var bodies []string
	for i := 0; i < 5; i++ {
		rec := a.do(http.MethodPost, "/api/v1/auth/password-reset/request", map[string]string{"email": "user@example.com"})
		assert.Equal(t, http.StatusOK, rec.Code)
		bodies = append(bodies, rec.Body.String())
	}
	assert.Len(t, a.mail.resets, 3)
	for _, b := range bodies {
		assert.Equal(t, bodies[0], b, "応答は常に同一")
	}
}

// パスワード再設定に成功するとアカウントのロックが解除される
func TestPasswordReset_UnlocksLockedAccount(t *testing.T) {
	a := newApp(t, func(c *config.Config) { c.RateLimitLoginMax = 3 })
	a.register(t, "user@example.com")
	for i := 0; i < 3; i++ {
		a.login(t, "user@example.com", "WrongPassword!!")
	}
	require.Equal(t, http.StatusTooManyRequests, a.login(t, "user@example.com", pass).Code)

	a.do(http.MethodPost, "/api/v1/auth/password-reset/request", map[string]string{"email": "user@example.com"})
	u, _ := url.Parse(a.mail.resets[0])
	require.Equal(t, http.StatusOK, a.do(http.MethodPost, "/api/v1/auth/password-reset/confirm", map[string]string{"token": u.Query().Get("token"), "new_password": newPass}).Code)

	assert.Equal(t, http.StatusOK, a.login(t, "user@example.com", newPass).Code)
}

// ---------- リフレッシュ / ログアウト (REQ-003, 005) ----------

func TestRefresh_Failures(t *testing.T) {
	a := newApp(t, nil)

	for name, mods := range map[string][]func(*http.Request){
		"Cookie なし": nil,
		"空の Cookie": {withCookie("")},
		"でたらめな値":    {withCookie("garbage")},
	} {
		t.Run(name, func(t *testing.T) {
			rec := a.do(http.MethodPost, "/api/v1/auth/refresh", nil, mods...)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.Equal(t, "invalid_token", decode(t, rec)["error"])
		})
	}
}

func TestLogout_RequiresBearer(t *testing.T) {
	a := newApp(t, nil)
	a.register(t, "user@example.com")
	access := decode(t, a.login(t, "user@example.com", pass))["access_token"].(string)

	assert.Equal(t, http.StatusUnauthorized, a.do(http.MethodPost, "/api/v1/auth/logout", nil).Code)
	assert.Equal(t, http.StatusUnauthorized, a.do(http.MethodPost, "/api/v1/auth/logout", nil, withBearer("garbage")).Code)

	rec := a.do(http.MethodPost, "/api/v1/auth/logout", nil, withBearer(access)) // Cookie なしでも冪等に成功
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Less(t, refreshCookie(rec).MaxAge, 0)
}

// ---------- パスワードリセット (REQ-004) ----------

func TestPasswordReset_FullFlow(t *testing.T) {
	a := newApp(t, nil)
	a.register(t, "user@example.com")
	oldCookie := refreshCookie(a.login(t, "user@example.com", pass))

	// 申請: 登録済み/未登録で同一レスポンス (REQ-004-AC-2)
	known := a.do(http.MethodPost, "/api/v1/auth/password-reset/request", map[string]string{"email": "user@example.com"})
	unknown := a.do(http.MethodPost, "/api/v1/auth/password-reset/request", map[string]string{"email": "nobody@example.com"})
	assert.Equal(t, http.StatusOK, known.Code)
	assert.Equal(t, known.Code, unknown.Code)
	assert.Equal(t, known.Body.String(), unknown.Body.String())
	require.Len(t, a.mail.resets, 1, "メールは登録済みの 1 通のみ")

	// メールの URL からトークンを取り出して確定
	u, err := url.Parse(a.mail.resets[0])
	require.NoError(t, err)
	token := u.Query().Get("token")
	require.NotEmpty(t, token)

	weak := a.do(http.MethodPost, "/api/v1/auth/password-reset/confirm", map[string]string{"token": token, "new_password": "short"})
	assert.Equal(t, http.StatusUnprocessableEntity, weak.Code, "弱いパスワードは拒否")

	ok := a.do(http.MethodPost, "/api/v1/auth/password-reset/confirm", map[string]string{"token": token, "new_password": newPass})
	require.Equal(t, http.StatusOK, ok.Code, ok.Body.String())

	// 新パスワードでログインでき、旧パスワードと旧セッションは無効
	assert.Equal(t, http.StatusOK, a.login(t, "user@example.com", newPass).Code)
	assert.Equal(t, http.StatusUnauthorized, a.login(t, "user@example.com", pass).Code)
	assert.Equal(t, http.StatusUnauthorized, a.do(http.MethodPost, "/api/v1/auth/refresh", nil, withCookie(oldCookie.Value)).Code, "リセットで既存セッションは失効")

	// 再利用は 400 (REQ-004-AC-4)
	again := a.do(http.MethodPost, "/api/v1/auth/password-reset/confirm", map[string]string{"token": token, "new_password": newPass})
	assert.Equal(t, http.StatusBadRequest, again.Code)
	assert.Equal(t, "invalid_reset_token", decode(t, again)["error"])
}

func TestPasswordReset_InvalidInputs(t *testing.T) {
	a := newApp(t, nil)

	assert.Equal(t, http.StatusBadRequest, a.do(http.MethodPost, "/api/v1/auth/password-reset/confirm", map[string]string{"token": "unknown-token", "new_password": newPass}).Code)
	assert.Equal(t, http.StatusUnprocessableEntity, a.do(http.MethodPost, "/api/v1/auth/password-reset/request", map[string]string{"email": "bad"}).Code)
	assert.Equal(t, http.StatusUnprocessableEntity, a.do(http.MethodPost, "/api/v1/auth/password-reset/confirm", map[string]string{"token": "", "new_password": ""}).Code)
}

// 弱いパスワードで失敗してもトークンは消費されず、やり直せる
func TestPasswordReset_WeakPasswordKeepsTokenUsable(t *testing.T) {
	a := newApp(t, nil)
	a.register(t, "user@example.com")
	a.do(http.MethodPost, "/api/v1/auth/password-reset/request", map[string]string{"email": "user@example.com"})
	u, _ := url.Parse(a.mail.resets[0])
	token := u.Query().Get("token")

	assert.Equal(t, http.StatusUnprocessableEntity, a.do(http.MethodPost, "/api/v1/auth/password-reset/confirm", map[string]string{"token": token, "new_password": "tooshort"}).Code)
	assert.Equal(t, http.StatusOK, a.do(http.MethodPost, "/api/v1/auth/password-reset/confirm", map[string]string{"token": token, "new_password": newPass}).Code)
}

// ---------- 横断: エラー形式・秘匿情報・認可連携 ----------

func TestErrorFormatIsUnified(t *testing.T) {
	a := newApp(t, nil)

	rec := a.do(http.MethodGet, "/no/such/route", nil, func(r *http.Request) { r.Header.Set("X-Request-ID", "req-xyz") })

	assert.Equal(t, http.StatusNotFound, rec.Code)
	body := decode(t, rec)
	assert.Equal(t, "not_found", body["error"])
	assert.Equal(t, "req-xyz", body["request_id"])
	assert.Equal(t, "req-xyz", rec.Header().Get("X-Request-ID"))

	rec = a.do(http.MethodGet, "/api/v1/auth/login", nil) // メソッド違い
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, "method_not_allowed", decode(t, rec)["error"])
}

// 応答にもログにも、パスワード・トークン原文・ハッシュが出ない
func TestNoSecretsInResponsesOrLogs(t *testing.T) {
	a := newApp(t, nil)
	a.register(t, "user@example.com")
	rec := a.login(t, "user@example.com", pass)
	access := decode(t, rec)["access_token"].(string)
	refresh := refreshCookie(rec).Value
	a.do(http.MethodPost, "/api/v1/auth/refresh", nil, withCookie(refresh))
	a.do(http.MethodPost, "/api/v1/auth/password-reset/request", map[string]string{"email": "user@example.com"})
	resetURL := a.mail.resets[0]
	token, _ := url.Parse(resetURL)
	a.do(http.MethodPost, "/api/v1/auth/password-reset/confirm", map[string]string{"token": token.Query().Get("token"), "new_password": newPass})

	u, err := a.users.FindByEmail(t.Context(), "user@example.com")
	require.NoError(t, err)
	logs := a.logs.String()
	for name, secret := range map[string]string{
		"password": pass, "new password": newPass, "access token": access, "refresh token": refresh,
		"reset token": token.Query().Get("token"), "password hash": u.PasswordHash,
	} {
		assert.NotContains(t, logs, secret, "%s がログに出ている", name)
	}
	assert.NotContains(t, rec.Body.String(), u.PasswordHash)
}

// アクセストークンのロールが認可ミドルウェアに連携される (clinic-reservation #6)
func TestAccessTokenFeedsRoleAuthorization(t *testing.T) {
	a := newApp(t, nil)
	a.e.GET("/test/staff", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, middleware.JWTAuth(a.tm), middleware.StaffOnly())
	a.register(t, "p@example.com")
	patientTok := decode(t, a.login(t, "p@example.com", pass))["access_token"].(string)

	assert.Equal(t, http.StatusForbidden, a.do(http.MethodGet, "/test/staff", nil, withBearer(patientTok)).Code)

	// DB 上でロールを staff にして再ログインすると通る
	_, err := a.q.CreateUser(t.Context(), db.CreateUserParams{Email: "s@example.com", PasswordHash: mustHash(t, pass), Name: "受付"})
	require.NoError(t, err)
	require.NoError(t, execRole(t, a, "s@example.com", "staff"))
	staffTok := decode(t, a.login(t, "s@example.com", pass))["access_token"].(string)
	assert.Equal(t, http.StatusOK, a.do(http.MethodGet, "/test/staff", nil, withBearer(staffTok)).Code)
}

func TestHealthzStillWorksWithoutAuth(t *testing.T) {
	a := newApp(t, nil)
	rec := a.do(http.MethodGet, "/healthz", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
}

func mustHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	require.NoError(t, err)
	return string(h)
}

func execRole(t *testing.T, a *testApp, email, role string) error {
	t.Helper()
	_, err := a.pool.Exec(context.Background(), "UPDATE users SET role = $2 WHERE email = $1", email, role)
	return err
}
