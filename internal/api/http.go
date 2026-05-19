package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type DecisionEngine interface {
	Evaluate(context.Context, engine.DecideRequest) (engine.DecideResponse, error)
}

type Dependencies struct {
	Engine DecisionEngine
	Ready  func(context.Context) error
	Reload func(context.Context) error
}

func NewRouter(deps Dependencies) http.Handler {
	if deps.Engine == nil {
		deps.Engine = engine.NewAllowEngine("bootstrap")
	}
	if deps.Ready == nil {
		deps.Ready = func(context.Context) error { return nil }
	}
	if deps.Reload == nil {
		deps.Reload = func(context.Context) error { return nil }
	}

	r := chi.NewRouter()
	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz(deps.Ready))
	r.Handle("/metrics", promhttp.Handler())
	r.Post("/v1/decide", decide(deps.Engine))
	r.Post("/v1/admin/reload", reload(deps.Reload))
	return r
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readyz(check func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := check(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"error":  err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func decide(decisionEngine DecisionEngine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID := strings.TrimSpace(r.Header.Get("X-Client-Id"))
		if clientID == "" {
			writeError(w, http.StatusBadRequest, "missing X-Client-Id")
			return
		}

		defer r.Body.Close()
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var req engine.DecideRequest
		if err := decoder.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "malformed request")
			return
		}
		if err := validateDecideRequest(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		req.ClientID = clientID
		req.RequestID = strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if req.RequestID == "" {
			req.RequestID = uuid.NewString()
		}

		response, err := decisionEngine.Evaluate(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "decision engine failed")
			return
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func reload(fn func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "reloaded"})
	}
}

func validateDecideRequest(req engine.DecideRequest) error {
	if strings.TrimSpace(req.ActorID) == "" {
		return errors.New("actor_id is required")
	}
	if strings.TrimSpace(req.Action.Type) == "" {
		return errors.New("action.type is required")
	}
	return nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
