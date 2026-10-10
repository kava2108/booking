package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// RateLimiter は IP ごとのメモリストアによるレートリミットを返す（単一インスタンス前提）。
func RateLimiter(rate limiter.Rate) echo.MiddlewareFunc {
	return NewRateLimiter(memory.NewStore(), rate)
}

// NewRateLimiter はストアを指定できる版。呼び出し側が rate を注入するため、
// 認証用・公開 API 用など別設定のインスタンスを作れる。
// ストア障害時は認証系を無防備にしないよう 503 を返す（フェイルクローズ）。
// IP は c.RealIP() を使うため、サーバー側で IPExtractor を設定すること。
// IMP 3-user-auth-go-infra TASK-0001 / REQ-002-AC-3
func NewRateLimiter(store limiter.Store, rate limiter.Rate) echo.MiddlewareFunc {
	lim := limiter.New(store, rate)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			lc, err := lim.Get(c.Request().Context(), c.RealIP())
			if err != nil {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "rate limiter unavailable")
			}
			if lc.Reached {
				retry := int(time.Until(time.Unix(lc.Reset, 0)).Seconds()) + 1
				if retry < 1 {
					retry = 1
				}
				c.Response().Header().Set("Retry-After", strconv.Itoa(retry))
				return echo.NewHTTPError(http.StatusTooManyRequests, "too many requests")
			}
			return next(c)
		}
	}
}
