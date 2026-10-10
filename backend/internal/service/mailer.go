package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net/mail"
	"net/smtp"
	"strings"
)

// Mailer は認証まわりのメール送信。実装は差し替え可能(SMTP / 何もしない)。
// IMP 8-user-auth-auth-service-register-login TASK-0001
type Mailer interface {
	SendRegistrationConfirmation(ctx context.Context, to, name string) error
	// resetURL はトークンを含む完全な URL。ログに出さないこと。
	SendPasswordReset(ctx context.Context, to, name, resetURL string) error
}

// NoopMailer は SMTP 未設定時の実装。何も送らない(トークンを含む内容もログに出さない)。
type NoopMailer struct{}

func (NoopMailer) SendRegistrationConfirmation(context.Context, string, string) error { return nil }
func (NoopMailer) SendPasswordReset(context.Context, string, string, string) error    { return nil }

func (NoopMailer) Send(context.Context, string, string, string) error { return nil }

// SMTPConfig は SMTP 送信の設定。
type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

// SMTPMailer は net/smtp による送信。件名・本文は UTF-8(RFC 2047 / base64)でエンコードする。
type SMTPMailer struct {
	cfg  SMTPConfig
	auth smtp.Auth
	// send はテストで差し替える。既定は smtp.SendMail。
	send func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	m := &SMTPMailer{cfg: cfg, send: smtp.SendMail}
	if cfg.User != "" {
		m.auth = smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
	}
	return m
}

func (m *SMTPMailer) SendRegistrationConfirmation(_ context.Context, to, name string) error {
	body := fmt.Sprintf("%s 様\r\n\r\nご登録ありがとうございます。アカウントを作成しました。\r\n\r\nこのメールに心当たりがない場合は破棄してください。\r\n", sanitizeText(name))
	return m.deliver(to, "ご登録ありがとうございます", body)
}

func (m *SMTPMailer) SendPasswordReset(_ context.Context, to, name, resetURL string) error {
	body := fmt.Sprintf("%s 様\r\n\r\nパスワード再設定のご依頼を受け付けました。30 分以内に次の URL から設定してください。\r\n\r\n%s\r\n\r\nご依頼に心当たりがない場合は、このメールを破棄してください(パスワードは変更されません)。\r\n", sanitizeText(name), resetURL)
	return m.deliver(to, "パスワード再設定のご案内", body)
}

// Send は任意の件名・本文のメールを送る（NotificationSender の実装）。改行は CRLF に正規化する。
func (m *SMTPMailer) Send(_ context.Context, to, subject, body string) error {
	if strings.ContainsAny(subject, "\r\n") {
		return errors.New("mailer: invalid subject")
	}
	body = strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n")
	return m.deliver(to, subject, body)
}

// deliver は宛先を検証して送信する。宛先に改行・複数アドレスが含まれる場合は拒否する(ヘッダーインジェクション対策)。
func (m *SMTPMailer) deliver(to, subject, body string) error {
	addr, err := mail.ParseAddress(to)
	if err != nil || addr.Address != to || strings.ContainsAny(to, "\r\n,;") {
		return errors.New("mailer: invalid recipient")
	}
	var msg strings.Builder
	msg.WriteString("From: " + m.cfg.From + "\r\n")
	msg.WriteString("To: " + to + "\r\n")
	msg.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	msg.WriteString(wrapBase64(base64.StdEncoding.EncodeToString([]byte(body))))

	hostPort := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	if err := m.send(hostPort, m.auth, m.cfg.From, []string{to}, []byte(msg.String())); err != nil {
		return fmt.Errorf("mailer: send: %w", err)
	}
	return nil
}

// sanitizeText は本文に差し込む名前から改行を除く(本文は base64 だが、念のため 1 行に保つ)。
func sanitizeText(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
}

func wrapBase64(s string) string {
	var b strings.Builder
	for len(s) > 76 {
		b.WriteString(s[:76] + "\r\n")
		s = s[76:]
	}
	b.WriteString(s + "\r\n")
	return b.String()
}
