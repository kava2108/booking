// Package handler は HTTP の入出力のみを扱う。業務ロジックは service に委譲する。
package handler

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/kava2108/booking/backend/internal/apierror"
	"github.com/kava2108/booking/backend/internal/service"
)

const (
	refreshCookieName = "refresh_token"
	// Cookie の Path。refresh と logout の両方に Cookie を送るため /api/v1/auth 配下全体にする
	// (設計書の /api/v1/auth/refresh のままだと logout で Cookie が届かない)。
	refreshCookiePath = "/api/v1/auth"
)

// AuthHandler は /api/v1/auth の各エンドポイント。
type AuthHandler struct {
	svc          service.AuthService
	cookieSecure bool
}

func NewAuthHandler(svc service.AuthService, cookieSecure bool) *AuthHandler {
	return &AuthHandler{svc: svc, cookieSecure: cookieSecure}
}

// ---- リクエスト / レスポンス DTO（password_hash などは含めない） ----

type registerRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=12,max=72"`
	Name     string `json:"name" validate:"required,max=100"`
}

type registerResponse struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,max=128"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type resetRequestRequest struct {
	Email string `json:"email" validate:"required,email,max=255"`
}

type resetConfirmRequest struct {
	Token       string `json:"token" validate:"required,max=128"`
	NewPassword string `json:"new_password" validate:"required,min=12,max=72"`
}

type messageResponse struct {
	Message string `json:"message"`
}

// bindValidate は JSON を読み込んで検証する。応答済み(エラー応答を書いた)なら handled=true を返す。
func bindValidate(c echo.Context, dst any) (handled bool, err error) {
	if err := c.Bind(dst); err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) && he.Code == http.StatusRequestEntityTooLarge {
			return true, err
		}
		return true, apierror.Write(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
	}
	if err := c.Validate(dst); err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			return true, apierror.Write(c, http.StatusUnprocessableEntity, "validation_failed", "validation failed", ve.Fields...)
		}
		return true, err
	}
	return false, nil
}

func (h *AuthHandler) refreshCookie(value string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name: refreshCookieName, Value: value, Path: refreshCookiePath,
		Expires: expires, MaxAge: int(time.Until(expires).Seconds()),
		HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteStrictMode,
	}
}

func (h *AuthHandler) clearCookie() *http.Cookie {
	return &http.Cookie{
		Name: refreshCookieName, Value: "", Path: refreshCookiePath, MaxAge: -1,
		Expires: time.Unix(0, 0), HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteStrictMode,
	}
}

func (h *AuthHandler) writeTokens(c echo.Context, pair *service.TokenPair) error {
	c.SetCookie(h.refreshCookie(pair.RefreshToken, pair.RefreshExpiresAt))
	return c.JSON(http.StatusOK, tokenResponse{AccessToken: pair.AccessToken, TokenType: "Bearer", ExpiresIn: pair.ExpiresIn})
}

// Register: POST /api/v1/auth/register
func (h *AuthHandler) Register(c echo.Context) error {
	var req registerRequest
	if handled, err := bindValidate(c, &req); handled {
		return err
	}
	u, err := h.svc.Register(c.Request().Context(), service.RegisterInput{Email: req.Email, Password: req.Password, Name: req.Name})
	switch {
	case errors.Is(err, service.ErrEmailAlreadyExists):
		return apierror.Write(c, http.StatusConflict, "email_already_registered", "email already registered")
	case errors.Is(err, service.ErrWeakPassword):
		return apierror.Write(c, http.StatusUnprocessableEntity, "validation_failed", "validation failed",
			apierror.Field{Field: "password", Message: "must be 12 to 72 bytes and at least 12 characters"})
	case err != nil:
		return err
	}
	return c.JSON(http.StatusCreated, registerResponse{UserID: u.ID.String(), Email: u.Email, Name: u.Name, CreatedAt: u.CreatedAt})
}

// Login: POST /api/v1/auth/login
func (h *AuthHandler) Login(c echo.Context) error {
	var req loginRequest
	if handled, err := bindValidate(c, &req); handled {
		return err
	}
	pair, err := h.svc.Login(c.Request().Context(), req.Email, req.Password)
	var tooMany *service.TooManyAttemptsError
	switch {
	case errors.As(err, &tooMany):
		c.Response().Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(tooMany.RetryAfter.Seconds()))))
		return apierror.Write(c, http.StatusTooManyRequests, "too_many_requests", "too many failed login attempts; try again later")
	case errors.Is(err, service.ErrInvalidCredentials):
		return apierror.Write(c, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
	case err != nil:
		return err
	}
	return h.writeTokens(c, pair)
}

// Refresh: POST /api/v1/auth/refresh (Cookie 必須)
func (h *AuthHandler) Refresh(c echo.Context) error {
	cookie, err := c.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" {
		return apierror.Write(c, http.StatusUnauthorized, "invalid_token", "invalid or expired token")
	}
	pair, err := h.svc.RefreshToken(c.Request().Context(), cookie.Value)
	switch {
	case errors.Is(err, service.ErrInvalidToken):
		c.SetCookie(h.clearCookie())
		return apierror.Write(c, http.StatusUnauthorized, "invalid_token", "invalid or expired token")
	case err != nil:
		return err
	}
	return h.writeTokens(c, pair)
}

// Logout: POST /api/v1/auth/logout (Bearer 必須)。Cookie が無くても成功(冪等)。
func (h *AuthHandler) Logout(c echo.Context) error {
	if cookie, err := c.Cookie(refreshCookieName); err == nil {
		if err := h.svc.Logout(c.Request().Context(), cookie.Value); err != nil {
			return err
		}
	}
	c.SetCookie(h.clearCookie())
	return c.JSON(http.StatusOK, messageResponse{Message: "logged out"})
}

// RequestPasswordReset: POST /api/v1/auth/password-reset/request。登録の有無にかかわらず同一応答。
func (h *AuthHandler) RequestPasswordReset(c echo.Context) error {
	var req resetRequestRequest
	if handled, err := bindValidate(c, &req); handled {
		return err
	}
	if err := h.svc.RequestPasswordReset(c.Request().Context(), req.Email); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, messageResponse{Message: "If the email is registered, a reset link has been sent."})
}

// ConfirmPasswordReset: POST /api/v1/auth/password-reset/confirm
func (h *AuthHandler) ConfirmPasswordReset(c echo.Context) error {
	var req resetConfirmRequest
	if handled, err := bindValidate(c, &req); handled {
		return err
	}
	err := h.svc.ConfirmPasswordReset(c.Request().Context(), req.Token, req.NewPassword)
	switch {
	case errors.Is(err, service.ErrInvalidResetToken):
		return apierror.Write(c, http.StatusBadRequest, "invalid_reset_token", "invalid or expired token")
	case errors.Is(err, service.ErrWeakPassword):
		return apierror.Write(c, http.StatusUnprocessableEntity, "validation_failed", "validation failed",
			apierror.Field{Field: "new_password", Message: "must be 12 to 72 bytes and at least 12 characters"})
	case err != nil:
		return err
	}
	return c.JSON(http.StatusOK, messageResponse{Message: "password has been reset successfully"})
}

// RateLimiters は認証系エンドポイントごとのレートリミット。
type RateLimiters struct {
	Login     echo.MiddlewareFunc // IP 単位の緩い天井(アカウント単位の制限は service の AttemptGuard)
	Sensitive echo.MiddlewareFunc // 登録・リセット申請/確定(メール送信の濫用防止)
	Refresh   echo.MiddlewareFunc // 通常の更新頻度を妨げない緩い制限
}

// Routes は /api/v1/auth 配下のルートを登録する。トークンを返す応答はキャッシュさせない。
func (h *AuthHandler) Routes(g *echo.Group, rl RateLimiters, jwtAuth echo.MiddlewareFunc) {
	// g.Use ではなくルートごとに付ける(g.Use は全メソッドを受ける 404 ルートを作り、メソッド違いが 405 にならない)
	g.POST("/register", h.Register, noStore, rl.Sensitive)
	g.POST("/login", h.Login, noStore, rl.Login)
	g.POST("/refresh", h.Refresh, noStore, rl.Refresh)
	g.POST("/logout", h.Logout, noStore, jwtAuth)
	g.POST("/password-reset/request", h.RequestPasswordReset, noStore, rl.Sensitive)
	g.POST("/password-reset/confirm", h.ConfirmPasswordReset, noStore, rl.Sensitive)
}

func noStore(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		c.Response().Header().Set("Pragma", "no-cache")
		return next(c)
	}
}
