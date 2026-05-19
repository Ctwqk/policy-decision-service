package store

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
	"github.com/Ctwqk/policy-decision-service/internal/telemetry"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditWriter struct {
	pool      *pgxpool.Pool
	queue     chan engine.AuditRecord
	batchSize int
	dropped   atomic.Int64
}

func NewAuditWriter(pool *pgxpool.Pool, queueSize int, batchSize int) *AuditWriter {
	if queueSize <= 0 {
		queueSize = 10000
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	return &AuditWriter{
		pool:      pool,
		queue:     make(chan engine.AuditRecord, queueSize),
		batchSize: batchSize,
	}
}

func (w *AuditWriter) Enqueue(ctx context.Context, record engine.AuditRecord) {
	if w == nil || w.pool == nil {
		return
	}
	select {
	case w.queue <- record:
	default:
		w.dropped.Add(1)
	}
}

func (w *AuditWriter) Dropped() int64 {
	if w == nil {
		return 0
	}
	return w.dropped.Load()
}

func (w *AuditWriter) Run(ctx context.Context) {
	if w == nil || w.pool == nil {
		return
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	batch := make([]engine.AuditRecord, 0, w.batchSize)
	for {
		select {
		case <-ctx.Done():
			_ = w.flush(context.Background(), batch)
			return
		case record := <-w.queue:
			batch = append(batch, record)
			if len(batch) >= w.batchSize {
				if err := w.flush(ctx, batch); err == nil {
					batch = batch[:0]
				}
			}
		case <-ticker.C:
			if len(batch) > 0 {
				if err := w.flush(ctx, batch); err == nil {
					batch = batch[:0]
				}
			}
		}
	}
}

func (w *AuditWriter) flush(ctx context.Context, records []engine.AuditRecord) error {
	for _, record := range records {
		reasons, err := json.Marshal(record.Reasons)
		if err != nil {
			telemetry.AuditWriteErrorsTotal.Inc()
			return err
		}
		request, err := json.Marshal(record.Request)
		if err != nil {
			telemetry.AuditWriteErrorsTotal.Inc()
			return err
		}
		_, err = w.pool.Exec(ctx, `
INSERT INTO pds.decisions (
  id, actor_id, action_type, platform, verdict, score, reasons,
  evaluated_rules, request, latency_us, rules_version, client
) VALUES (
  $1, $2, $3, $4, $5, $6, $7::jsonb,
  $8, $9::jsonb, $10, $11, $12
)
ON CONFLICT (id) DO NOTHING`,
			record.DecisionID,
			record.ActorID,
			record.ActionType,
			emptyStringToNil(record.Platform),
			string(record.Verdict),
			record.Score,
			string(reasons),
			record.EvaluatedRules,
			string(request),
			record.LatencyUS,
			record.RulesVersion,
			emptyStringToNil(record.Client),
		)
		if err != nil {
			telemetry.AuditWriteErrorsTotal.Inc()
			return err
		}
	}
	return nil
}

func emptyStringToNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}
