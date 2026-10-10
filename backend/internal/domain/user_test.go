package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// IMP 7-user-auth-domain-repository TASK-0001: トークンの有効性判定

var now = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)

func ptr(t time.Time) *time.Time { return &t }

func TestRefreshToken_IsUsable(t *testing.T) {
	cases := map[string]struct {
		tok  RefreshToken
		want bool
	}{
		"有効":        {RefreshToken{ExpiresAt: now.Add(time.Hour)}, true},
		"失効済み":      {RefreshToken{ExpiresAt: now.Add(time.Hour), RevokedAt: ptr(now.Add(-time.Minute))}, false},
		"期限切れ":      {RefreshToken{ExpiresAt: now.Add(-time.Second)}, false},
		"期限ちょうどは無効": {RefreshToken{ExpiresAt: now}, false},
		"期限1ns前は有効": {RefreshToken{ExpiresAt: now.Add(time.Nanosecond)}, true},
		"失効かつ期限切れ":  {RefreshToken{ExpiresAt: now.Add(-time.Hour), RevokedAt: ptr(now)}, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { assert.Equal(t, c.want, c.tok.IsUsable(now)) })
	}
}

func TestPasswordResetToken_IsUsable(t *testing.T) {
	cases := map[string]struct {
		tok  PasswordResetToken
		want bool
	}{
		"有効":        {PasswordResetToken{ExpiresAt: now.Add(30 * time.Minute)}, true},
		"使用済み":      {PasswordResetToken{ExpiresAt: now.Add(time.Hour), UsedAt: ptr(now)}, false},
		"期限切れ":      {PasswordResetToken{ExpiresAt: now.Add(-time.Second)}, false},
		"期限ちょうどは無効": {PasswordResetToken{ExpiresAt: now}, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { assert.Equal(t, c.want, c.tok.IsUsable(now)) })
	}
}
