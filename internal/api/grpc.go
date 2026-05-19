package api

import (
	"context"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
	pdsv1 "github.com/Ctwqk/policy-decision-service/proto/gen/pds/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	contextMap := make(map[string]any, len(req.GetContext()))
	for key, value := range req.GetContext() {
		contextMap[key] = value
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
	return &pdsv1.DecideResponse{
		DecisionId:     response.DecisionID,
		Verdict:        string(response.Verdict),
		Score:          response.Score,
		Reasons:        reasons,
		EvaluatedRules: response.EvaluatedRules,
		RulesVersion:   response.RulesVersion,
		LatencyMs:      response.LatencyMS,
	}
}
