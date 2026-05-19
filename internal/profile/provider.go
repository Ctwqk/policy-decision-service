package profile

import (
	"context"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
)

type ActorFeatures = engine.ActorFeatures

type Provider interface {
	GetActorFeatures(context.Context, string) (ActorFeatures, bool)
}

type StaticProvider struct {
	Features ActorFeatures
	Err      error
}

func (p StaticProvider) GetActorFeatures(context.Context, string) (ActorFeatures, bool) {
	if p.Err != nil {
		return ActorFeatures{}, true
	}
	return p.Features, false
}

type FallbackProvider struct {
	Providers []Provider
}

func (p FallbackProvider) GetActorFeatures(ctx context.Context, actorID string) (ActorFeatures, bool) {
	for _, provider := range p.Providers {
		if provider == nil {
			continue
		}
		features, degraded := provider.GetActorFeatures(ctx, actorID)
		if !degraded {
			return features, false
		}
	}
	return ActorFeatures{}, true
}
