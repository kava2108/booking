// Package config は環境変数から設定を読み込む。
// IMP 3-user-auth-go-infra TASK-0001
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
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

	// ログイン制限（REQ-002-AC-3）。アカウント(メール)単位で「連続 RateLimitLoginMax 回の失敗で RateLimitLoginWindow ロック」。
	RateLimitLoginMax    int
	RateLimitLoginWindow time.Duration
	// RateLimitLoginIPMax は IP 単位の天井（全アカウントをまたぐ試行回数 / RateLimitLoginWindow）。
	// パスワードスプレー対策であり、共有 Wi-Fi の患者を巻き込まないよう緩くする。
	RateLimitLoginIPMax int
	// RateLimitSensitiveMax は登録・パスワードリセット申請/確定の IP 単位の上限（RateLimitLoginWindow あたり）。
	RateLimitSensitiveMax int

	// 公開 API（待ち時間など未ログインで呼べるもの）向け
	RateLimitPublicMax    int
	RateLimitPublicWindow time.Duration

	// CookieSecure は Cookie の Secure 属性。本番は true。http の開発環境のみ false にできる。
	CookieSecure bool
	// TrustedProxies は X-Forwarded-For を信用するプロキシの CIDR。空なら接続元 IP をそのまま使う。
	TrustedProxies []*net.IPNet

	// AppBaseURL はフロントエンドの URL(パスワードリセットのリンクに使う)。
	AppBaseURL string
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
	v.SetDefault("RATE_LIMIT_LOGIN_IP_MAX", 100)
	v.SetDefault("RATE_LIMIT_SENSITIVE_MAX", 20)
	v.SetDefault("RATE_LIMIT_PUBLIC_MAX", 60)
	v.SetDefault("RATE_LIMIT_PUBLIC_WINDOW", "1m")
	v.SetDefault("COOKIE_SECURE", "true")
	v.SetDefault("APP_BASE_URL", "http://localhost:5173")

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

	boolean := func(key string) bool {
		b, err := strconv.ParseBool(v.GetString(key))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s must be true or false", key))
		}
		return b
	}
	httpURL := func(key string) string {
		raw := v.GetString(key)
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, fmt.Errorf("%s must be an http(s) URL", key))
		}
		return raw
	}

	var trusted []*net.IPNet
	for _, raw := range strings.Split(v.GetString("TRUSTED_PROXIES"), ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, n, err := net.ParseCIDR(raw)
		if err != nil {
			errs = append(errs, errors.New("TRUSTED_PROXIES must be a comma-separated list of CIDRs"))
			break
		}
		trusted = append(trusted, n)
	}

	cfg := &Config{
		ServerPort:            positive("SERVER_PORT"),
		DBDSN:                 required("DB_DSN"),
		DBMaxConns:            int32(positive("DB_MAX_CONNS")),
		JWTSecret:             required("JWT_SECRET"),
		JWTRefreshSecret:      required("JWT_REFRESH_SECRET"),
		JWTAccessTTL:          duration("JWT_ACCESS_TTL"),
		JWTRefreshTTL:         duration("JWT_REFRESH_TTL"),
		SMTPHost:              v.GetString("SMTP_HOST"),
		SMTPPort:              positive("SMTP_PORT"),
		SMTPUser:              v.GetString("SMTP_USER"),
		SMTPPassword:          v.GetString("SMTP_PASSWORD"),
		SMTPFrom:              v.GetString("SMTP_FROM"),
		RateLimitLoginMax:     positive("RATE_LIMIT_LOGIN_MAX"),
		RateLimitLoginWindow:  duration("RATE_LIMIT_LOGIN_WINDOW"),
		RateLimitLoginIPMax:   positive("RATE_LIMIT_LOGIN_IP_MAX"),
		RateLimitSensitiveMax: positive("RATE_LIMIT_SENSITIVE_MAX"),

		RateLimitPublicMax:    positive("RATE_LIMIT_PUBLIC_MAX"),
		RateLimitPublicWindow: duration("RATE_LIMIT_PUBLIC_WINDOW"),

		CookieSecure:   boolean("COOKIE_SECURE"),
		AppBaseURL:     httpURL("APP_BASE_URL"),
		TrustedProxies: trusted,
	}
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}
