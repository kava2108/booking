// Package apierror は API の統一エラー形式 {error, message, request_id, details} を提供する。
// IMP 10-user-auth-auth-handler TASK-0001
package apierror

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

// HeaderRequestID は middleware.HeaderRequestID と同じ値(循環参照を避けるため再定義)。
const headerRequestID = "X-Request-ID"

// Field はフィールド単位のバリデーションエラー。
type Field struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Response はエラー応答の本体。error は機械可読なコード、message は人向けの説明。
type Response struct {
	Error     string  `json:"error"`
	Message   string  `json:"message,omitempty"`
	RequestID string  `json:"request_id,omitempty"`
	Details   []Field `json:"details,omitempty"`
}

// Write はエラー応答を書き込む。request_id は RequestID ミドルウェアが設定したレスポンスヘッダーから取る。
func Write(c echo.Context, status int, code, message string, details ...Field) error {
	return c.JSON(status, Response{
		Error:     code,
		Message:   message,
		RequestID: c.Response().Header().Get(headerRequestID),
		Details:   details,
	})
}

// codes は echo.HTTPError のステータスをエラーコードに対応付ける。
var codes = map[int]string{
	http.StatusBadRequest:            "invalid_request",
	http.StatusUnauthorized:          "unauthorized",
	http.StatusForbidden:             "forbidden",
	http.StatusNotFound:              "not_found",
	http.StatusMethodNotAllowed:      "method_not_allowed",
	http.StatusRequestEntityTooLarge: "payload_too_large",
	http.StatusTooManyRequests:       "too_many_requests",
	http.StatusServiceUnavailable:    "service_unavailable",
}

// Handler は Echo の HTTPErrorHandler。すべてのエラーを統一形式に変換する。
// 5xx と未知のエラーは内部情報を応答に含めず、汎用メッセージにする(詳細は Logger がログに出す)。
func Handler() echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		status, code, msg := http.StatusInternalServerError, "internal_error", "internal server error"

		var he *echo.HTTPError
		if errors.As(err, &he) {
			status = he.Code
			if status < 500 {
				if cc, ok := codes[status]; ok {
					code = cc
				} else {
					code = "error"
				}
				msg = http.StatusText(status)
			} else if cc, ok := codes[status]; ok {
				code, msg = cc, http.StatusText(status)
			}
		}
		if c.Request().Method == http.MethodHead {
			_ = c.NoContent(status)
			return
		}
		_ = Write(c, status, code, msg)
	}
}
