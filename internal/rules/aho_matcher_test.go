package rules

import "testing"

func TestAhoMatcherMatchesOverlappingKeywords(t *testing.T) {
	matcher := newAhoMatcher([]string{"risk", "blocked phrase", "phrase"})

	if !matcher.match("clean prefix blocked phrase suffix") {
		t.Fatalf("expected matcher to find configured phrase")
	}
	if !matcher.match("risk appears first") {
		t.Fatalf("expected matcher to find shorter configured keyword")
	}
	if matcher.match("clean title") {
		t.Fatalf("expected matcher to ignore text without configured keywords")
	}
}
