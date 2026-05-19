package rules

import (
	"context"
	"testing"
	"time"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRateLimitRuleBlocksWhenLimitExceeded(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	now := time.Date(2026, 5, 19, 8, 0, 0, 0, time.UTC)
	rule, err := NewRateLimitRule(RateLimitRuleConfig{
		ID:     "rate_limit_publish_daily",
		Scope:  "actor",
		Action: "publish_video",
		Window: time.Hour,
		Limit:  2,
		OnExceed: RuleAction{
			Verdict: engine.VerdictBlock,
			Code:    "daily_publish_quota_exceeded",
		},
		Now: func() time.Time { return now },
	}, client)
	if err != nil {
		t.Fatalf("new rate limit rule: %v", err)
	}

	req := engine.DecideRequest{
		ActorID: "actor-1",
		Action:  engine.ActionContext{Type: "publish_video"},
	}
	for i := 0; i < 2; i++ {
		result, err := rule.Evaluate(context.Background(), engine.EvalState{Request: req})
		if err != nil {
			t.Fatalf("evaluate %d: %v", i, err)
		}
		if result.Matched {
			t.Fatalf("request %d should be allowed, got %#v", i, result)
		}
	}

	result, err := rule.Evaluate(context.Background(), engine.EvalState{Request: req})
	if err != nil {
		t.Fatalf("evaluate exceeded: %v", err)
	}
	if !result.Matched || result.Verdict != engine.VerdictBlock {
		t.Fatalf("expected block after limit exceeded, got %#v", result)
	}
	if result.Reason.Code != "daily_publish_quota_exceeded" {
		t.Fatalf("unexpected reason: %#v", result.Reason)
	}
}

func TestRateLimitRuleIgnoresOtherActions(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	rule, err := NewRateLimitRule(RateLimitRuleConfig{
		ID:       "publish_only",
		Scope:    "actor",
		Action:   "publish_video",
		Window:   time.Hour,
		Limit:    1,
		OnExceed: RuleAction{Verdict: engine.VerdictBlock, Code: "blocked"},
	}, client)
	if err != nil {
		t.Fatalf("new rate limit rule: %v", err)
	}

	result, err := rule.Evaluate(context.Background(), engine.EvalState{
		Request: engine.DecideRequest{
			ActorID: "actor-1",
			Action:  engine.ActionContext{Type: "post_comment"},
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Matched {
		t.Fatalf("expected other action to skip rule, got %#v", result)
	}
}
