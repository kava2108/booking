package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runLogged(t *testing.T, req *http.Request, h echo.HandlerFunc) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	e := echo.New()
	e.Use(RequestID(), Logger(zerolog.New(&buf)))
	e.Any("/*", h)
	e.ServeHTTP(httptest.NewRecorder(), req)

	var out map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &out), buf.String())
	return out
}

// IMP 3-user-auth-go-infra TASK-0001: JSON ログに request_id を必ず含める
func TestLogger_IncludesRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"super-secret"}`))
	req.Header.Set(HeaderRequestID, "req-abc")
	req.Header.Set("Authorization", "Bearer secret-token")

	out := runLogged(t, req, func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	assert.Equal(t, "req-abc", out["request_id"])
	assert.Equal(t, "POST", out["method"])
	assert.Equal(t, "/api/v1/auth/login", out["path"])
	assert.EqualValues(t, 200, out["status"])
	assert.Contains(t, out, "latency_ms")
}

// セキュリティ: ボディ・Authorization・クエリ文字列（トークンが載り得る）をログに出さない
func TestLogger_DoesNotLeakSecrets(t *testing.T) {
	var buf bytes.Buffer
	e := echo.New()
	e.Use(RequestID(), Logger(zerolog.New(&buf)))
	e.Any("/*", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset?token=reset-token-xyz", strings.NewReader(`{"password":"super-secret"}`))
	req.Header.Set("Authorization", "Bearer secret-token")
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "cookie-secret"})
	e.ServeHTTP(httptest.NewRecorder(), req)

	for _, s := range []string{"super-secret", "secret-token", "cookie-secret", "reset-token-xyz"} {
		assert.NotContains(t, buf.String(), s)
	}
}

func TestLogger_ErrorLevelOnServerError(t *testing.T) {
	out := runLogged(t, httptest.NewRequest(http.MethodGet, "/boom", nil),
		func(c echo.Context) error { return c.NoContent(http.StatusInternalServerError) })
	assert.Equal(t, "error", out["level"])
}

func TestLogger_WarnOnClientError(t *testing.T) {
	out := runLogged(t, httptest.NewRequest(http.MethodGet, "/x", nil),
		func(c echo.Context) error { return c.NoContent(http.StatusUnauthorized) })
	assert.Equal(t, "warn", out["level"])
}

// ハンドラーが error を返した場合も、最終ステータスでログされる
func TestLogger_HandlerError(t *testing.T) {
	out := runLogged(t, httptest.NewRequest(http.MethodGet, "/x", nil),
		func(c echo.Context) error { return echo.NewHTTPError(http.StatusForbidden, "no") })
	assert.EqualValues(t, 403, out["status"])
}
