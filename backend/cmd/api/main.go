// Command api は REST API サーバーのエントリーポイント。
// IMP 3-user-auth-go-infra TASK-0001
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

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"

	"github.com/kava2108/booking/backend/internal/config"
	"github.com/kava2108/booking/backend/internal/middleware"
)

// newEcho はミドルウェアチェイン（request_id → logger → recover）を組み立てる。
func newEcho(_ *config.Config, logger zerolog.Logger) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	// X-Forwarded-For は偽装可能なためレートリミット等で信用しない。
	// リバースプロキシ配下に置く場合は信頼するプロキシを明示して差し替える。
	e.IPExtractor = echo.ExtractIPDirect()

	e.Use(middleware.RequestID(), middleware.Logger(logger), echomw.Recover())

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	return e
}

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
	}

	e := newEcho(cfg, logger)
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.ServerPort),
		Handler:           e,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal().Err(err).Msg("server error")
		}
	}()
	logger.Info().Int("port", cfg.ServerPort).Msg("server started")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("graceful shutdown failed")
	}
}
