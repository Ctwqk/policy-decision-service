package rules

import (
	"context"
	"testing"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
)

func TestCombinerRuleAllRequiresEveryReferencedMatch(t *testing.T) {
	rule, err := NewCombinerRule(CombinerRuleConfig{
		ID: "combo_high_risk_publish",
		Op: "all",
		Of: []string{"rate_limit_publish_daily", "new_actor_review"},
		OnMatch: RuleAction{
			Verdict: engine.VerdictBlock,
			Code:    "high_risk_actor_blocked",
		},
	})
	if err != nil {
		t.Fatalf("new combiner: %v", err)
	}

	prior := map[string]engine.RuleResult{
		"rate_limit_publish_daily": {RuleID: "rate_limit_publish_daily", Matched: true, Verdict: engine.VerdictBlock},
		"new_actor_review":         {RuleID: "new_actor_review", Matched: true, Verdict: engine.VerdictFlag},
	}
	result, err := rule.EvaluateWithResults(context.Background(), engine.DecideRequest{}, prior)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !result.Matched || result.Verdict != engine.VerdictBlock {
		t.Fatalf("expected combo block, got %#v", result)
	}

	prior["new_actor_review"] = engine.RuleResult{RuleID: "new_actor_review", Matched: false, Verdict: engine.VerdictAllow}
	result, err = rule.EvaluateWithResults(context.Background(), engine.DecideRequest{}, prior)
	if err != nil {
		t.Fatalf("evaluate missing: %v", err)
	}
	if result.Matched {
		t.Fatalf("expected all combiner to skip when one dep does not match, got %#v", result)
	}
}

func TestCombinerRuleAnyMatchesOneReferencedRule(t *testing.T) {
	rule, err := NewCombinerRule(CombinerRuleConfig{
		ID:      "any_combo",
		Op:      "any",
		Of:      []string{"a", "b"},
		OnMatch: RuleAction{Verdict: engine.VerdictFlag, Code: "any_flag"},
	})
	if err != nil {
		t.Fatalf("new combiner: %v", err)
	}

	result, err := rule.EvaluateWithResults(context.Background(), engine.DecideRequest{}, map[string]engine.RuleResult{
		"a": {RuleID: "a", Matched: false, Verdict: engine.VerdictAllow},
		"b": {RuleID: "b", Matched: true, Verdict: engine.VerdictBlock},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !result.Matched || result.Reason.Code != "any_flag" {
		t.Fatalf("expected any combo match, got %#v", result)
	}
}
