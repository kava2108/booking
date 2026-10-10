package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/auth"
	"github.com/kava2108/booking/backend/internal/domain"
)

// IMP 10-user-auth-auth-handler TASK-0001

const jwtTestSecret = "0123456789abcdef0123456789abcdef"

func jwtServer(t *testing.T, clock func() time.Time) (*echo.Echo, *auth.TokenManager, *Identity) {
	t.Helper()
	tm, err := auth.NewTokenManager(jwtTestSecret, 15*time.Minute, auth.WithClock(clock))
	require.NoError(t, err)
	var seen Identity
	e := echo.New()
	e.GET("/me", func(c echo.Context) error {
		id, _ := IdentityFrom(c)
		seen = id
		return c.NoContent(http.StatusOK)
	}, JWTAuth(tm))
	e.GET("/staff", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, JWTAuth(tm), StaffOnly())
	return e, tm, &seen
}

func get(e *echo.Echo, path, authz string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestJWTAuth_SetsIdentity(t *testing.T) {
	e, tm, seen := jwtServer(t, time.Now)
	uid := uuid.New()
	tok, _, _ := tm.IssueAccess(uid, domain.RoleStaff)

	rec := get(e, "/me", "Bearer "+tok)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, Identity{UserID: uid, Role: domain.RoleStaff}, *seen)
}

// RFC 7235: スキーム名は大文字小文字を区別しない
func TestJWTAuth_SchemeIsCaseInsensitive(t *testing.T) {
	e, tm, _ := jwtServer(t, time.Now)
	tok, _, _ := tm.IssueAccess(uuid.New(), domain.RolePatient)

	assert.Equal(t, http.StatusOK, get(e, "/me", "bearer "+tok).Code)
	assert.Equal(t, http.StatusOK, get(e, "/me", "BEARER "+tok).Code)
}

func TestJWTAuth_Rejects(t *testing.T) {
	now := time.Now()
	e, tm, _ := jwtServer(t, func() time.Time { return now })
	tok, _, _ := tm.IssueAccess(uuid.New(), domain.RolePatient)
	now = now.Add(time.Hour) // 期限切れにする

	cases := map[string]string{
		"ヘッダーなし":   "",
		"Basic 認証": "Basic dXNlcjpwYXNz",
		"トークンなし":   "Bearer",
		"トークン空":    "Bearer ",
		"ゴミ":       "Bearer not-a-jwt",
		"期限切れ":     "Bearer " + tok,
		"スキームなし":   tok,
		"余分なトークン":  "Bearer " + tok + " extra",
	}
	for name, h := range cases {
		t.Run(name, func(t *testing.T) {
			rec := get(e, "/me", h)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.JSONEq(t, `{"error":"unauthorized"}`, rec.Body.String())
			assert.Equal(t, "Bearer", rec.Header().Get("WWW-Authenticate"))
		})
	}
}

// 認可との連携: トークンのロールが StaffOnly に反映される
func TestJWTAuth_WithStaffOnly(t *testing.T) {
	e, tm, _ := jwtServer(t, time.Now)
	patient, _, _ := tm.IssueAccess(uuid.New(), domain.RolePatient)
	staff, _, _ := tm.IssueAccess(uuid.New(), domain.RoleStaff)

	assert.Equal(t, http.StatusForbidden, get(e, "/staff", "Bearer "+patient).Code)
	assert.Equal(t, http.StatusOK, get(e, "/staff", "Bearer "+staff).Code)
	assert.Equal(t, http.StatusUnauthorized, get(e, "/staff", "").Code)
}
