package app_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/kava2108/booking/backend/internal/app"
	"github.com/kava2108/booking/backend/internal/config"
)

// IMP 3-user-auth-go-infra のテストを app パッケージへ移動（newEcho → app.New）。認証なしでも起動できる。

func baseServer() (*httptest.ResponseRecorder, *bytes.Buffer, func(*http.Request)) {
	var buf bytes.Buffer
	e := app.New(app.Deps{Config: &config.Config{}, Logger: zerolog.New(&buf)})
	rec := httptest.NewRecorder()
	return rec, &buf, func(r *http.Request) { e.ServeHTTP(rec, r) }
}

func TestHealthz(t *testing.T) {
	rec, _, serve := baseServer()
	serve(httptest.NewRequest(http.MethodGet, "/healthz", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
}

func TestServer_LogsRequestID(t *testing.T) {
	_, buf, serve := baseServer()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "req-wired")
	serve(req)

	assert.Contains(t, buf.String(), `"request_id":"req-wired"`)
}

// セキュリティ: X-Forwarded-For を信用しない（レートリミット迂回の防止）
func TestServer_DoesNotTrustForwardedFor(t *testing.T) {
	e := app.New(app.Deps{Config: &config.Config{}, Logger: zerolog.Nop()})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	req.Header.Set("X-Forwarded-For", "1.1.1.1")

	assert.Equal(t, "203.0.113.9", e.IPExtractor(req))
}

func TestServer_NotFound(t *testing.T) {
	rec, _, serve := baseServer()
	serve(httptest.NewRequest(http.MethodGet, "/nope", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// panic しても 500 を返し、サーバーは継続する（recover）。内部情報は返さない
func TestServer_RecoversFromPanic(t *testing.T) {
	var buf bytes.Buffer
	e := app.New(app.Deps{Config: &config.Config{}, Logger: zerolog.New(&buf)})
	e.GET("/boom", func(c echo.Context) error { panic("secret internal detail") })
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "secret internal detail")
	assert.JSONEq(t, `{"error":"internal_error","message":"internal server error","request_id":"`+rec.Header().Get("X-Request-ID")+`"}`, rec.Body.String())
}
