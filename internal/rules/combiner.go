package rules

import (
	"context"
	"errors"
	"strings"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
)

type CombinerRule struct {
	id      string
	op      string
	of      []string
	onMatch RuleAction
}

func NewCombinerRule(cfg CombinerRuleConfig) (*CombinerRule, error) {
	if strings.TrimSpace(cfg.ID) == "" {
		return nil, errors.New("combiner rule id is required")
	}
	if cfg.Op != "all" && cfg.Op != "any" {
		return nil, errors.New("combiner op must be all or any")
	}
	if len(cfg.Of) == 0 {
		return nil, errors.New("combiner requires referenced rules")
	}
	if err := cfg.OnMatch.validate("combiner on_match"); err != nil {
		return nil, err
	}
	return &CombinerRule{
		id:      cfg.ID,
		op:      cfg.Op,
		of:      append([]string(nil), cfg.Of...),
		onMatch: cfg.OnMatch,
	}, nil
}

func (r *CombinerRule) ID() string {
	return r.id
}

func (r *CombinerRule) Dependencies() []string {
	return append([]string(nil), r.of...)
}

func (r *CombinerRule) Evaluate(ctx context.Context, req engine.DecideRequest) (engine.RuleResult, error) {
	return r.EvaluateWithResults(ctx, req, nil)
}

func (r *CombinerRule) EvaluateWithResults(ctx context.Context, _ engine.DecideRequest, prior map[string]engine.RuleResult) (engine.RuleResult, error) {
	select {
	case <-ctx.Done():
		return engine.RuleResult{}, ctx.Err()
	default:
	}
	if prior == nil {
		return engine.RuleResult{RuleID: r.id, Matched: false, Verdict: engine.VerdictAllow}, nil
	}

	matchedCount := 0
	for _, ref := range r.of {
		result, ok := prior[ref]
		if !ok {
			return engine.RuleResult{}, errors.New("combiner dependency has not been evaluated")
		}
		if result.Matched {
			matchedCount++
		}
	}

	matched := matchedCount == len(r.of)
	if r.op == "any" {
		matched = matchedCount > 0
	}
	if !matched {
		return engine.RuleResult{RuleID: r.id, Matched: false, Verdict: engine.VerdictAllow}, nil
	}
	return engine.RuleResult{
		RuleID:  r.id,
		Matched: true,
		Verdict: r.onMatch.Verdict,
		Reason:  engine.Reason{Code: r.onMatch.Code, Rule: r.id},
	}, nil
}
