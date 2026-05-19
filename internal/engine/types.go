package engine

type Verdict string

const (
	VerdictAllow Verdict = "allow"
	VerdictFlag  Verdict = "flag"
	VerdictBlock Verdict = "block"
)

type ActionContext struct {
	Type     string `json:"type"`
	Platform string `json:"platform,omitempty"`
}

type ContentContext struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	DurationS   int      `json:"duration_s,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type DecideRequest struct {
	ActorID   string         `json:"actor_id"`
	Action    ActionContext  `json:"action"`
	Content   ContentContext `json:"content"`
	Context   map[string]any `json:"context,omitempty"`
	ClientID  string         `json:"-"`
	RequestID string         `json:"-"`
}

type Reason struct {
	Code   string `json:"code"`
	Rule   string `json:"rule,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type DecideResponse struct {
	DecisionID     string   `json:"decision_id"`
	Verdict        Verdict  `json:"verdict"`
	Score          float64  `json:"score"`
	Reasons        []Reason `json:"reasons"`
	EvaluatedRules []string `json:"evaluated_rules"`
	RulesVersion   string   `json:"rules_version"`
	LatencyMS      int64    `json:"latency_ms"`
}
