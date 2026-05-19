package profile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
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

func (p *HTTPFeatureProvider) GetActorFeatures(ctx context.Context, actorID string) (ActorFeatures, bool) {
	if p == nil || p.baseURL == "" || strings.TrimSpace(actorID) == "" {
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

	var features ActorFeatures
	if err := json.NewDecoder(resp.Body).Decode(&features); err != nil {
		return ActorFeatures{}, true
	}
	if features.ActorID == "" {
		features.ActorID = actorID
	}
	return features, false
}
