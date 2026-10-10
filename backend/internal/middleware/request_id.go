// Package middleware は Echo 共通ミドルウェアを提供する。
package middleware

import (
	"context"
	"regexp"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// HeaderRequestID はリクエスト ID を運ぶヘッダー名。
const HeaderRequestID = "X-Request-ID"

type ctxKey struct{}

// 受け入れる外部 ID は英数字・ハイフン・アンダースコアの 64 文字以内（ログインジェクション対策）
var safeRequestID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// RequestID は X-Request-ID を引き継ぐ（安全な値のみ）か新規生成し、レスポンスと context に載せる。
// IMP 3-user-auth-go-infra TASK-0001
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id := c.Request().Header.Get(HeaderRequestID)
			if !safeRequestID.MatchString(id) {
				id = uuid.NewString()
			}
			c.Response().Header().Set(HeaderRequestID, id)
			ctx := context.WithValue(c.Request().Context(), ctxKey{}, id)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// GetRequestID は context からリクエスト ID を取り出す。無ければ空文字。
func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}
