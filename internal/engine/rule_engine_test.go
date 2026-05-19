package engine_test

import (
	"context"
	"testing"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
	"github.com/Ctwqk/policy-decision-service/internal/rules"
)

func TestRuleEngineEvaluatesCombinerAfterDependencies(t *testing.T) {
	snapshot, err := rules.LoadBytes([]byte(`
version: 1
rules:
  - id: combo
    type: combiner
    enabled: true
    op: all
    of: [new_actor_review]
    on_match: {verdict: block, code: combo_block}
  - id: new_actor_review
    type: cel
    enabled: true
    expr: actor.age_days < 7 && action.type == "publish_video"
    on_match: {verdict: flag, code: new_actor_pending_review}
`), rules.LoaderOptions{})
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}

	response, err := engine.NewRuleEngine(snapshot.Version, snapshot.Rules).Evaluate(context.Background(), engine.DecideRequest{
		ActorID: "actor-1",
		Action:  engine.ActionContext{Type: "publish_video"},
		Context: map[string]any{"actor": map[string]any{"age_days": 3}},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if response.Verdict != engine.VerdictBlock {
		t.Fatalf("expected combiner block to win, got %#v", response)
	}
	if len(response.EvaluatedRules) != 2 || response.EvaluatedRules[0] != "new_actor_review" || response.EvaluatedRules[1] != "combo" {
		t.Fatalf("unexpected evaluated rules: %#v", response.EvaluatedRules)
	}
}

type captureStateRule struct {
	state engine.EvalState
}

func (r *captureStateRule) ID() string {
	return "capture_state"
}

func (r *captureStateRule) Evaluate(_ context.Context, state engine.EvalState) (engine.RuleResult, error) {
	r.state = state
	return engine.RuleResult{RuleID: r.ID(), Matched: false, Verdict: engine.VerdictAllow}, nil
}

type stubFeatureProvider struct {
	features engine.ActorFeatures
	degraded bool
}

func (p stubFeatureProvider) GetActorFeatures(context.Context, string) (engine.ActorFeatures, bool) {
	return p.features, p.degraded
}

func TestRuleEnginePopulatesEvalStateFromFeatureProvider(t *testing.T) {
	rule := &captureStateRule{}
	response, err := engine.NewRuleEngine("test", []engine.Rule{rule}).
		WithFeatureProvider(stubFeatureProvider{features: engine.ActorFeatures{ActorID: "actor-1", Publishes5M: 4}}).
		Evaluate(context.Background(), engine.DecideRequest{ActorID: "actor-1"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if rule.state.Features.Publishes5M != 4 || rule.state.FeatureDegraded {
		t.Fatalf("unexpected eval state: %+v", rule.state)
	}
	if response.Metadata != nil {
		t.Fatalf("expected no metadata on non-degraded feature lookup, got %#v", response.Metadata)
	}
}

func TestRuleEngineAddsWarningMetadataWhenFeatureProviderDegraded(t *testing.T) {
	rule := &captureStateRule{}
	response, err := engine.NewRuleEngine("test", []engine.Rule{rule}).
		WithFeatureProvider(stubFeatureProvider{degraded: true}).
		Evaluate(context.Background(), engine.DecideRequest{ActorID: "actor-1"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !rule.state.FeatureDegraded {
		t.Fatalf("expected degraded eval state")
	}
	warnings, ok := response.Metadata["warnings"].([]string)
	if !ok || len(warnings) != 1 || warnings[0] != "feature_provider_unavailable" {
		t.Fatalf("unexpected metadata warnings: %#v", response.Metadata)
	}
}
