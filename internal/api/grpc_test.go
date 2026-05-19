package api_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/Ctwqk/policy-decision-service/internal/api"
	"github.com/Ctwqk/policy-decision-service/internal/engine"
	pdsv1 "github.com/Ctwqk/policy-decision-service/proto/gen/pds/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"
)

type captureEngine struct {
	lastRequest engine.DecideRequest
	response    engine.DecideResponse
}

func (e *captureEngine) Evaluate(_ context.Context, req engine.DecideRequest) (engine.DecideResponse, error) {
	e.lastRequest = req
	if e.response.DecisionID == "" {
		return engine.DecideResponse{DecisionID: "decision-1", Verdict: engine.VerdictAllow}, nil
	}
	return e.response, nil
}

func TestGRPCDecidePreservesStructuredContext(t *testing.T) {
	fake := &captureEngine{}
	client, cleanup := newGRPCTestClient(t, fake)
	defer cleanup()
	ctx := mustStruct(t, map[string]any{
		"actor": map[string]any{
			"age_days": float64(14),
			"trusted":  true,
		},
		"risk_score": float64(0.7),
		"labels":     []any{"new", "review"},
	})

	_, err := client.Decide(context.Background(), &pdsv1.DecideRequest{
		ActorId: "actor-1",
		Action:  &pdsv1.Action{Type: "publish", Platform: "youtube"},
		Content: &pdsv1.Content{Title: "demo"},
		Context: ctx,
	})
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}

	actor, ok := fake.lastRequest.Context["actor"].(map[string]any)
	if !ok {
		t.Fatalf("expected structured actor context, got %#v", fake.lastRequest.Context["actor"])
	}
	if actor["age_days"] != float64(14) {
		t.Fatalf("expected actor.age_days to survive conversion, got %#v", actor["age_days"])
	}
	if actor["trusted"] != true {
		t.Fatalf("expected actor.trusted to survive conversion, got %#v", actor["trusted"])
	}
	if fake.lastRequest.Context["risk_score"] != float64(0.7) {
		t.Fatalf("expected risk_score to survive conversion, got %#v", fake.lastRequest.Context["risk_score"])
	}
	labels, ok := fake.lastRequest.Context["labels"].([]any)
	if !ok || len(labels) != 2 || labels[0] != "new" || labels[1] != "review" {
		t.Fatalf("expected labels array to survive conversion, got %#v", fake.lastRequest.Context["labels"])
	}
	if fake.lastRequest.ClientID != "grpc" {
		t.Fatalf("expected grpc client id, got %q", fake.lastRequest.ClientID)
	}
}

func TestGRPCDecidePreservesResponseMetadata(t *testing.T) {
	fake := &captureEngine{
		response: engine.DecideResponse{
			DecisionID:     "decision-2",
			Verdict:        engine.VerdictFlag,
			Score:          0.8,
			EvaluatedRules: []string{"rule-a"},
			RulesVersion:   "v1",
			LatencyMS:      3,
			Metadata: map[string]any{
				"warnings": []string{"feature_provider_unavailable"},
				"audit": map[string]any{
					"queued": true,
				},
			},
		},
	}
	client, cleanup := newGRPCTestClient(t, fake)
	defer cleanup()

	resp, err := client.Decide(context.Background(), &pdsv1.DecideRequest{
		ActorId: "actor-1",
		Action:  &pdsv1.Action{Type: "publish"},
	})
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}

	metadata := resp.GetMetadata().AsMap()
	warnings, ok := metadata["warnings"].([]any)
	if !ok || len(warnings) != 1 || warnings[0] != "feature_provider_unavailable" {
		t.Fatalf("expected metadata warnings to survive conversion, got %#v", metadata["warnings"])
	}
	audit, ok := metadata["audit"].(map[string]any)
	if !ok || audit["queued"] != true {
		t.Fatalf("expected nested metadata to survive conversion, got %#v", metadata["audit"])
	}
}

func TestGRPCDecideMapsInvalidRequestToInvalidArgument(t *testing.T) {
	client, cleanup := newGRPCTestClient(t, &captureEngine{})
	defer cleanup()

	_, err := client.Decide(context.Background(), &pdsv1.DecideRequest{
		Action: &pdsv1.Action{Type: "publish"},
	})
	if err == nil {
		t.Fatalf("expected invalid request error")
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", got)
	}
}

func newGRPCTestClient(t *testing.T, decisionEngine api.DecisionEngine) (pdsv1.PolicyDecisionServiceClient, func()) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := api.NewGRPCServer(decisionEngine)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("Serve returned error: %v", err)
		}
	}()
	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	cleanup := func() {
		_ = conn.Close()
		server.Stop()
		_ = listener.Close()
	}
	return pdsv1.NewPolicyDecisionServiceClient(conn), cleanup
}

func mustStruct(t *testing.T, values map[string]any) *structpb.Struct {
	t.Helper()
	out, err := structpb.NewStruct(values)
	if err != nil {
		t.Fatalf("build struct: %v", err)
	}
	return out
}
