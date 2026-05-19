package rules

import (
	"context"
	"testing"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
)

func TestCELRuleFlagsNewPublishActor(t *testing.T) {
	rule, err := NewCELRule(CELRuleConfig{
		ID:   "new_actor_review",
		Expr: `actor.age_days < 7 && action.type == "publish_video"`,
		OnMatch: RuleAction{
			Verdict: engine.VerdictFlag,
			Code:    "new_actor_pending_review",
		},
	})
	if err != nil {
		t.Fatalf("new cel rule: %v", err)
	}

	result, err := rule.Evaluate(context.Background(), engine.DecideRequest{
		ActorID: "actor-1",
		Action:  engine.ActionContext{Type: "publish_video"},
		Context: map[string]any{"actor": map[string]any{"age_days": 3}},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !result.Matched || result.Verdict != engine.VerdictFlag {
		t.Fatalf("expected flag match, got %#v", result)
	}
	if result.Reason.Code != "new_actor_pending_review" {
		t.Fatalf("unexpected reason: %#v", result.Reason)
	}
}

func TestCELRuleCompileErrorsFailAtLoad(t *testing.T) {
	_, err := NewCELRule(CELRuleConfig{
		ID:      "bad_expr",
		Expr:    `actor.age_days <`,
		OnMatch: RuleAction{Verdict: engine.VerdictFlag, Code: "bad"},
	})
	if err == nil {
		t.Fatalf("expected CEL compile error")
	}
}
