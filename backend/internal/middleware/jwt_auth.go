package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/kava2108/booking/backend/internal/auth"
)

// JWTAuth は Authorization: Bearer のアクセストークンを検証し、Identity を設定する。
// 検証は auth.TokenManager に一本化している(HS256 固定・exp 必須・issuer/sub/role 検証)。
// 失敗はすべて 401(理由を区別しない)。IMP 10-user-auth-auth-handler TASK-0001
func JWTAuth(tm *auth.TokenManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token, ok := bearerToken(c.Request().Header.Get(echo.HeaderAuthorization))
			if !ok {
				return unauthorized(c)
			}
			id, err := tm.ParseAccess(token)
			if err != nil {
				return unauthorized(c)
			}
			SetIdentity(c, Identity{UserID: id.UserID, Role: id.Role})
			return next(c)
		}
	}
}

func unauthorized(c echo.Context) error {
	c.Response().Header().Set("WWW-Authenticate", "Bearer")
	return writeError(c, http.StatusUnauthorized, "unauthorized")
}

// bearerToken は "Bearer <token>" からトークンを取り出す(スキーム名は大文字小文字を区別しない)。
func bearerToken(h string) (string, bool) {
	parts := strings.Fields(h)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return parts[1], true
}
