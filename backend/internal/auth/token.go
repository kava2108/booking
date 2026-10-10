// Package auth はアクセストークン(JWT)とリフレッシュ/リセット用の opaque トークンを扱う。
// service（発行）と middleware（検証）が同じ実装を使うために独立させている。
// IMP 8-user-auth-auth-service-register-login TASK-0001
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/kava2108/booking/backend/internal/domain"
)

// Issuer はアクセストークンの発行者。検証時にも一致を要求する。
const Issuer = "clinic-booking"

// minSecretBytes は HS256 の鍵として最低限必要な長さ。
const minSecretBytes = 32

// ErrInvalidToken は署名不正・期限切れ・クレーム不正など、検証に失敗したすべてのケース。
// 理由を呼び出し側に区別させない（情報を与えない）。
var ErrInvalidToken = errors.New("auth: invalid token")

// Claims はアクセストークンのクレーム。sub = ユーザー ID。
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// Identity は検証済みのトークンから取り出したユーザー情報。
type Identity struct {
	UserID uuid.UUID
	Role   domain.Role
}

// TokenManager はアクセストークン(HS256)の発行と検証を行う。
type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// Option は TokenManager の設定。
type Option func(*TokenManager)

// WithClock はテスト用に現在時刻を差し替える。
func WithClock(now func() time.Time) Option { return func(m *TokenManager) { m.now = now } }

// NewTokenManager は秘密鍵(32 bytes 以上)と有効期間(正の値)を検証して作成する。
func NewTokenManager(secret string, ttl time.Duration, opts ...Option) (*TokenManager, error) {
	if len(secret) < minSecretBytes {
		return nil, fmt.Errorf("auth: JWT secret must be at least %d bytes", minSecretBytes)
	}
	if ttl <= 0 {
		return nil, errors.New("auth: access token TTL must be positive")
	}
	m := &TokenManager{secret: []byte(secret), ttl: ttl, now: time.Now}
	for _, o := range opts {
		o(m)
	}
	return m, nil
}

// IssueAccess はアクセストークンを発行し、有効期間を返す。
func (m *TokenManager) IssueAccess(userID uuid.UUID, role domain.Role) (string, time.Duration, error) {
	if !role.Valid() {
		return "", 0, fmt.Errorf("auth: invalid role %q", role)
	}
	now := m.now()
	claims := Claims{Role: string(role), RegisteredClaims: jwt.RegisteredClaims{
		Subject:   userID.String(),
		Issuer:    Issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
	}}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", 0, fmt.Errorf("auth: sign token: %w", err)
	}
	return s, m.ttl, nil
}

// ParseAccess はアクセストークンを検証する。
// HS256 のみ受理(alg=none / 他アルゴリズムは拒否)、exp 必須、issuer・sub(UUID)・role を検証する。
func (m *TokenManager) ParseAccess(token string) (Identity, error) {
	var claims Claims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(Issuer),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return Identity{}, ErrInvalidToken
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return Identity{}, ErrInvalidToken
	}
	role := domain.Role(claims.Role)
	if !role.Valid() {
		return Identity{}, ErrInvalidToken
	}
	return Identity{UserID: id, Role: role}, nil
}

// GenerateOpaqueToken は 32 bytes の乱数から raw トークン(base64url)とその SHA-256 ハッシュを返す。
// DB にはハッシュのみ保存し、raw はユーザーに渡す(Cookie / メール)だけにする。
func GenerateOpaqueToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("auth: generate token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashToken(raw), nil
}

// HashToken は raw トークンの SHA-256 を 16 進文字列(64 文字)で返す。
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
