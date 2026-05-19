package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Ctwqk/policy-decision-service/internal/api"
	"github.com/Ctwqk/policy-decision-service/internal/engine"
)

func TestHealthzReturnsOKWithoutDependencies(t *testing.T) {
	router := api.NewRouter(api.Dependencies{
		Engine: engine.NewAllowEngine("bootstrap"),
		Ready:  func(context.Context) error { return nil },
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestReadyzReturnsUnavailableWhenReadinessFails(t *testing.T) {
	router := api.NewRouter(api.Dependencies{
		Engine: engine.NewAllowEngine("bootstrap"),
		Ready:  func(context.Context) error { return errors.New("rules not loaded") },
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMetricsEndpointReturnsPrometheusText(t *testing.T) {
	router := api.NewRouter(api.Dependencies{
		Engine: engine.NewAllowEngine("bootstrap"),
		Ready:  func(context.Context) error { return nil },
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Fatalf("expected prometheus content type header")
	}
}

func TestDecideRequiresClientID(t *testing.T) {
	router := api.NewRouter(api.Dependencies{
		Engine: engine.NewAllowEngine("bootstrap"),
		Ready:  func(context.Context) error { return nil },
	})
	body := []byte(`{"actor_id":"channel_demo","action":{"type":"publish_video","platform":"youtube"},"content":{"title":"demo"},"context":{}}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/decide", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDecideReturnsAllowDecisionForValidRequest(t *testing.T) {
	router := api.NewRouter(api.Dependencies{
		Engine: engine.NewAllowEngine("bootstrap"),
		Ready:  func(context.Context) error { return nil },
	})
	body := []byte(`{"actor_id":"channel_demo","action":{"type":"publish_video","platform":"youtube"},"content":{"title":"demo","duration_s":30,"tags":["shorts"]},"context":{"session_id":"local"}}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/decide", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Id", "vp-local")
	req.Header.Set("X-Request-Id", "req-1")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response engine.DecideResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode decide response: %v", err)
	}
	if response.DecisionID == "" {
		t.Fatalf("expected decision_id")
	}
	if response.Verdict != engine.VerdictAllow {
		t.Fatalf("expected allow verdict, got %q", response.Verdict)
	}
	if response.RulesVersion != "bootstrap" {
		t.Fatalf("expected bootstrap rules version, got %q", response.RulesVersion)
	}
	if len(response.Reasons) != 0 {
		t.Fatalf("expected no reasons, got %#v", response.Reasons)
	}
	if len(response.EvaluatedRules) != 0 {
		t.Fatalf("expected no evaluated rules, got %#v", response.EvaluatedRules)
	}
	if response.LatencyMS < 0 {
		t.Fatalf("expected non-negative latency, got %d", response.LatencyMS)
	}
}
