package middleware

import (
	"github.com/labstack/echo/v4"

	"github.com/kava2108/booking/backend/internal/apierror"
)

// writeError は認証・認可ミドルウェアの共通エラー応答。
// RequestID ミドルウェアがあれば request_id が付く(単体では {"error": code} のみ)。
func writeError(c echo.Context, status int, code string) error {
	return apierror.Write(c, status, code, "")
}
