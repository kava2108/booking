package middleware

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"

	"github.com/kava2108/booking/backend/internal/repository/db"
)

// AuditRecorder は監査ログの書き込み先。repository.AdminRepository が満たす。
type AuditRecorder interface {
	InsertAuditLog(ctx context.Context, arg db.InsertAuditLogParams) error
}

// AuditLog は個人情報にアクセスするハンドラーの前に actor / action / target を記録する。
//   - ハンドラーの前に記録し、書き込みに失敗したら 500 を返してハンドラーを実行しない（フェイルクローズ）。
//   - 記録するのは対象の識別子とアクションのみ。クエリ文字列・ボディ（症状メモ等）は記録しない。
//   - target が nil のときはクエリ文字列を除いたリクエストパスを使う。
//
// IMP 6-clinic-reservation-rbac-audit-openapi TASK-0001 / REQ-007-AC-3
func AuditLog(rec AuditRecorder, action string, target func(echo.Context) string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id, ok := IdentityFrom(c)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}
			t := c.Request().URL.Path
			if target != nil {
				t = target(c)
			}
			err := rec.InsertAuditLog(c.Request().Context(), db.InsertAuditLogParams{
				ActorID: pgtype.UUID{Bytes: id.UserID, Valid: true},
				Action:  action,
				Target:  t,
			})
			if err != nil {
				c.Logger().Errorf("audit log failed: request_id=%s action=%s err=%v", GetRequestID(c.Request().Context()), action, err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal"})
			}
			return next(c)
		}
	}
}
