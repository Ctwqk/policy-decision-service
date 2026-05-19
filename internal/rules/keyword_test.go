package rules

import (
	"context"
	"testing"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
)

func TestKeywordRuleMatchesConfiguredField(t *testing.T) {
	rule, err := NewKeywordRule(KeywordRuleConfig{
		ID:       "title_keyword_blocklist",
		Field:    "content.title",
		Keywords: []string{"blocked phrase"},
		OnMatch: RuleAction{
			Verdict: engine.VerdictBlock,
			Code:    "title_blocked_keyword",
		},
	})
	if err != nil {
		t.Fatalf("new keyword rule: %v", err)
	}

	result, err := rule.Evaluate(context.Background(), engine.EvalState{
		Request: engine.DecideRequest{
			ActorID: "actor-1",
			Action:  engine.ActionContext{Type: "publish_video"},
			Content: engine.ContentContext{Title: "A BLOCKED phrase appears"},
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected keyword match")
	}
	if result.Reason.Code != "title_blocked_keyword" || result.Reason.Rule != "title_keyword_blocklist" {
		t.Fatalf("unexpected reason: %#v", result.Reason)
	}
}

func TestKeywordRuleWithEmptyKeywordsDoesNotMatch(t *testing.T) {
	rule, err := NewKeywordRule(KeywordRuleConfig{
		ID:       "empty",
		Field:    "content.title",
		Keywords: []string{},
		OnMatch:  RuleAction{Verdict: engine.VerdictBlock, Code: "blocked"},
	})
	if err != nil {
		t.Fatalf("new keyword rule: %v", err)
	}

	result, err := rule.Evaluate(context.Background(), engine.EvalState{
		Request: engine.DecideRequest{
			ActorID: "actor-1",
			Action:  engine.ActionContext{Type: "publish_video"},
			Content: engine.ContentContext{Title: "anything"},
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Matched {
		t.Fatalf("expected empty keyword list to be no-op")
	}
}
