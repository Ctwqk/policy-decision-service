package engine

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type AllowEngine struct {
	rulesVersion string
}

func NewAllowEngine(rulesVersion string) *AllowEngine {
	if rulesVersion == "" {
		rulesVersion = "bootstrap"
	}
	return &AllowEngine{rulesVersion: rulesVersion}
}

func (e *AllowEngine) Evaluate(ctx context.Context, req DecideRequest) (DecideResponse, error) {
	started := time.Now()
	select {
	case <-ctx.Done():
		return DecideResponse{}, ctx.Err()
	default:
	}
	return DecideResponse{
		DecisionID:     uuid.NewString(),
		Verdict:        VerdictAllow,
		Score:          0,
		Reasons:        []Reason{},
		EvaluatedRules: []string{},
		RulesVersion:   e.rulesVersion,
		LatencyMS:      time.Since(started).Milliseconds(),
	}, nil
}
