package service

import (
	"context"
	"net/smtp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// IMP 8-user-auth-auth-service-register-login TASK-0001: メール送信

type sent struct {
	addr string
	from string
	to   []string
	msg  string
}

func newSMTP(t *testing.T) (*SMTPMailer, *[]sent) {
	t.Helper()
	var out []sent
	m := NewSMTPMailer(SMTPConfig{Host: "smtp.example.com", Port: 587, User: "u", Password: "p", From: "noreply@example.com"})
	m.send = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		out = append(out, sent{addr, from, to, string(msg)})
		return nil
	}
	return m, &out
}

func TestSMTPMailer_PasswordReset(t *testing.T) {
	m, out := newSMTP(t)

	require.NoError(t, m.SendPasswordReset(context.Background(), "user@example.com", "山田 太郎", "https://app.example.com/password-reset/confirm?token=abc"))

	require.Len(t, *out, 1)
	got := (*out)[0]
	assert.Equal(t, "smtp.example.com:587", got.addr)
	assert.Equal(t, "noreply@example.com", got.from)
	assert.Equal(t, []string{"user@example.com"}, got.to)
	assert.Contains(t, got.msg, "To: user@example.com\r\n")
	assert.Contains(t, got.msg, "Content-Type: text/plain; charset=UTF-8")
	assert.Contains(t, got.msg, "Content-Transfer-Encoding: base64")
	assert.Contains(t, got.msg, "Subject: =?utf-8?")
}

func TestSMTPMailer_Registration(t *testing.T) {
	m, out := newSMTP(t)

	require.NoError(t, m.SendRegistrationConfirmation(context.Background(), "user@example.com", "山田 太郎"))
	assert.Len(t, *out, 1)
}

// セキュリティ: ヘッダーインジェクション（宛先・件名への改行）を拒否する
func TestSMTPMailer_RejectsHeaderInjection(t *testing.T) {
	m, out := newSMTP(t)

	for _, to := range []string{"a@example.com\r\nBcc: evil@example.com", "a@example.com\nBcc: x@example.com", "a@example.com,b@example.com", ""} {
		err := m.SendRegistrationConfirmation(context.Background(), to, "名前")
		assert.Error(t, err, "%q", to)
	}
	assert.Empty(t, *out, "不正な宛先では送信しない")
}

// 名前に改行があってもヘッダーには影響しない（本文のみに入る・エンコード済み）
func TestSMTPMailer_NameWithNewlineDoesNotBreakHeaders(t *testing.T) {
	m, out := newSMTP(t)

	require.NoError(t, m.SendRegistrationConfirmation(context.Background(), "a@example.com", "Evil\r\nBcc: x@example.com"))
	headers := strings.SplitN((*out)[0].msg, "\r\n\r\n", 2)[0]
	assert.NotContains(t, headers, "Bcc")
}

func TestSMTPMailer_PropagatesSendError(t *testing.T) {
	m, _ := newSMTP(t)
	m.send = func(string, smtp.Auth, string, []string, []byte) error { return assert.AnError }

	err := m.SendRegistrationConfirmation(context.Background(), "a@example.com", "n")
	assert.ErrorIs(t, err, assert.AnError)
}

func TestNoopMailer(t *testing.T) {
	var m Mailer = NoopMailer{}
	assert.NoError(t, m.SendRegistrationConfirmation(context.Background(), "a@example.com", "n"))
	assert.NoError(t, m.SendPasswordReset(context.Background(), "a@example.com", "n", "https://x"))
}

// IMP 14: 汎用 Send（通知ワーカー用）
func TestSMTPMailer_SendGeneric(t *testing.T) {
	m, out := newSMTP(t)

	require.NoError(t, m.Send(context.Background(), "user@example.com", "【予約確認】テスト", "1行目\n2行目"))

	require.Len(t, *out, 1)
	assert.Contains(t, (*out)[0].msg, "Subject: =?utf-8?")
	assert.NotContains(t, (*out)[0].msg, "【予約確認】", "件名は RFC 2047 でエンコードされる")
}

func TestSMTPMailer_SendRejectsHeaderInjection(t *testing.T) {
	m, out := newSMTP(t)

	assert.Error(t, m.Send(context.Background(), "user@example.com", "件名\r\nBcc: evil@example.com", "本文"))
	assert.Error(t, m.Send(context.Background(), "a@example.com\nBcc: x@example.com", "件名", "本文"))
	assert.Empty(t, *out)
}

func TestNoopMailer_Send(t *testing.T) {
	assert.NoError(t, NoopMailer{}.Send(context.Background(), "a@example.com", "s", "b"))
}
