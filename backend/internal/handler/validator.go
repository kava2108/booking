package handler

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/kava2108/booking/backend/internal/apierror"
)

// ValidationError は入力検証の失敗(フィールド別)。HTTP 422 に変換される。
type ValidationError struct{ Fields []apierror.Field }

func (e *ValidationError) Error() string { return "validation failed" }

// Validator は echo.Validator の実装。フィールド名は json タグを使う。
type Validator struct{ v *validator.Validate }

func NewValidator() *Validator {
	v := validator.New()
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return f.Name
		}
		return name
	})
	return &Validator{v: v}
}

func (val *Validator) Validate(i any) error {
	err := val.v.Struct(i)
	if err == nil {
		return nil
	}
	verrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return err
	}
	out := &ValidationError{}
	for _, fe := range verrs {
		out.Fields = append(out.Fields, apierror.Field{Field: fieldPath(fe), Message: message(fe)})
	}
	return out
}

func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return fmt.Sprintf("must be at least %s characters", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s characters", fe.Param())
	default:
		return "is invalid"
	}
}

// fieldPath は "structName.schedules[0].weekday" から先頭の構造体名を除いた JSON 上のパスを返す。
func fieldPath(fe validator.FieldError) string {
	ns := fe.Namespace()
	if i := strings.Index(ns, "."); i >= 0 {
		return ns[i+1:]
	}
	return fe.Field()
}
