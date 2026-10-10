package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"github.com/kava2108/booking/backend/internal/domain"
)

// withIdentity は JWT ミドルウェアの代わりに認証済み情報を注入するテスト用ミドルウェア。
func withIdentity(role domain.Role) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			SetIdentity(c, Identity{UserID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Role: role})
			return next(c)
		}
	}
}

func callWith(mws []echo.MiddlewareFunc, h echo.HandlerFunc) (*httptest.ResponseRecorder, *bool) {
	called := false
	e := echo.New()
	e.GET("/x", func(c echo.Context) error { called = true; return h(c) }, mws...)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	return rec, &called
}

func ok(c echo.Context) error { return c.NoContent(http.StatusOK) }

// REQ-005-AC-4: 権限マトリクス（staff API）
func TestStaffOnly_Matrix(t *testing.T) {
	cases := map[domain.Role]int{
		domain.RolePatient:     http.StatusForbidden,
		domain.RoleStaff:       http.StatusOK,
		domain.RoleClinicAdmin: http.StatusOK, // 管理者はスタッフ操作も可（IMP 6 で確定）
		domain.RoleSystemAdmin: http.StatusForbidden,
		domain.Role("unknown"): http.StatusForbidden,
		domain.Role(""):        http.StatusForbidden,
	}
	for role, want := range cases {
		t.Run(string(role), func(t *testing.T) {
			rec, called := callWith([]echo.MiddlewareFunc{withIdentity(role), StaffOnly()}, ok)
			assert.Equal(t, want, rec.Code)
			assert.Equal(t, want == http.StatusOK, *called, "拒否時はハンドラーを呼ばない")
		})
	}
}

// 権限マトリクス（admin API）
func TestAdminOnly_Matrix(t *testing.T) {
	cases := map[domain.Role]int{
		domain.RolePatient:     http.StatusForbidden,
		domain.RoleStaff:       http.StatusForbidden,
		domain.RoleClinicAdmin: http.StatusOK,
		domain.RoleSystemAdmin: http.StatusForbidden,
	}
	for role, want := range cases {
		t.Run(string(role), func(t *testing.T) {
			rec, _ := callWith([]echo.MiddlewareFunc{withIdentity(role), AdminOnly()}, ok)
			assert.Equal(t, want, rec.Code)
		})
	}
}

// 認証情報が無い（JWT ミドルウェアを通っていない）場合は 401
func TestRequireRole_Unauthenticated(t *testing.T) {
	rec, called := callWith([]echo.MiddlewareFunc{StaffOnly()}, ok)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, *called)
}

func TestRequireRole_ForbiddenBody(t *testing.T) {
	rec, _ := callWith([]echo.MiddlewareFunc{withIdentity(domain.RolePatient), StaffOnly()}, ok)
	assert.JSONEq(t, `{"error":"forbidden"}`, rec.Body.String())
}

func TestRequireRole_CustomList(t *testing.T) {
	rec, _ := callWith([]echo.MiddlewareFunc{withIdentity(domain.RoleSystemAdmin), RequireRole(domain.RoleSystemAdmin)}, ok)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// 引数なしの RequireRole は誰も通さない（設定ミスで全開放にならない）
func TestRequireRole_EmptyListDeniesAll(t *testing.T) {
	rec, _ := callWith([]echo.MiddlewareFunc{withIdentity(domain.RoleClinicAdmin), RequireRole()}, ok)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestIdentityFrom(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())

	_, found := IdentityFrom(c)
	assert.False(t, found)

	SetIdentity(c, Identity{UserID: uuid.New(), Role: domain.RoleStaff})
	id, found := IdentityFrom(c)
	assert.True(t, found)
	assert.Equal(t, domain.RoleStaff, id.Role)
}
