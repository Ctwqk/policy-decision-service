package sink

import (
	"context"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
)

type DecisionSink interface {
	Enqueue(context.Context, engine.AuditRecord)
}

type MultiDecisionSink struct {
	Sinks []DecisionSink
}

func (s MultiDecisionSink) Enqueue(ctx context.Context, record engine.AuditRecord) {
	for _, sink := range s.Sinks {
		if sink != nil {
			sink.Enqueue(ctx, record)
		}
	}
}
