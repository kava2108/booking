package middleware

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

// Logger はリクエストごとの構造化ログを出力する。request_id を必ず含める。
// ボディ・ヘッダー・クエリ文字列は出力しない（トークン・パスワード漏えい防止）。
// IMP 3-user-auth-go-infra TASK-0001
func Logger(l zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			if err != nil {
				c.Error(err) // 最終ステータスを確定させる
			}

			status := c.Response().Status
			var he *echo.HTTPError
			if errors.As(err, &he) {
				status = he.Code
			}

			ev := l.Info()
			switch {
			case status >= http.StatusInternalServerError:
				ev = l.Error()
			case status >= http.StatusBadRequest:
				ev = l.Warn()
			}
			if err != nil && status >= http.StatusInternalServerError {
				ev = ev.Err(err) // 5xx の原因(応答には出さない)
			}
			ev.Str("request_id", GetRequestID(c.Request().Context())).
				Str("method", c.Request().Method).
				Str("path", c.Request().URL.Path).
				Int("status", status).
				Int64("latency_ms", time.Since(start).Milliseconds()).
				Str("remote_ip", c.RealIP()).
				Msg("request")
			return nil
		}
	}
}
