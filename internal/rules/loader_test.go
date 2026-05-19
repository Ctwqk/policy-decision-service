package rules

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
)

func TestLoadBytesRejectsUnknownRuleType(t *testing.T) {
	_, err := LoadBytes([]byte(`
version: 1
rules:
  - id: mystery
    type: surprise
    enabled: true
`), LoaderOptions{})
	if err == nil || !strings.Contains(err.Error(), "unknown rule type") {
		t.Fatalf("expected unknown rule type error, got %v", err)
	}
}

func TestLoadBytesRejectsDuplicateRuleIDs(t *testing.T) {
	_, err := LoadBytes([]byte(`
version: 1
rules:
  - id: repeated
    type: keyword_match
    enabled: false
    field: content.title
    keywords_file: blocklist.txt
    on_match: {verdict: block, code: blocked}
  - id: repeated
    type: keyword_match
    enabled: false
    field: content.title
    keywords_file: blocklist.txt
    on_match: {verdict: block, code: blocked}
`), LoaderOptions{})
	if err == nil || !strings.Contains(err.Error(), "duplicate rule id") {
		t.Fatalf("expected duplicate rule id error, got %v", err)
	}
}

func TestLoadBytesRejectsCombinerMissingReference(t *testing.T) {
	_, err := LoadBytes([]byte(`
version: 1
rules:
  - id: combo
    type: combiner
    enabled: true
    op: all
    of: [missing_rule]
    on_match: {verdict: block, code: combo_block}
`), LoaderOptions{})
	if err == nil || !strings.Contains(err.Error(), "unknown referenced rule") {
		t.Fatalf("expected missing reference error, got %v", err)
	}
}

func TestLoadFileBuildsEnabledKeywordRuleAndSkipsDisabledRules(t *testing.T) {
	dir := t.TempDir()
	blocklistPath := filepath.Join(dir, "blocklist.txt")
	if err := os.WriteFile(blocklistPath, []byte("blocked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rulesPath := filepath.Join(dir, "rules.yaml")
	if err := os.WriteFile(rulesPath, []byte(`
version: 1
rules:
  - id: title_keyword_blocklist
    type: keyword_match
    enabled: true
    field: content.title
    keywords_file: blocklist.txt
    on_match: {verdict: block, code: title_blocked_keyword}
  - id: disabled_keyword
    type: keyword_match
    enabled: false
    field: content.title
    keywords_file: blocklist.txt
    on_match: {verdict: block, code: disabled}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot, err := LoadFile(rulesPath, LoaderOptions{})
	if err != nil {
		t.Fatalf("load file: %v", err)
	}
	if snapshot.Version == "" {
		t.Fatalf("expected rules version hash")
	}
	if len(snapshot.Rules) != 1 {
		t.Fatalf("expected one enabled rule, got %d", len(snapshot.Rules))
	}

	result, err := snapshot.Rules[0].Evaluate(context.Background(), engine.DecideRequest{
		ActorID: "actor-1",
		Action:  engine.ActionContext{Type: "publish_video"},
		Content: engine.ContentContext{Title: "this is blocked"},
	})
	if err != nil {
		t.Fatalf("evaluate keyword rule: %v", err)
	}
	if !result.Matched || result.Verdict != engine.VerdictBlock {
		t.Fatalf("expected block match, got %#v", result)
	}
}
