package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ulule/limiter/v3"
)

func hit(e *echo.Echo, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = ip + ":40000"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func newLimited(mw echo.MiddlewareFunc) *echo.Echo {
	e := echo.New()
	e.IPExtractor = echo.ExtractIPDirect()
	e.POST("/login", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, mw)
	return e
}

// IMP 3-user-auth-go-infra TASK-0001 / REQ-002-AC-3: 5 回目は通過、6 回目で 429
func TestRateLimiter_BlocksAfterLimit(t *testing.T) {
	e := newLimited(RateLimiter(limiter.Rate{Limit: 5, Period: 15 * time.Minute}))

	for i := 1; i <= 5; i++ {
		assert.Equal(t, http.StatusOK, hit(e, "203.0.113.1").Code, "request %d", i)
	}
	rec := hit(e, "203.0.113.1")
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}

// IP ごとに独立してカウントする
func TestRateLimiter_PerIP(t *testing.T) {
	e := newLimited(RateLimiter(limiter.Rate{Limit: 1, Period: time.Minute}))

	assert.Equal(t, http.StatusOK, hit(e, "203.0.113.1").Code)
	assert.Equal(t, http.StatusTooManyRequests, hit(e, "203.0.113.1").Code)
	assert.Equal(t, http.StatusOK, hit(e, "203.0.113.2").Code)
}

// 境界値: 期間経過後に解除される
func TestRateLimiter_ResetsAfterPeriod(t *testing.T) {
	e := newLimited(RateLimiter(limiter.Rate{Limit: 1, Period: 200 * time.Millisecond}))

	require.Equal(t, http.StatusOK, hit(e, "203.0.113.1").Code)
	require.Equal(t, http.StatusTooManyRequests, hit(e, "203.0.113.1").Code)
	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, http.StatusOK, hit(e, "203.0.113.1").Code)
}

// セキュリティ: X-Forwarded-For を偽装してもバイパスできない（ExtractIPDirect 前提）
func TestRateLimiter_IgnoresSpoofedForwardedFor(t *testing.T) {
	e := newLimited(RateLimiter(limiter.Rate{Limit: 1, Period: time.Minute}))

	do := func(xff string) int {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "203.0.113.9:1"
		req.Header.Set("X-Forwarded-For", xff)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}
	assert.Equal(t, http.StatusOK, do("1.1.1.1"))
	assert.Equal(t, http.StatusTooManyRequests, do("2.2.2.2"))
}

type failingStore struct{}

func (failingStore) Get(context.Context, string, limiter.Rate) (limiter.Context, error) {
	return limiter.Context{}, errors.New("store down")
}
func (failingStore) Peek(context.Context, string, limiter.Rate) (limiter.Context, error) {
	return limiter.Context{}, errors.New("store down")
}
func (failingStore) Reset(context.Context, string, limiter.Rate) (limiter.Context, error) {
	return limiter.Context{}, errors.New("store down")
}
func (failingStore) Increment(context.Context, string, int64, limiter.Rate) (limiter.Context, error) {
	return limiter.Context{}, errors.New("store down")
}

// ストア障害時は認証系のためフェイルクローズ（503）
func TestRateLimiter_FailsClosedOnStoreError(t *testing.T) {
	e := newLimited(NewRateLimiter(failingStore{}, limiter.Rate{Limit: 5, Period: time.Minute}))

	assert.Equal(t, http.StatusServiceUnavailable, hit(e, "203.0.113.1").Code)
}
