package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DB_DSN", "postgres://app@localhost:5432/booking")
	t.Setenv("JWT_SECRET", "access-secret")
	t.Setenv("JWT_REFRESH_SECRET", "refresh-secret")
}

// IMP 3-user-auth-go-infra TASK-0001: 必須の環境変数が揃えばデフォルト値付きで読み込める
func TestLoad_Defaults(t *testing.T) {
	setRequired(t)

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "postgres://app@localhost:5432/booking", cfg.DBDSN)
	assert.Equal(t, 8080, cfg.ServerPort)
	assert.Equal(t, int32(10), cfg.DBMaxConns)
	assert.Equal(t, 15*time.Minute, cfg.JWTAccessTTL)
	assert.Equal(t, 7*24*time.Hour, cfg.JWTRefreshTTL)
	assert.Equal(t, 5, cfg.RateLimitLoginMax)
	assert.Equal(t, 15*time.Minute, cfg.RateLimitLoginWindow)
	assert.Equal(t, 100, cfg.RateLimitLoginIPMax, "IP 単位の上限は緩い天井（共有 Wi-Fi を巻き込まない）")
	assert.Equal(t, 20, cfg.RateLimitSensitiveMax)
	assert.Equal(t, 60, cfg.RateLimitPublicMax)
	assert.Equal(t, time.Minute, cfg.RateLimitPublicWindow)
}

func TestLoad_Overrides(t *testing.T) {
	setRequired(t)
	t.Setenv("SERVER_PORT", "9000")
	t.Setenv("DB_MAX_CONNS", "25")
	t.Setenv("JWT_ACCESS_TTL", "5m")
	t.Setenv("RATE_LIMIT_LOGIN_MAX", "3")
	t.Setenv("RATE_LIMIT_LOGIN_WINDOW", "1h")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "587")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 9000, cfg.ServerPort)
	assert.Equal(t, int32(25), cfg.DBMaxConns)
	assert.Equal(t, 5*time.Minute, cfg.JWTAccessTTL)
	assert.Equal(t, 3, cfg.RateLimitLoginMax)
	assert.Equal(t, time.Hour, cfg.RateLimitLoginWindow)
	assert.Equal(t, "smtp.example.com", cfg.SMTPHost)
	assert.Equal(t, 587, cfg.SMTPPort)
}

// 異常系: 必須項目が欠けたら起動失敗し、欠けたキー名を示す（値は出さない）
func TestLoad_MissingRequired(t *testing.T) {
	for _, key := range []string{"DB_DSN", "JWT_SECRET", "JWT_REFRESH_SECRET"} {
		t.Run(key, func(t *testing.T) {
			setRequired(t)
			t.Setenv(key, "")

			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), key)
			assert.NotContains(t, err.Error(), "access-secret")
		})
	}
}

// 境界値: 不正な値は握りつぶさずエラーにする
func TestLoad_InvalidValues(t *testing.T) {
	setRequired(t)
	t.Setenv("JWT_ACCESS_TTL", "not-a-duration")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_ACCESS_TTL")
}

func TestLoad_InvalidRateLimit(t *testing.T) {
	setRequired(t)
	t.Setenv("RATE_LIMIT_LOGIN_MAX", "0")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RATE_LIMIT_LOGIN_MAX")
}

// IMP 6-clinic-reservation-rbac-audit-openapi: 公開 API（待ち時間など）用のレートリミット設定
func TestLoad_PublicRateLimitOverrides(t *testing.T) {
	setRequired(t)
	t.Setenv("RATE_LIMIT_PUBLIC_MAX", "120")
	t.Setenv("RATE_LIMIT_PUBLIC_WINDOW", "30s")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 120, cfg.RateLimitPublicMax)
	assert.Equal(t, 30*time.Second, cfg.RateLimitPublicWindow)
}

func TestLoad_InvalidPublicRateLimit(t *testing.T) {
	setRequired(t)
	t.Setenv("RATE_LIMIT_PUBLIC_MAX", "-1")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RATE_LIMIT_PUBLIC_MAX")
}

// IMP 10-user-auth-auth-handler: Cookie と フロント URL の設定
func TestLoad_CookieAndBaseURL(t *testing.T) {
	setRequired(t)
	cfg, err := Load()
	require.NoError(t, err)
	assert.True(t, cfg.CookieSecure, "既定で Secure Cookie")
	assert.Equal(t, "http://localhost:5173", cfg.AppBaseURL)

	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("APP_BASE_URL", "https://app.example.com")
	cfg, err = Load()
	require.NoError(t, err)
	assert.False(t, cfg.CookieSecure)
	assert.Equal(t, "https://app.example.com", cfg.AppBaseURL)
}

func TestLoad_InvalidCookieAndBaseURL(t *testing.T) {
	for key, val := range map[string]string{
		"COOKIE_SECURE": "maybe",
		"APP_BASE_URL":  "javascript:alert(1)",
	} {
		t.Run(key, func(t *testing.T) {
			setRequired(t)
			t.Setenv(key, val)
			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), key)
		})
	}
}

// IMP 10: ログイン制限はアカウント単位（RATE_LIMIT_LOGIN_MAX = 連続失敗回数）、IP 単位は別の天井
func TestLoad_LoginLimitOverrides(t *testing.T) {
	setRequired(t)
	t.Setenv("RATE_LIMIT_LOGIN_IP_MAX", "300")
	t.Setenv("RATE_LIMIT_SENSITIVE_MAX", "50")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 300, cfg.RateLimitLoginIPMax)
	assert.Equal(t, 50, cfg.RateLimitSensitiveMax)
}

func TestLoad_InvalidLoginIPLimit(t *testing.T) {
	for _, key := range []string{"RATE_LIMIT_LOGIN_IP_MAX", "RATE_LIMIT_SENSITIVE_MAX"} {
		t.Run(key, func(t *testing.T) {
			setRequired(t)
			t.Setenv(key, "0")
			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), key)
		})
	}
}

func TestLoad_TrustedProxies(t *testing.T) {
	setRequired(t)
	cfg, err := Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.TrustedProxies, "既定は誰も信用しない")

	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8, 172.16.0.0/12")
	cfg, err = Load()
	require.NoError(t, err)
	assert.Len(t, cfg.TrustedProxies, 2)

	t.Setenv("TRUSTED_PROXIES", "not-a-cidr")
	_, err = Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TRUSTED_PROXIES")
}
