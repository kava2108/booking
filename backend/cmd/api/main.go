// Command api は REST API サーバーのエントリーポイント。
// IMP 3-user-auth-go-infra / 10-user-auth-auth-handler TASK-0001
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/kava2108/booking/backend/internal/app"
	"github.com/kava2108/booking/backend/internal/auth"
	"github.com/kava2108/booking/backend/internal/config"
	"github.com/kava2108/booking/backend/internal/repository"
	"github.com/kava2108/booking/backend/internal/repository/db"
	"github.com/kava2108/booking/backend/internal/service"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	if err := run(logger); err != nil {
		logger.Fatal().Err(err).Msg("fatal")
	}
}

func run(logger zerolog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	poolCfg, err := pgxpool.ParseConfig(cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("parse DB_DSN: %w", errors.New("invalid DSN")) // DSN にはパスワードが含まれ得るため内容は出さない
	}
	poolCfg.MaxConns = cfg.DBMaxConns
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	tm, err := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTAccessTTL)
	if err != nil {
		return err
	}
	q := db.New(pool)
	svc := service.NewAuthService(repository.NewUserRepository(q), repository.NewTokenRepository(q), tm, newMailer(cfg, logger),
		service.AuthConfig{
			RefreshTTL: cfg.JWTRefreshTTL, ResetTTL: 30 * time.Minute, ResetURLBase: cfg.AppBaseURL,
			LoginMaxFailures: cfg.RateLimitLoginMax, LoginLockWindow: cfg.RateLimitLoginWindow,
		},
		service.WithLogger(logger))

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.ServerPort),
		Handler:           app.New(app.Deps{Config: cfg, Logger: logger, Auth: svc, Tokens: tm}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	logger.Info().Int("port", cfg.ServerPort).Msg("server started")

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
	}
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), app.ShutdownTimeout)
	defer cancelShutdown()
	return srv.Shutdown(shutdownCtx)
}

// newMailer は SMTP_HOST があれば SMTP、無ければ何も送らない実装を返す(開発用)。
func newMailer(cfg *config.Config, logger zerolog.Logger) service.Mailer {
	if cfg.SMTPHost == "" {
		logger.Warn().Msg("SMTP_HOST is not set: emails will not be sent")
		return service.NoopMailer{}
	}
	return service.NewSMTPMailer(service.SMTPConfig{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Password: cfg.SMTPPassword, From: cfg.SMTPFrom,
	})
}
