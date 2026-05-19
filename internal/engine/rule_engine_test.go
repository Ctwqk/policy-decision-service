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
