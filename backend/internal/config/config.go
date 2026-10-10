// Package config は環境変数から設定を読み込む。
// IMP 3-user-auth-go-infra TASK-0001
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config はアプリ全体の設定。シークレットは環境変数のみから取得し、デフォルト値を持たない。
type Config struct {
	ServerPort int

	DBDSN      string
	DBMaxConns int32

	JWTSecret        string
	JWTRefreshSecret string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration

	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	// 認証エンドポイント向け（REQ-002-AC-3: 5 回 / 15 分）
	RateLimitLoginMax    int
	RateLimitLoginWindow time.Duration

	// 公開 API（待ち時間など未ログインで呼べるもの）向け
	RateLimitPublicMax    int
	RateLimitPublicWindow time.Duration
}

// Load は環境変数を読み込んで検証する。エラーメッセージにはキー名のみ含め、値は含めない。
func Load() (*Config, error) {
	v := viper.New()
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetDefault("SERVER_PORT", 8080)
	v.SetDefault("DB_MAX_CONNS", 10)
	v.SetDefault("JWT_ACCESS_TTL", "15m")
	v.SetDefault("JWT_REFRESH_TTL", "168h")
	v.SetDefault("SMTP_PORT", 587)
	v.SetDefault("RATE_LIMIT_LOGIN_MAX", 5)
	v.SetDefault("RATE_LIMIT_LOGIN_WINDOW", "15m")
	v.SetDefault("RATE_LIMIT_PUBLIC_MAX", 60)
	v.SetDefault("RATE_LIMIT_PUBLIC_WINDOW", "1m")

	var errs []error
	required := func(key string) string {
		s := v.GetString(key)
		if s == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
		return s
	}
	duration := func(key string) time.Duration {
		d, err := time.ParseDuration(v.GetString(key))
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("%s must be a positive duration", key))
		}
		return d
	}
	positive := func(key string) int {
		n := v.GetInt(key)
		if n <= 0 {
			errs = append(errs, fmt.Errorf("%s must be a positive integer", key))
		}
		return n
	}

	cfg := &Config{
		ServerPort:           positive("SERVER_PORT"),
		DBDSN:                required("DB_DSN"),
		DBMaxConns:           int32(positive("DB_MAX_CONNS")),
		JWTSecret:            required("JWT_SECRET"),
		JWTRefreshSecret:     required("JWT_REFRESH_SECRET"),
		JWTAccessTTL:         duration("JWT_ACCESS_TTL"),
		JWTRefreshTTL:        duration("JWT_REFRESH_TTL"),
		SMTPHost:             v.GetString("SMTP_HOST"),
		SMTPPort:             positive("SMTP_PORT"),
		SMTPUser:             v.GetString("SMTP_USER"),
		SMTPPassword:         v.GetString("SMTP_PASSWORD"),
		SMTPFrom:             v.GetString("SMTP_FROM"),
		RateLimitLoginMax:    positive("RATE_LIMIT_LOGIN_MAX"),
		RateLimitLoginWindow: duration("RATE_LIMIT_LOGIN_WINDOW"),

		RateLimitPublicMax:    positive("RATE_LIMIT_PUBLIC_MAX"),
		RateLimitPublicWindow: duration("RATE_LIMIT_PUBLIC_WINDOW"),
	}
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}
