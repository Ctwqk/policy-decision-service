package engine

type RuleResult struct {
	RuleID  string
	Matched bool
	Verdict Verdict
	Reason  Reason
	Err     error
}

func Combine(results []RuleResult) DecideResponse {
	verdict := VerdictAllow
	reasons := make([]Reason, 0)
	evaluated := make([]string, 0, len(results))
	for _, result := range results {
		evaluated = append(evaluated, result.RuleID)
		if !result.Matched {
			continue
		}
		if result.Reason.Code != "" {
			reasons = append(reasons, result.Reason)
		}
		if result.Verdict == VerdictBlock {
			verdict = VerdictBlock
			continue
		}
		if result.Verdict == VerdictFlag && verdict != VerdictBlock {
			verdict = VerdictFlag
		}
	}
	return DecideResponse{
		Verdict:        verdict,
		Reasons:        reasons,
		EvaluatedRules: evaluated,
	}
}
