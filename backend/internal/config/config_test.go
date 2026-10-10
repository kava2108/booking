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
