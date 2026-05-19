package rules

import (
	"context"
	"errors"
	"strings"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
)

type KeywordRule struct {
	id       string
	field    string
	keywords []string
	onMatch  RuleAction
}

func NewKeywordRule(cfg KeywordRuleConfig) (*KeywordRule, error) {
	if strings.TrimSpace(cfg.ID) == "" {
		return nil, errors.New("keyword rule id is required")
	}
	if cfg.Field != "content.title" && cfg.Field != "content.description" {
		return nil, errors.New("keyword rule field must be content.title or content.description")
	}
	if err := cfg.OnMatch.validate("keyword on_match"); err != nil {
		return nil, err
	}
	keywords := make([]string, 0, len(cfg.Keywords))
	for _, keyword := range cfg.Keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword != "" {
			keywords = append(keywords, keyword)
		}
	}
	return &KeywordRule{
		id:       cfg.ID,
		field:    cfg.Field,
		keywords: keywords,
		onMatch:  cfg.OnMatch,
	}, nil
}

func (r *KeywordRule) ID() string {
	return r.id
}

func (r *KeywordRule) Evaluate(ctx context.Context, state engine.EvalState) (engine.RuleResult, error) {
	select {
	case <-ctx.Done():
		return engine.RuleResult{}, ctx.Err()
	default:
	}
	req := state.Request
	if len(r.keywords) == 0 {
		return engine.RuleResult{RuleID: r.id, Matched: false, Verdict: engine.VerdictAllow}, nil
	}

	value := ""
	switch r.field {
	case "content.title":
		value = req.Content.Title
	case "content.description":
		value = req.Content.Description
	}
	value = strings.ToLower(value)
	for _, keyword := range r.keywords {
		if strings.Contains(value, keyword) {
			return engine.RuleResult{
				RuleID:  r.id,
				Matched: true,
				Verdict: r.onMatch.Verdict,
				Reason:  engine.Reason{Code: r.onMatch.Code, Rule: r.id},
			}, nil
		}
	}
	return engine.RuleResult{RuleID: r.id, Matched: false, Verdict: engine.VerdictAllow}, nil
}
