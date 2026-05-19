package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Ctwqk/policy-decision-service/internal/api"
	"github.com/Ctwqk/policy-decision-service/internal/config"
	"github.com/Ctwqk/policy-decision-service/internal/engine"
	"github.com/Ctwqk/policy-decision-service/internal/profile"
	"github.com/Ctwqk/policy-decision-service/internal/rules"
	"github.com/Ctwqk/policy-decision-service/internal/sink"
	"github.com/Ctwqk/policy-decision-service/internal/store"
	"github.com/Ctwqk/policy-decision-service/internal/telemetry"
)

type engineHolder struct {
	current atomic.Value
}

func newEngineHolder(initial api.DecisionEngine) *engineHolder {
	holder := &engineHolder{}
	holder.current.Store(initial)
	return holder
}

func (h *engineHolder) Evaluate(ctx context.Context, req engine.DecideRequest) (engine.DecideResponse, error) {
	current, _ := h.current.Load().(api.DecisionEngine)
	if current == nil {
		return engine.DecideResponse{}, errors.New("decision engine is not loaded")
	}
	return current.Evaluate(ctx, req)
}

func (h *engineHolder) Store(next api.DecisionEngine) {
	h.current.Store(next)
}

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

	featureProvider := profile.NewHTTPFeatureProvider(cfg.FeatureProviderURL, cfg.FeatureProviderTimeout, nil)
	decisionSinks := make([]sink.DecisionSink, 0, 2)
	var kafkaWG sync.WaitGroup
	if postgres != nil {
		auditWriter := store.NewAuditWriter(postgres.Pool(), cfg.AuditQueueSize, cfg.AuditBatchSize)
		go auditWriter.Run(ctx)
		decisionSinks = append(decisionSinks, auditWriter)
	}
	var franzPublisher *sink.FranzPublisher
	if cfg.KafkaEnabled {
		var err error
		franzPublisher, err = sink.NewFranzPublisher(cfg.KafkaBrokers, cfg.KafkaClientID)
		if err != nil {
			logger.Fatal().Err(err).Msg("kafka publisher init failed")
		}
		kafkaSink := sink.NewKafkaDecisionSink(sink.KafkaDecisionSinkConfig{
			Topic:     cfg.KafkaDecisionTopic,
			QueueSize: cfg.KafkaQueueSize,
			Publisher: franzPublisher,
		})
		kafkaWG.Add(1)
		go func() {
			defer kafkaWG.Done()
			kafkaSink.Run(ctx)
		}()
		decisionSinks = append(decisionSinks, kafkaSink)
	}
	var decisionSink engine.AuditSink
	if len(decisionSinks) == 1 {
		decisionSink = decisionSinks[0]
	} else if len(decisionSinks) > 1 {
		decisionSink = sink.MultiDecisionSink{Sinks: decisionSinks}
	}

	buildEngine := func(snapshot rules.Snapshot) *engine.RuleEngine {
		decisionEngine := engine.NewRuleEngine(snapshot.Version, snapshot.Rules).WithFeatureProvider(featureProvider)
		if decisionSink != nil {
			decisionEngine.WithAuditSink(decisionSink)
		}
		return decisionEngine
	}

	snapshot, err := rules.LoadFile(cfg.RulesPath, rules.LoaderOptions{
		Redis: redisStore.Client(),
	})
	if err != nil {
		logger.Fatal().Err(err).Str("path", cfg.RulesPath).Msg("rules load failed")
	}
	holder := newEngineHolder(buildEngine(snapshot))
	reload := func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		snapshot, err := rules.LoadFile(cfg.RulesPath, rules.LoaderOptions{
			Redis: redisStore.Client(),
		})
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		holder.Store(buildEngine(snapshot))
		logger.Info().Str("rules_version", snapshot.Version).Msg("rules reloaded")
		return nil
	}

	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-hup:
				reloadCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				if err := reload(reloadCtx); err != nil {
					logger.Error().Err(err).Msg("rules reload failed")
				}
				cancel()
			}
		}
	}()

	router := api.NewRouter(api.Dependencies{
		Engine: holder,
		Reload: reload,
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

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
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
	<-shutdownDone
	kafkaWG.Wait()
	if franzPublisher != nil {
		franzPublisher.Close()
	}
}
