package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/repository/mocks"
)

func uuidValue(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

var actor = uuid.MustParse("11111111-1111-1111-1111-111111111111")

func auditedServer(rec AuditRecorder, role domain.Role, handler echo.HandlerFunc) *echo.Echo {
	e := echo.New()
	e.GET("/api/staff/queue/:id", handler, withIdentity(role), AuditLog(rec, "view_queue_entry", nil))
	return e
}

// REQ-007-AC-3: 個人情報へのアクセスを actor / action / target で記録する
func TestAuditLog_RecordsBeforeHandler(t *testing.T) {
	m := mocks.NewMockAdminRepository(t)
	m.EXPECT().InsertAuditLog(mock.Anything, db.InsertAuditLogParams{
		ActorID: uuidValue(actor), Action: "view_queue_entry", Target: "/api/staff/queue/abc",
	}).Return(nil).Once()

	called := false
	e := auditedServer(m, domain.RoleStaff, func(c echo.Context) error { called = true; return c.NoContent(http.StatusOK) })
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/staff/queue/abc", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called)
}

// セキュリティ: クエリ文字列（トークン等が載り得る）やボディは記録しない
func TestAuditLog_DoesNotRecordQueryStringOrBody(t *testing.T) {
	m := mocks.NewMockAdminRepository(t)
	var got db.InsertAuditLogParams
	m.EXPECT().InsertAuditLog(mock.Anything, mock.Anything).Run(func(_ context.Context, p db.InsertAuditLogParams) { got = p }).Return(nil)

	e := auditedServer(m, domain.RoleStaff, func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/staff/queue/abc?token=secret&note=咳", nil))

	assert.Equal(t, "/api/staff/queue/abc", got.Target)
	assert.NotContains(t, got.Action+got.Target, "secret")
	assert.NotContains(t, got.Action+got.Target, "咳")
}

// 監査ログを書けないときは個人情報を返さない（フェイルクローズ）
func TestAuditLog_FailsClosed(t *testing.T) {
	m := mocks.NewMockAdminRepository(t)
	m.EXPECT().InsertAuditLog(mock.Anything, mock.Anything).Return(errors.New("db down"))

	called := false
	e := auditedServer(m, domain.RoleStaff, func(c echo.Context) error { called = true; return c.NoContent(http.StatusOK) })
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/staff/queue/abc", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.False(t, called)
}

func TestAuditLog_RequiresIdentity(t *testing.T) {
	m := mocks.NewMockAdminRepository(t) // InsertAuditLog は呼ばれない
	e := echo.New()
	e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, AuditLog(m, "a", nil))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuditLog_CustomTarget(t *testing.T) {
	m := mocks.NewMockAdminRepository(t)
	m.EXPECT().InsertAuditLog(mock.Anything, mock.MatchedBy(func(p db.InsertAuditLogParams) bool {
		return p.Target == "appointment:42"
	})).Return(nil)

	e := echo.New()
	e.GET("/a/:id", func(c echo.Context) error { return c.NoContent(http.StatusOK) },
		withIdentity(domain.RoleStaff),
		AuditLog(m, "view", func(c echo.Context) string { return "appointment:" + c.Param("id") }))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/a/42", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}
