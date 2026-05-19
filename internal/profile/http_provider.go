package profile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Ctwqk/policy-decision-service/internal/telemetry"
)

const defaultHTTPFeatureTimeout = 100 * time.Millisecond

type HTTPFeatureProvider struct {
	baseURL string
	timeout time.Duration
	client  *http.Client
}

func NewHTTPFeatureProvider(baseURL string, timeout time.Duration, client *http.Client) *HTTPFeatureProvider {
	if timeout <= 0 {
		timeout = defaultHTTPFeatureTimeout
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPFeatureProvider{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		timeout: timeout,
		client:  client,
	}
}

func (p *HTTPFeatureProvider) GetActorFeatures(ctx context.Context, actorID string) (features ActorFeatures, degraded bool) {
	started := time.Now()
	defer func() {
		telemetry.FeatureLookupLatencySeconds.Observe(time.Since(started).Seconds())
		if degraded {
			telemetry.FeatureLookupDegradedTotal.Inc()
		}
	}()

	actorID = strings.TrimSpace(actorID)
	if p == nil || p.baseURL == "" || actorID == "" {
		return ActorFeatures{}, true
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	endpoint := p.baseURL + "/v1/features/" + url.PathEscape(actorID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ActorFeatures{}, true
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return ActorFeatures{}, true
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ActorFeatures{}, true
	}

	if err := json.NewDecoder(resp.Body).Decode(&features); err != nil {
		return ActorFeatures{}, true
	}
	if features.ActorID == "" {
		features.ActorID = actorID
	} else if features.ActorID != actorID {
		return ActorFeatures{}, true
	}
	return features, false
}
