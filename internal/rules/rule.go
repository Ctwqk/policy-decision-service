package rules

import (
	"errors"
	"time"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
)

type RuleAction struct {
	Verdict engine.Verdict `yaml:"verdict"`
	Code    string         `yaml:"code"`
}

func (a RuleAction) validate(context string) error {
	if a.Verdict == "" {
		return errors.New(context + " verdict is required")
	}
	switch a.Verdict {
	case engine.VerdictAllow, engine.VerdictFlag, engine.VerdictBlock:
	default:
		return errors.New(context + " verdict must be allow, flag, or block")
	}
	if a.Code == "" {
		return errors.New(context + " code is required")
	}
	return nil
}

type KeywordRuleConfig struct {
	ID       string
	Field    string
	Keywords []string
	OnMatch  RuleAction
}

type RateLimitRuleConfig struct {
	ID       string
	Scope    string
	Action   string
	Window   time.Duration
	Limit    int64
	OnExceed RuleAction
	Now      func() time.Time
}

type CELRuleConfig struct {
	ID      string
	Expr    string
	OnMatch RuleAction
}

type CombinerRuleConfig struct {
	ID      string
	Op      string
	Of      []string
	OnMatch RuleAction
}
