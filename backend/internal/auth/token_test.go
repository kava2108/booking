package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kava2108/booking/backend/internal/domain"
)

// IMP 8-user-auth-auth-service-register-login TASK-0001

const secret = "0123456789abcdef0123456789abcdef" // 32 bytes

var (
	t0  = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	uid = uuid.MustParse("11111111-1111-1111-1111-111111111111")
)

func newManager(t *testing.T, now *time.Time) *TokenManager {
	t.Helper()
	m, err := NewTokenManager(secret, 15*time.Minute, WithClock(func() time.Time { return *now }))
	require.NoError(t, err)
	return m
}

func TestIssueAndParse_RoundTrip(t *testing.T) {
	now := t0
	m := newManager(t, &now)

	tok, expiresIn, err := m.IssueAccess(uid, domain.RoleStaff)
	require.NoError(t, err)
	assert.Equal(t, 15*time.Minute, expiresIn)

	id, err := m.ParseAccess(tok)
	require.NoError(t, err)
	assert.Equal(t, uid, id.UserID)
	assert.Equal(t, domain.RoleStaff, id.Role)
}

// 境界値: 期限ちょうどまでは有効でなく、期限後は無効
func TestParseAccess_Expiry(t *testing.T) {
	now := t0
	m := newManager(t, &now)
	tok, _, err := m.IssueAccess(uid, domain.RolePatient)
	require.NoError(t, err)

	now = t0.Add(15*time.Minute - time.Second)
	_, err = m.ParseAccess(tok)
	assert.NoError(t, err)

	now = t0.Add(15*time.Minute + time.Second)
	_, err = m.ParseAccess(tok)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestParseAccess_RejectsTamperedAndMalformed(t *testing.T) {
	now := t0
	m := newManager(t, &now)
	tok, _, _ := m.IssueAccess(uid, domain.RolePatient)
	parts := strings.Split(tok, ".")

	for name, bad := range map[string]string{
		"空":           "",
		"ゴミ":          "not-a-jwt",
		"署名改ざん":       parts[0] + "." + parts[1] + "." + strings.Repeat("A", len(parts[2])),
		"payload 改ざん": parts[0] + "." + parts[1] + "x." + parts[2],
		"署名なし":        parts[0] + "." + parts[1] + ".",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := m.ParseAccess(bad)
			assert.ErrorIs(t, err, ErrInvalidToken)
		})
	}
}

func sign(t *testing.T, method jwt.SigningMethod, key any, c Claims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, c).SignedString(key)
	require.NoError(t, err)
	return s
}

func validClaims(role string) Claims {
	return Claims{Role: role, RegisteredClaims: jwt.RegisteredClaims{
		Subject: uid.String(), Issuer: Issuer, IssuedAt: jwt.NewNumericDate(t0), ExpiresAt: jwt.NewNumericDate(t0.Add(time.Minute)),
	}}
}

// セキュリティ: alg 混同攻撃（none / 別 HMAC / 別鍵）を拒否する
func TestParseAccess_RejectsAlgorithmConfusionAndWrongKey(t *testing.T) {
	now := t0
	m := newManager(t, &now)

	cases := map[string]string{
		"alg=none": sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, validClaims("patient")),
		"HS512":    sign(t, jwt.SigningMethodHS512, []byte(secret), validClaims("patient")),
		"別の鍵":      sign(t, jwt.SigningMethodHS256, []byte("another-secret-another-secret-0000"), validClaims("patient")),
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := m.ParseAccess(tok)
			assert.ErrorIs(t, err, ErrInvalidToken)
		})
	}
	// 対照: 同じ作りで正しい鍵・HS256 なら通る（テストの前提確認）
	_, err := m.ParseAccess(sign(t, jwt.SigningMethodHS256, []byte(secret), validClaims("patient")))
	assert.NoError(t, err)
}

func TestParseAccess_RejectsBadClaims(t *testing.T) {
	now := t0
	m := newManager(t, &now)
	key := []byte(secret)

	noExp := validClaims("patient")
	noExp.ExpiresAt = nil
	wrongIssuer := validClaims("patient")
	wrongIssuer.Issuer = "someone-else"
	badSub := validClaims("patient")
	badSub.Subject = "not-a-uuid"
	noSub := validClaims("patient")
	noSub.Subject = ""

	cases := map[string]Claims{
		"exp なし":         noExp,
		"issuer 不一致":     wrongIssuer,
		"sub が UUID でない": badSub,
		"sub なし":         noSub,
		"未知のロール":         validClaims("superuser"),
		"旧ロール user":      validClaims("user"),
		"ロール空":           validClaims(""),
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := m.ParseAccess(sign(t, jwt.SigningMethodHS256, key, c))
			assert.ErrorIs(t, err, ErrInvalidToken)
		})
	}
}

// 秘密鍵が短い場合は起動時に拒否する（HS256 には 32 bytes 以上を要求）
func TestNewTokenManager_RejectsWeakSecretAndTTL(t *testing.T) {
	_, err := NewTokenManager("short", time.Minute)
	assert.Error(t, err)
	_, err = NewTokenManager(secret, 0)
	assert.Error(t, err)
	_, err = NewTokenManager(secret, -time.Minute)
	assert.Error(t, err)
	_, err = NewTokenManager(strings.Repeat("x", 31), time.Minute)
	assert.Error(t, err, "31 bytes は不可")
	_, err = NewTokenManager(strings.Repeat("x", 32), time.Minute)
	assert.NoError(t, err, "32 bytes は可")
}

func TestIssueAccess_RejectsInvalidRole(t *testing.T) {
	now := t0
	m := newManager(t, &now)

	_, _, err := m.IssueAccess(uid, domain.Role("hacker"))
	assert.Error(t, err)
}

func TestOpaqueToken(t *testing.T) {
	raw, hash, err := GenerateOpaqueToken()
	require.NoError(t, err)

	assert.Len(t, raw, 43, "32 bytes の base64url（パディングなし）")
	assert.Len(t, hash, 64, "SHA-256 の 16 進。refresh_tokens.token_hash は varchar(64)")
	assert.Equal(t, hash, HashToken(raw))
	assert.NotEqual(t, raw, hash)

	raw2, hash2, err := GenerateOpaqueToken()
	require.NoError(t, err)
	assert.NotEqual(t, raw, raw2)
	assert.NotEqual(t, hash, hash2)
}

func TestHashToken_KnownVector(t *testing.T) {
	// echo -n abc | sha256sum
	assert.Equal(t, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", HashToken("abc"))
}
