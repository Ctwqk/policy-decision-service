package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ctwqk/policy-decision-service/internal/api"
	"github.com/Ctwqk/policy-decision-service/internal/config"
	"github.com/Ctwqk/policy-decision-service/internal/engine"
	"github.com/Ctwqk/policy-decision-service/internal/store"
	"github.com/Ctwqk/policy-decision-service/internal/telemetry"
)

func main() {
	cfg := config.Load()
	logger := telemetry.NewLogger()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	postgres, pgErr := store.NewPostgres(ctx, cfg.DatabaseURL)
	if pgErr != nil {
		logger.Warn().Err(pgErr).Msg("postgres readiness disabled")
	}
	defer postgres.Close()

	redisStore, redisErr := store.NewRedis(ctx, cfg.RedisURL)
	if redisErr != nil {
		logger.Warn().Err(redisErr).Msg("redis readiness disabled")
	}
	defer func() {
		if err := redisStore.Close(); err != nil {
			logger.Warn().Err(err).Msg("redis close failed")
		}
	}()

	router := api.NewRouter(api.Dependencies{
		Engine: engine.NewAllowEngine("bootstrap"),
		Ready: func(ctx context.Context) error {
			if pgErr != nil {
				return pgErr
			}
			if redisErr != nil {
				return redisErr
			}
			if err := postgres.Health(ctx); err != nil {
				return err
			}
			return redisStore.Health(ctx)
		},
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error().Err(err).Msg("http shutdown failed")
		}
	}()

	logger.Info().Str("addr", cfg.HTTPAddr).Msg("starting pds http server")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal().Err(err).Msg("http server failed")
	}
}
