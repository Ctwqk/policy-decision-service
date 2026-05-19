package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

type LoaderOptions struct {
	BaseDir string
	Redis   redis.Cmdable
	Now     func() time.Time
}

type Snapshot struct {
	Version string
	Rules   []engine.Rule
}

type ruleFile struct {
	Version  int       `yaml:"version"`
	Defaults defaults  `yaml:"defaults"`
	Rules    []rawRule `yaml:"rules"`
}

type defaults struct {
	OnEvalError string `yaml:"on_eval_error"`
}

type rawRule struct {
	ID           string     `yaml:"id"`
	Type         string     `yaml:"type"`
	Enabled      *bool      `yaml:"enabled"`
	Scope        string     `yaml:"scope"`
	Action       string     `yaml:"action"`
	Window       string     `yaml:"window"`
	Limit        int64      `yaml:"limit"`
	Field        string     `yaml:"field"`
	KeywordsFile string     `yaml:"keywords_file"`
	Expr         string     `yaml:"expr"`
	OnExceed     RuleAction `yaml:"on_exceed"`
	OnMatch      RuleAction `yaml:"on_match"`
	Op           string     `yaml:"op"`
	Of           []string   `yaml:"of"`
}

func LoadFile(path string, opts LoaderOptions) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	if opts.BaseDir == "" {
		opts.BaseDir = filepath.Dir(path)
	}
	return LoadBytes(data, opts)
}

func LoadBytes(data []byte, opts LoaderOptions) (Snapshot, error) {
	var parsed ruleFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return Snapshot{}, err
	}
	if parsed.Version != 1 {
		return Snapshot{}, fmt.Errorf("unsupported rules version %d", parsed.Version)
	}
	if parsed.Defaults.OnEvalError != "" && parsed.Defaults.OnEvalError != "allow" {
		return Snapshot{}, fmt.Errorf("defaults.on_eval_error must be allow")
	}

	ids := make(map[string]struct{}, len(parsed.Rules))
	for _, raw := range parsed.Rules {
		if strings.TrimSpace(raw.ID) == "" {
			return Snapshot{}, fmt.Errorf("rule id is required")
		}
		if _, exists := ids[raw.ID]; exists {
			return Snapshot{}, fmt.Errorf("duplicate rule id %q", raw.ID)
		}
		ids[raw.ID] = struct{}{}
		if !knownRuleType(raw.Type) {
			return Snapshot{}, fmt.Errorf("unknown rule type %q for %s", raw.Type, raw.ID)
		}
	}

	for _, raw := range parsed.Rules {
		if raw.Type != "combiner" {
			continue
		}
		for _, ref := range raw.Of {
			if _, exists := ids[ref]; !exists {
				return Snapshot{}, fmt.Errorf("combiner %s references unknown referenced rule %q", raw.ID, ref)
			}
		}
	}

	ordered, err := orderRules(parsed.Rules)
	if err != nil {
		return Snapshot{}, err
	}

	built := make([]engine.Rule, 0, len(ordered))
	for _, raw := range ordered {
		if !ruleEnabled(raw.Enabled) {
			continue
		}
		rule, err := buildRule(raw, opts)
		if err != nil {
			return Snapshot{}, err
		}
		if rule != nil {
			built = append(built, rule)
		}
	}

	sum := sha256.Sum256(data)
	return Snapshot{
		Version: "sha256:" + hex.EncodeToString(sum[:]),
		Rules:   built,
	}, nil
}

func knownRuleType(ruleType string) bool {
	switch ruleType {
	case "rate_limit", "keyword_match", "cel", "combiner":
		return true
	default:
		return false
	}
}

func ruleEnabled(enabled *bool) bool {
	return enabled == nil || *enabled
}

func buildRule(raw rawRule, opts LoaderOptions) (engine.Rule, error) {
	switch raw.Type {
	case "keyword_match":
		keywords, err := readKeywords(raw.KeywordsFile, opts.BaseDir)
		if err != nil {
			return nil, fmt.Errorf("keyword rule %s: %w", raw.ID, err)
		}
		return NewKeywordRule(KeywordRuleConfig{
			ID:       raw.ID,
			Field:    raw.Field,
			Keywords: keywords,
			OnMatch:  raw.OnMatch,
		})
	case "rate_limit":
		window, err := time.ParseDuration(raw.Window)
		if err != nil {
			return nil, fmt.Errorf("rate limit rule %s window: %w", raw.ID, err)
		}
		return NewRateLimitRule(RateLimitRuleConfig{
			ID:       raw.ID,
			Scope:    raw.Scope,
			Action:   raw.Action,
			Window:   window,
			Limit:    raw.Limit,
			OnExceed: raw.OnExceed,
			Now:      opts.Now,
		}, opts.Redis)
	case "cel":
		return NewCELRule(CELRuleConfig{
			ID:      raw.ID,
			Expr:    raw.Expr,
			OnMatch: raw.OnMatch,
		})
	case "combiner":
		return NewCombinerRule(CombinerRuleConfig{
			ID:      raw.ID,
			Op:      raw.Op,
			Of:      raw.Of,
			OnMatch: raw.OnMatch,
		})
	default:
		return nil, fmt.Errorf("unknown rule type %q for %s", raw.Type, raw.ID)
	}
}

func orderRules(raws []rawRule) ([]rawRule, error) {
	byID := make(map[string]rawRule, len(raws))
	for _, raw := range raws {
		byID[raw.ID] = raw
	}
	ordered := make([]rawRule, 0, len(raws))
	temporary := map[string]bool{}
	permanent := map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if permanent[id] {
			return nil
		}
		if temporary[id] {
			return fmt.Errorf("combiner cycle detected at %s", id)
		}
		raw, ok := byID[id]
		if !ok {
			return fmt.Errorf("unknown referenced rule %q", id)
		}
		temporary[id] = true
		if raw.Type == "combiner" {
			for _, ref := range raw.Of {
				if err := visit(ref); err != nil {
					return err
				}
			}
		}
		temporary[id] = false
		permanent[id] = true
		ordered = append(ordered, raw)
		return nil
	}
	for _, raw := range raws {
		if err := visit(raw.ID); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

func readKeywords(path string, baseDir string) ([]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("keywords_file is required")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	keywords := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			keywords = append(keywords, line)
		}
	}
	return keywords, nil
}
