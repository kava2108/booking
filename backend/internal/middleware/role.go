package middleware

import (
	"net/http"
	"slices"

	"github.com/labstack/echo/v4"

	"github.com/kava2108/booking/backend/internal/domain"
)

// RequireRole は許可ロールのいずれかを持つユーザーだけ通す。
// 認証情報が無ければ 401、ロール不足なら 403。引数なしは誰も通さない（設定ミスで全開放にならない）。
// 認可はこのミドルウェアに一本化し、ハンドラー内で role を手書き判定しない。
// IMP 6-clinic-reservation-rbac-audit-openapi TASK-0001 / REQ-005-AC-4
func RequireRole(allowed ...domain.Role) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id, ok := IdentityFrom(c)
			if !ok {
				return writeError(c, http.StatusUnauthorized, "unauthorized")
			}
			if !id.Role.Valid() || !slices.Contains(allowed, id.Role) {
				return writeError(c, http.StatusForbidden, "forbidden")
			}
			return next(c)
		}
	}
}

// StaffOnly は /api/v1/staff/** 用。受付スタッフとクリニック管理者を許可する。
func StaffOnly() echo.MiddlewareFunc {
	return RequireRole(domain.RoleStaff, domain.RoleClinicAdmin)
}

// AdminOnly は /api/v1/admin/** 用。クリニック管理者のみ許可する。
func AdminOnly() echo.MiddlewareFunc {
	return RequireRole(domain.RoleClinicAdmin)
}
