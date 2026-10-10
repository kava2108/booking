// Package app はミドルウェアチェイン・ルーティング・依存の結線を組み立てる。
// IMP 10-user-auth-auth-handler TASK-0001（cmd/api/main.go の newEcho を移動）
package app

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/ulule/limiter/v3"

	"github.com/kava2108/booking/backend/internal/apierror"
	"github.com/kava2108/booking/backend/internal/auth"
	"github.com/kava2108/booking/backend/internal/config"
	"github.com/kava2108/booking/backend/internal/handler"
	"github.com/kava2108/booking/backend/internal/middleware"
	"github.com/kava2108/booking/backend/internal/service"
)

// Deps は app が必要とする依存。Auth / Tokens が nil の場合は認証ルートを登録しない(healthz のみ)。
type Deps struct {
	Config *config.Config
	Logger zerolog.Logger
	Auth   service.AuthService
	Tokens *auth.TokenManager
}

// New は Echo を組み立てる。
func New(d Deps) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	// X-Forwarded-For は偽装可能なためレートリミット等で信用しない。
	// リバースプロキシ配下に置く場合は信頼するプロキシを明示して差し替える。
	e.IPExtractor = echo.ExtractIPDirect()
	e.Validator = handler.NewValidator()
	e.HTTPErrorHandler = apierror.Handler()

	e.Use(
		middleware.RequestID(),
		middleware.Logger(d.Logger),
		echomw.RecoverWithConfig(echomw.RecoverConfig{
			// panic は zerolog で記録する(Echo 標準ロガーに出さない)。応答には内部情報を出さない。
			LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
				d.Logger.Error().Err(err).Str("request_id", middleware.GetRequestID(c.Request().Context())).
					Str("path", c.Request().URL.Path).Bytes("stack", stack).Msg("panic recovered")
				return err
			},
		}),
		echomw.BodyLimit("1M"),
	)

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	if d.Auth != nil && d.Tokens != nil {
		cfg := d.Config
		// ログインのアカウント単位の制限(連続失敗でロック)は service の AttemptGuard が担う。
		// ここの IP 単位の制限は、全アカウントをまたぐ試行に対する緩い天井。
		loginIP := limiter.Rate{Limit: int64(cfg.RateLimitLoginIPMax), Period: cfg.RateLimitLoginWindow}
		sensitive := limiter.Rate{Limit: int64(cfg.RateLimitSensitiveMax), Period: cfg.RateLimitLoginWindow}
		public := limiter.Rate{Limit: int64(cfg.RateLimitPublicMax), Period: cfg.RateLimitPublicWindow}
		// インスタンスを分けてカウンタを独立させる(refresh の頻度が login の枠を消費しない)
		rl := handler.RateLimiters{
			Login:     middleware.RateLimiter(loginIP),
			Sensitive: middleware.RateLimiter(sensitive),
			Refresh:   middleware.RateLimiter(public),
		}
		handler.NewAuthHandler(d.Auth, cfg.CookieSecure).Routes(e.Group("/api/v1/auth"), rl, middleware.JWTAuth(d.Tokens))
	}
	return e
}

// ShutdownTimeout は graceful shutdown の待ち時間。
const ShutdownTimeout = 10 * time.Second
