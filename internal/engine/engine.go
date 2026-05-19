package engine

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Rule interface {
	ID() string
	Evaluate(context.Context, DecideRequest) (RuleResult, error)
}

type ResultAwareRule interface {
	Rule
	Dependencies() []string
	EvaluateWithResults(context.Context, DecideRequest, map[string]RuleResult) (RuleResult, error)
}

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

type RuleEngine struct {
	rulesVersion string
	rules        []Rule
	audit        AuditSink
}

type AuditSink interface {
	Enqueue(context.Context, AuditRecord)
}

func NewRuleEngine(rulesVersion string, rules []Rule) *RuleEngine {
	if rulesVersion == "" {
		rulesVersion = "bootstrap"
	}
	copied := append([]Rule(nil), rules...)
	return &RuleEngine{rulesVersion: rulesVersion, rules: copied}
}

func (e *RuleEngine) WithAuditSink(sink AuditSink) *RuleEngine {
	e.audit = sink
	return e
}

func (e *RuleEngine) Evaluate(ctx context.Context, req DecideRequest) (DecideResponse, error) {
	started := time.Now()
	results := make([]RuleResult, 0, len(e.rules))
	byID := make(map[string]RuleResult, len(e.rules))
	for _, rule := range e.rules {
		select {
		case <-ctx.Done():
			return DecideResponse{}, ctx.Err()
		default:
		}

		var result RuleResult
		var err error
		if aware, ok := rule.(ResultAwareRule); ok {
			result, err = aware.EvaluateWithResults(ctx, req, byID)
		} else {
			result, err = rule.Evaluate(ctx, req)
		}
		if err != nil {
			result = RuleResult{RuleID: rule.ID(), Matched: false, Verdict: VerdictAllow, Err: err}
		}
		results = append(results, result)
		byID[rule.ID()] = result
	}

	response := Combine(results)
	response.DecisionID = uuid.NewString()
	response.RulesVersion = e.rulesVersion
	response.LatencyMS = time.Since(started).Milliseconds()
	if response.Reasons == nil {
		response.Reasons = []Reason{}
	}
	if response.EvaluatedRules == nil {
		response.EvaluatedRules = []string{}
	}
	if e.audit != nil {
		e.audit.Enqueue(ctx, AuditRecord{
			DecisionID:     response.DecisionID,
			ActorID:        req.ActorID,
			ActionType:     req.Action.Type,
			Platform:       req.Action.Platform,
			Verdict:        response.Verdict,
			Score:          response.Score,
			Reasons:        response.Reasons,
			EvaluatedRules: response.EvaluatedRules,
			Request:        req,
			LatencyUS:      time.Since(started).Microseconds(),
			RulesVersion:   response.RulesVersion,
			Client:         req.ClientID,
		})
	}
	return response, nil
}
