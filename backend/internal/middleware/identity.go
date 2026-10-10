package middleware

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/kava2108/booking/backend/internal/domain"
)

// Identity は認証済みユーザー。JWT 検証ミドルウェア（user-auth の jwt_auth）が SetIdentity で設定し、
// 認可・監査ミドルウェアが IdentityFrom で参照する。
type Identity struct {
	UserID uuid.UUID
	Role   domain.Role
}

const identityKey = "auth.identity"

func SetIdentity(c echo.Context, id Identity) { c.Set(identityKey, id) }

func IdentityFrom(c echo.Context) (Identity, bool) {
	id, ok := c.Get(identityKey).(Identity)
	return id, ok
}
