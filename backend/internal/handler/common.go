package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/kava2108/booking/backend/internal/apierror"
	"github.com/kava2108/booking/backend/internal/service"
)

// serviceError は service の入力エラー(ValidationError)を 422 に変換する。それ以外はそのまま返す(Handler が 5xx/既定の形式にする)。
func serviceError(c echo.Context, err error) error {
	var ve *service.ValidationError
	if errors.As(err, &ve) {
		fields := make([]apierror.Field, len(ve.Fields))
		for i, f := range ve.Fields {
			fields[i] = apierror.Field{Field: f.Field, Message: f.Message}
		}
		return apierror.Write(c, http.StatusUnprocessableEntity, "validation_failed", "validation failed", fields...)
	}
	return err
}

func validationFailed(c echo.Context, fields ...apierror.Field) error {
	return apierror.Write(c, http.StatusUnprocessableEntity, "validation_failed", "validation failed", fields...)
}

// queryInt64 は正の整数のクエリパラメータを読む。required で未指定、または不正ならエラーを返す。
func queryInt64(c echo.Context, name string, required bool) (value int64, present bool, bad *apierror.Field) {
	raw := c.QueryParam(name)
	if raw == "" {
		if required {
			return 0, false, &apierror.Field{Field: name, Message: "is required"}
		}
		return 0, false, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return 0, false, &apierror.Field{Field: name, Message: "must be a positive integer"}
	}
	return v, true, nil
}
