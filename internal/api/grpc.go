package api

import (
	"context"
	"encoding/json"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
	pdsv1 "github.com/Ctwqk/policy-decision-service/proto/gen/pds/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type policyDecisionGRPCServer struct {
	pdsv1.UnimplementedPolicyDecisionServiceServer

	engine DecisionEngine
}

func NewGRPCServer(decisionEngine DecisionEngine, opts ...grpc.ServerOption) *grpc.Server {
	if decisionEngine == nil {
		decisionEngine = engine.NewAllowEngine("bootstrap")
	}
	server := grpc.NewServer(opts...)
	pdsv1.RegisterPolicyDecisionServiceServer(server, &policyDecisionGRPCServer{
		engine: decisionEngine,
	})
	return server
}

func (s *policyDecisionGRPCServer) Decide(ctx context.Context, req *pdsv1.DecideRequest) (*pdsv1.DecideResponse, error) {
	engineReq := grpcRequestToEngine(req)
	if err := validateDecideRequest(engineReq); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	response, err := s.engine.Evaluate(ctx, engineReq)
	if err != nil {
		return nil, status.Error(codes.Internal, "decision engine failed")
	}
	return engineResponseToGRPC(response), nil
}

func grpcRequestToEngine(req *pdsv1.DecideRequest) engine.DecideRequest {
	var contextMap map[string]any
	if req.GetContext() != nil {
		contextMap = req.GetContext().AsMap()
	}
	return engine.DecideRequest{
		ActorID: req.GetActorId(),
		Action: engine.ActionContext{
			Type:     req.GetAction().GetType(),
			Platform: req.GetAction().GetPlatform(),
		},
		Content: engine.ContentContext{
			Title:       req.GetContent().GetTitle(),
			Description: req.GetContent().GetDescription(),
			DurationS:   int(req.GetContent().GetDurationS()),
			Tags:        req.GetContent().GetTags(),
		},
		Context:  contextMap,
		ClientID: "grpc",
	}
}

func engineResponseToGRPC(response engine.DecideResponse) *pdsv1.DecideResponse {
	reasons := make([]*pdsv1.Reason, 0, len(response.Reasons))
	for _, reason := range response.Reasons {
		reasons = append(reasons, &pdsv1.Reason{
			Code:   reason.Code,
			Rule:   reason.Rule,
			Detail: reason.Detail,
		})
	}
	metadata := mapToStruct(response.Metadata)
	return &pdsv1.DecideResponse{
		DecisionId:     response.DecisionID,
		Verdict:        string(response.Verdict),
		Score:          response.Score,
		Reasons:        reasons,
		EvaluatedRules: response.EvaluatedRules,
		RulesVersion:   response.RulesVersion,
		LatencyMs:      response.LatencyMS,
		Metadata:       metadata,
	}
}

func mapToStruct(values map[string]any) *structpb.Struct {
	if len(values) == 0 {
		return nil
	}
	normalized, err := normalizeJSONMap(values)
	if err != nil {
		return nil
	}
	out, err := structpb.NewStruct(normalized)
	if err != nil {
		return nil
	}
	return out
}

func normalizeJSONMap(values map[string]any) (map[string]any, error) {
	data, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	var normalized map[string]any
	if err := json.Unmarshal(data, &normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}
