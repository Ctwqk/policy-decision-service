package engine

import "testing"

func TestCombineUsesBlockFlagAllowPrecedence(t *testing.T) {
	result := Combine([]RuleResult{
		{RuleID: "allow_rule", Matched: true, Verdict: VerdictAllow},
		{RuleID: "flag_rule", Matched: true, Verdict: VerdictFlag, Reason: Reason{Code: "needs_review", Rule: "flag_rule"}},
		{RuleID: "block_rule", Matched: true, Verdict: VerdictBlock, Reason: Reason{Code: "hard_block", Rule: "block_rule"}},
	})

	if result.Verdict != VerdictBlock {
		t.Fatalf("expected block to win, got %q", result.Verdict)
	}
	if len(result.Reasons) != 2 {
		t.Fatalf("expected matched reasons from flag and block, got %#v", result.Reasons)
	}
	if len(result.EvaluatedRules) != 3 {
		t.Fatalf("expected all rules evaluated, got %#v", result.EvaluatedRules)
	}
}

func TestCombineReturnsAllowWhenNoRulesMatch(t *testing.T) {
	result := Combine([]RuleResult{
		{RuleID: "rule_1", Matched: false, Verdict: VerdictBlock},
	})

	if result.Verdict != VerdictAllow {
		t.Fatalf("expected allow, got %q", result.Verdict)
	}
	if len(result.Reasons) != 0 {
		t.Fatalf("expected no reasons, got %#v", result.Reasons)
	}
}
