package service

import "time"

// ClinicOption は clinic-reservation の service 共通の設定（時刻の差し替えなど）。
type ClinicOption func(*clinicOptions)

type clinicOptions struct {
	now     func() time.Time
	numbers func(slotStart time.Time) string // 予約番号の生成（テスト用に差し替え可能）
}

// WithClinicClock はテスト用に現在時刻を差し替える。
func WithClinicClock(now func() time.Time) ClinicOption {
	return func(o *clinicOptions) { o.now = now }
}

func newClinicOptions(opts []ClinicOption) clinicOptions {
	o := clinicOptions{now: time.Now}
	for _, f := range opts {
		f(&o)
	}
	return o
}

// FieldError はフィールド単位の入力エラー。
type FieldError struct{ Field, Message string }

// ValidationError は入力検証・業務ルールのうち「入力の誤り」に当たるもの（HTTP 422 に変換される）。
type ValidationError struct{ Fields []FieldError }

func (e *ValidationError) Error() string { return "validation failed" }

func validationError(field, msg string) *ValidationError {
	return &ValidationError{Fields: []FieldError{{Field: field, Message: msg}}}
}
