package rules

import (
	"context"
	"errors"
	"strings"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
	"github.com/google/cel-go/cel"
)

type CELRule struct {
	id      string
	expr    string
	onMatch RuleAction
	program cel.Program
}

func NewCELRule(cfg CELRuleConfig) (*CELRule, error) {
	if strings.TrimSpace(cfg.ID) == "" {
		return nil, errors.New("cel rule id is required")
	}
	if strings.TrimSpace(cfg.Expr) == "" {
		return nil, errors.New("cel rule expr is required")
	}
	if err := cfg.OnMatch.validate("cel on_match"); err != nil {
		return nil, err
	}
	env, err := cel.NewEnv(
		cel.Variable("actor", cel.DynType),
		cel.Variable("action", cel.DynType),
		cel.Variable("content", cel.DynType),
		cel.Variable("context", cel.DynType),
	)
	if err != nil {
		return nil, err
	}
	ast, issues := env.Compile(cfg.Expr)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}
	program, err := env.Program(ast)
	if err != nil {
		return nil, err
	}
	return &CELRule{id: cfg.ID, expr: cfg.Expr, onMatch: cfg.OnMatch, program: program}, nil
}

func (r *CELRule) ID() string {
	return r.id
}

func (r *CELRule) Evaluate(ctx context.Context, req engine.DecideRequest) (engine.RuleResult, error) {
	select {
	case <-ctx.Done():
		return engine.RuleResult{}, ctx.Err()
	default:
	}
	out, _, err := r.program.Eval(map[string]any{
		"actor":   actorActivation(req),
		"action":  map[string]any{"type": req.Action.Type, "platform": req.Action.Platform},
		"content": map[string]any{"title": req.Content.Title, "description": req.Content.Description, "duration_s": req.Content.DurationS, "tags": req.Content.Tags},
		"context": req.Context,
	})
	if err != nil {
		return engine.RuleResult{}, err
	}
	matched, ok := out.Value().(bool)
	if !ok || !matched {
		return engine.RuleResult{RuleID: r.id, Matched: false, Verdict: engine.VerdictAllow}, nil
	}
	return engine.RuleResult{
		RuleID:  r.id,
		Matched: true,
		Verdict: r.onMatch.Verdict,
		Reason:  engine.Reason{Code: r.onMatch.Code, Rule: r.id},
	}, nil
}

func actorActivation(req engine.DecideRequest) map[string]any {
	actor := map[string]any{
		"id": req.ActorID,
	}
	if req.Context == nil {
		return actor
	}
	if raw, ok := req.Context["actor"].(map[string]any); ok {
		for key, value := range raw {
			actor[key] = value
		}
	}
	return actor
}
