package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/app"
	"github.com/kava2108/booking/backend/internal/auth"
	"github.com/kava2108/booking/backend/internal/config"
	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/service"
	"github.com/kava2108/booking/backend/internal/testutil/pgtest"
)

// clinic-reservation の HTTP 統合テスト用の環境（実 DB・固定時計・OpenAPI 契約チェック）。

// 基準時刻: 2026-10-12(月) 09:00 JST
var clinicNow = time.Date(2026, 10, 12, 9, 0, 0, 0, domain.JST)

type clinicApp struct {
	t      *testing.T
	e      *echo.Echo
	pool   *pgxpool.Pool
	tm     *auth.TokenManager
	logs   *syncBuffer
	clock  *time.Time
	router routers.Router

	deptID, doctorID int64
	staffUser        *db.User
}

func newClinicApp(t *testing.T, mutate func(*config.Config)) *clinicApp {
	t.Helper()
	pool := pgtest.NewPool(t)
	store := repository.NewStore(pool)
	tm, err := auth.NewTokenManager(secret, 15*time.Minute)
	require.NoError(t, err)
	clock := clinicNow
	now := func() time.Time { return clock }
	logs := &syncBuffer{}

	cfg := &config.Config{
		RateLimitLoginMax: 1000, RateLimitLoginIPMax: 1000, RateLimitSensitiveMax: 1000, RateLimitLoginWindow: time.Minute,
		RateLimitPublicMax: 1000, RateLimitPublicWindow: time.Minute, CookieSecure: true,
	}
	if mutate != nil {
		mutate(cfg)
	}
	repos := store.Repositories()
	slots := service.NewSlotService(repos, store, service.WithClinicClock(now))
	admin := service.NewAdminService(repos, store, slots, service.WithClinicClock(now))
	appts := service.NewAppointmentService(repos, store, service.WithClinicClock(now))
	queue := service.NewQueueService(repos, store, service.WithClinicClock(now))

	a := &clinicApp{t: t, pool: pool, tm: tm, logs: logs, clock: &clock}
	a.e = app.New(app.Deps{Config: cfg, Logger: zerolog.New(logs), Tokens: tm, Slots: slots, Admin: admin, Appointments: appts, Queue: queue, Audit: repos.Admin})
	a.router = loadRouter(t)

	require.NoError(t, pool.QueryRow(context.Background(), `INSERT INTO departments(name, standard_consult_minutes) VALUES ('内科', 10) RETURNING id`).Scan(&a.deptID))
	require.NoError(t, pool.QueryRow(context.Background(), `INSERT INTO doctors(department_id, name) VALUES ($1, '山田') RETURNING id`, a.deptID).Scan(&a.doctorID))
	return a
}

func loadRouter(t *testing.T) routers.Router {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)
	r, err := legacy.NewRouter(doc)
	require.NoError(t, err)
	return r
}

// token は指定ロールのアクセストークンを発行する（DB 上のユーザーは不要なエンドポイント用）。
func (a *clinicApp) token(role domain.Role) string {
	tok, _, err := a.tm.IssueAccess(uuid.New(), role)
	require.NoError(a.t, err)
	return tok
}

// tokenFor は実在ユーザー（FK が必要な予約など）のトークンを発行する。
func (a *clinicApp) tokenFor(userID uuid.UUID, role domain.Role) string {
	tok, _, err := a.tm.IssueAccess(userID, role)
	require.NoError(a.t, err)
	return tok
}

func (a *clinicApp) newUser(email string, role domain.Role) db.User {
	a.t.Helper()
	q := db.New(a.pool)
	u, err := q.CreateUser(context.Background(), db.CreateUserParams{Email: email, PasswordHash: "h", Name: "患者 " + email})
	require.NoError(a.t, err)
	if role != domain.RolePatient {
		_, err = a.pool.Exec(context.Background(), "UPDATE users SET role=$2 WHERE id=$1", u.ID, string(role))
		require.NoError(a.t, err)
	}
	return u
}

// call はリクエストを実行し、レスポンスを OpenAPI 定義と突き合わせて返す（契約テスト）。
func (a *clinicApp) call(method, path string, body any, token string) *httptest.ResponseRecorder {
	a.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = bytes.NewReader([]byte(b))
	default:
		j, err := json.Marshal(b)
		require.NoError(a.t, err)
		rd = bytes.NewReader(j)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.20:4000"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.e.ServeHTTP(rec, req)
	a.assertConforms(req, rec)
	return rec
}

// assertConforms: レスポンスが OpenAPI のスキーマに合致する（定義にない 404 ルートなどは対象外）。
func (a *clinicApp) assertConforms(req *http.Request, rec *httptest.ResponseRecorder) {
	a.t.Helper()
	route, params, err := a.router.FindRoute(req)
	if err != nil {
		return // OpenAPI に無いパス
	}
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request: req, PathParams: params, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		},
		Status:  rec.Code,
		Header:  rec.Header(),
		Body:    io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
		Options: &openapi3filter.Options{IncludeResponseStatus: true},
	}
	if err := openapi3filter.ValidateResponse(context.Background(), in); err != nil {
		a.t.Errorf("OpenAPI 契約違反 %s %s -> %d: %v\nbody: %s", req.Method, req.URL.Path, rec.Code, err, rec.Body.String())
	}
}

func jsonBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &v), rec.Body.String())
	return v
}

// day は 2026 年 10 月 d 日 h:m (JST)。
func day(d, h, m int) time.Time { return time.Date(2026, 10, d, h, m, 0, 0, domain.JST) }

// syncBuffer は並行リクエストのログ書き込みで race を起こさないテスト用バッファ。
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}
func (b *syncBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.buf.String() }

// callQuiet は並行テスト用。ゴルーチン内で require を使わず、開始合図を待ってからリクエストする。
func (a *clinicApp) callQuiet(method, path string, body any, token string, start <-chan struct{}) *httptest.ResponseRecorder {
	j, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(j))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.RemoteAddr = "203.0.113.40:1"
	rec := httptest.NewRecorder()
	<-start
	a.e.ServeHTTP(rec, req)
	return rec
}
