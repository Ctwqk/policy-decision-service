package rules

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
	"github.com/redis/go-redis/v9"
)

var rateLimitScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
redis.call("EXPIRE", KEYS[1], ARGV[1])
local previous = redis.call("GET", KEYS[2])
if not previous then
  previous = "0"
end
return {current, previous}
`)

type RateLimitRule struct {
	id       string
	scope    string
	action   string
	window   time.Duration
	limit    int64
	onExceed RuleAction
	now      func() time.Time
	redis    redis.Cmdable
}

func NewRateLimitRule(cfg RateLimitRuleConfig, client redis.Cmdable) (*RateLimitRule, error) {
	if strings.TrimSpace(cfg.ID) == "" {
		return nil, errors.New("rate limit rule id is required")
	}
	if client == nil {
		return nil, errors.New("rate limit rule requires redis client")
	}
	if cfg.Scope == "" {
		cfg.Scope = "actor"
	}
	if cfg.Scope != "actor" && cfg.Scope != "actor+action" && cfg.Scope != "global" {
		return nil, errors.New("rate limit scope must be actor, actor+action, or global")
	}
	if cfg.Window <= 0 {
		return nil, errors.New("rate limit window must be positive")
	}
	if cfg.Limit <= 0 {
		return nil, errors.New("rate limit limit must be positive")
	}
	if err := cfg.OnExceed.validate("rate limit on_exceed"); err != nil {
		return nil, err
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &RateLimitRule{
		id:       cfg.ID,
		scope:    cfg.Scope,
		action:   cfg.Action,
		window:   cfg.Window,
		limit:    cfg.Limit,
		onExceed: cfg.OnExceed,
		now:      cfg.Now,
		redis:    client,
	}, nil
}

func (r *RateLimitRule) ID() string {
	return r.id
}

func (r *RateLimitRule) Evaluate(ctx context.Context, req engine.DecideRequest) (engine.RuleResult, error) {
	if r.action != "" && req.Action.Type != r.action {
		return engine.RuleResult{RuleID: r.id, Matched: false, Verdict: engine.VerdictAllow}, nil
	}

	now := r.now().UTC()
	windowSeconds := int64(r.window.Seconds())
	if windowSeconds <= 0 {
		return engine.RuleResult{}, errors.New("rate limit window is below one second")
	}
	bucket := now.Unix() / windowSeconds
	keyPrefix := r.keyPrefix(req)
	currentKey := fmt.Sprintf("%s:%d", keyPrefix, bucket)
	previousKey := fmt.Sprintf("%s:%d", keyPrefix, bucket-1)
	ttlSeconds := int64(math.Ceil((2 * r.window).Seconds()))

	values, err := rateLimitScript.Run(ctx, r.redis, []string{currentKey, previousKey}, ttlSeconds).Slice()
	if err != nil {
		return engine.RuleResult{}, err
	}
	current, err := int64FromRedis(values[0])
	if err != nil {
		return engine.RuleResult{}, err
	}
	previous, err := int64FromRedis(values[1])
	if err != nil {
		return engine.RuleResult{}, err
	}

	elapsed := now.Unix() % windowSeconds
	previousWeight := float64(windowSeconds-elapsed) / float64(windowSeconds)
	estimated := float64(current) + float64(previous)*previousWeight
	if estimated <= float64(r.limit) {
		return engine.RuleResult{RuleID: r.id, Matched: false, Verdict: engine.VerdictAllow}, nil
	}
	return engine.RuleResult{
		RuleID:  r.id,
		Matched: true,
		Verdict: r.onExceed.Verdict,
		Reason:  engine.Reason{Code: r.onExceed.Code, Rule: r.id},
	}, nil
}

func (r *RateLimitRule) keyPrefix(req engine.DecideRequest) string {
	switch r.scope {
	case "global":
		return "pds:rl:global"
	case "actor+action":
		return fmt.Sprintf("pds:rl:%s:%s", req.ActorID, req.Action.Type)
	default:
		return fmt.Sprintf("pds:rl:%s:%s", req.ActorID, r.action)
	}
}

func int64FromRedis(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		var parsed int64
		_, err := fmt.Sscanf(typed, "%d", &parsed)
		return parsed, err
	case []byte:
		var parsed int64
		_, err := fmt.Sscanf(string(typed), "%d", &parsed)
		return parsed, err
	default:
		return 0, fmt.Errorf("unexpected redis integer type %T", value)
	}
}
