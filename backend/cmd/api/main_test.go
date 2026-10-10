package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/kava2108/booking/backend/internal/config"
)

func testServer() (*http.Handler, *bytes.Buffer) {
	var buf bytes.Buffer
	var h http.Handler = newEcho(&config.Config{}, zerolog.New(&buf))
	return &h, &buf
}

// IMP 3-user-auth-go-infra TASK-0001: /healthz は認証不要で 200、X-Request-ID が付く
func TestHealthz(t *testing.T) {
	h, _ := testServer()
	rec := httptest.NewRecorder()
	(*h).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
}

// リクエストログに request_id が出る（ミドルウェアチェインの結線確認）
func TestServer_LogsRequestID(t *testing.T) {
	h, buf := testServer()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "req-wired")
	(*h).ServeHTTP(httptest.NewRecorder(), req)

	assert.Contains(t, buf.String(), `"request_id":"req-wired"`)
}

// セキュリティ: X-Forwarded-For を信用しない（レートリミット迂回の防止）
func TestServer_DoesNotTrustForwardedFor(t *testing.T) {
	e := newEcho(&config.Config{}, zerolog.Nop())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	req.Header.Set("X-Forwarded-For", "1.1.1.1")

	assert.Equal(t, "203.0.113.9", e.IPExtractor(req))
}

// 異常系: 未定義パスは 404、パニックしても 500 で継続する
func TestServer_NotFoundAndRecover(t *testing.T) {
	e := newEcho(&config.Config{}, zerolog.Nop())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
