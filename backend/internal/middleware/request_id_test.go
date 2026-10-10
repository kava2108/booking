package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serve(h echo.HandlerFunc, mw echo.MiddlewareFunc, req *http.Request) *httptest.ResponseRecorder {
	e := echo.New()
	rec := httptest.NewRecorder()
	e.GET("/", h, mw)
	e.ServeHTTP(rec, req)
	return rec
}

// IMP 3-user-auth-go-infra TASK-0001: ヘッダが無ければ生成し、レスポンスと context に載せる
func TestRequestID_Generates(t *testing.T) {
	var seen string
	rec := serve(func(c echo.Context) error {
		seen = GetRequestID(c.Request().Context())
		return c.NoContent(http.StatusOK)
	}, RequestID(), httptest.NewRequest(http.MethodGet, "/", nil))

	id := rec.Header().Get(HeaderRequestID)
	_, err := uuid.Parse(id)
	require.NoError(t, err)
	assert.Equal(t, id, seen)
}

func TestRequestID_PropagatesExisting(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderRequestID, "req-123")

	var seen string
	rec := serve(func(c echo.Context) error {
		seen = GetRequestID(c.Request().Context())
		return c.NoContent(http.StatusOK)
	}, RequestID(), req)

	assert.Equal(t, "req-123", rec.Header().Get(HeaderRequestID))
	assert.Equal(t, "req-123", seen)
}

// 境界値: 異常に長い・制御文字を含む値は信用せず再生成する（ログインジェクション対策）
func TestRequestID_RejectsUnsafeValues(t *testing.T) {
	for name, v := range map[string]string{
		"too long": string(make([]byte, 200)),
		"newline":  "abc\ndef",
		"spaces":   "a b",
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(HeaderRequestID, v)
			rec := serve(func(c echo.Context) error { return c.NoContent(http.StatusOK) }, RequestID(), req)

			got := rec.Header().Get(HeaderRequestID)
			assert.NotEqual(t, v, got)
			_, err := uuid.Parse(got)
			assert.NoError(t, err)
		})
	}
}

func TestGetRequestID_EmptyWhenMissing(t *testing.T) {
	assert.Equal(t, "", GetRequestID(httptest.NewRequest(http.MethodGet, "/", nil).Context()))
}
