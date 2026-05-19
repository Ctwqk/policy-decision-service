package profile

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFallbackProviderReturnsFeaturesFromFirstSuccessfulProvider(t *testing.T) {
	provider := FallbackProvider{
		Providers: []Provider{
			StaticProvider{Err: errors.New("redis unavailable")},
			StaticProvider{Features: ActorFeatures{Publishes1H: 3, Flags7D: 2}},
		},
	}

	features, degraded := provider.GetActorFeatures(context.Background(), "actor-1")

	if degraded {
		t.Fatalf("expected successful fallback without degraded=true")
	}
	if features.Publishes1H != 3 || features.Flags7D != 2 {
		t.Fatalf("unexpected features: %+v", features)
	}
}

func TestFallbackProviderFailsOpenWhenAllProvidersFail(t *testing.T) {
	provider := FallbackProvider{
		Providers: []Provider{
			StaticProvider{Err: errors.New("redis unavailable")},
			StaticProvider{Err: errors.New("http unavailable")},
		},
	}

	features, degraded := provider.GetActorFeatures(context.Background(), "actor-1")

	if !degraded {
		t.Fatalf("expected degraded=true")
	}
	if features != (ActorFeatures{}) {
		t.Fatalf("expected zero-value fail-open features, got %+v", features)
	}
}

func TestHTTPFeatureProviderParsesAggregatorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/features/actor-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"actor_id":"actor-1","publishes_5m":1,"publishes_1h":4,"publishes_24h":9,"blocks_24h":2,"flags_7d":7,"comment_burst_1m":3,"as_of":"2026-05-19T00:00:00Z","from_cache":true}`))
	}))
	defer server.Close()

	provider := NewHTTPFeatureProvider(server.URL, 200*time.Millisecond, server.Client())
	features, degraded := provider.GetActorFeatures(context.Background(), "actor-1")

	if degraded {
		t.Fatalf("expected degraded=false")
	}
	if features.Publishes5M != 1 || features.Publishes1H != 4 || features.Blocks24H != 2 || !features.FromCache {
		t.Fatalf("unexpected features: %+v", features)
	}
}

func TestHTTPFeatureProviderTrimsRequestedActorID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/features/actor-1" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"actor_id":"actor-1","publishes_5m":2}`))
	}))
	defer server.Close()

	provider := NewHTTPFeatureProvider(server.URL, 200*time.Millisecond, server.Client())
	features, degraded := provider.GetActorFeatures(context.Background(), " actor-1 ")

	if degraded {
		t.Fatalf("expected degraded=false")
	}
	if features.ActorID != "actor-1" || features.Publishes5M != 2 {
		t.Fatalf("unexpected features: %+v", features)
	}
}

func TestHTTPFeatureProviderFillsMissingActorID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"publishes_5m":2}`))
	}))
	defer server.Close()

	provider := NewHTTPFeatureProvider(server.URL, 200*time.Millisecond, server.Client())
	features, degraded := provider.GetActorFeatures(context.Background(), "actor-1")

	if degraded {
		t.Fatalf("expected degraded=false")
	}
	if features.ActorID != "actor-1" || features.Publishes5M != 2 {
		t.Fatalf("unexpected features: %+v", features)
	}
}

func TestHTTPFeatureProviderFailsOpenOnActorIDMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"actor_id":"actor-2","publishes_5m":9}`))
	}))
	defer server.Close()

	provider := NewHTTPFeatureProvider(server.URL, 200*time.Millisecond, server.Client())
	features, degraded := provider.GetActorFeatures(context.Background(), "actor-1")

	if !degraded {
		t.Fatalf("expected degraded=true")
	}
	if features != (ActorFeatures{}) {
		t.Fatalf("expected zero-value fail-open features, got %+v", features)
	}
}

func TestHTTPFeatureProviderFailsOpenOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	provider := NewHTTPFeatureProvider(server.URL, 200*time.Millisecond, server.Client())
	features, degraded := provider.GetActorFeatures(context.Background(), "actor-1")

	if !degraded {
		t.Fatalf("expected degraded=true")
	}
	if features != (ActorFeatures{}) {
		t.Fatalf("expected zero-value fail-open features, got %+v", features)
	}
}

func TestHTTPFeatureProviderFailsOpenOnMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"actor_id":`))
	}))
	defer server.Close()

	provider := NewHTTPFeatureProvider(server.URL, 200*time.Millisecond, server.Client())
	features, degraded := provider.GetActorFeatures(context.Background(), "actor-1")

	if !degraded {
		t.Fatalf("expected degraded=true")
	}
	if features != (ActorFeatures{}) {
		t.Fatalf("expected zero-value fail-open features, got %+v", features)
	}
}

func TestHTTPFeatureProviderFailsOpenOnContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("request should not reach server after context cancellation")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	provider := NewHTTPFeatureProvider(server.URL, 200*time.Millisecond, server.Client())
	features, degraded := provider.GetActorFeatures(ctx, "actor-1")

	if !degraded {
		t.Fatalf("expected degraded=true")
	}
	if features != (ActorFeatures{}) {
		t.Fatalf("expected zero-value fail-open features, got %+v", features)
	}
}
