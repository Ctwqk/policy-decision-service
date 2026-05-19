package sink

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
)

type recordingPublisher struct {
	payloads [][]byte
	err      error
}

func (p *recordingPublisher) Publish(ctx context.Context, topic string, key []byte, value []byte) error {
	p.payloads = append(p.payloads, append([]byte(nil), value...))
	return p.err
}

func TestKafkaDecisionSinkSerializesDecisionEvent(t *testing.T) {
	publisher := &recordingPublisher{}
	sink := NewKafkaDecisionSink(KafkaDecisionSinkConfig{Topic: "pds.decisions.v1", QueueSize: 2, Publisher: publisher})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sink.Run(ctx)

	sink.Enqueue(ctx, engine.AuditRecord{DecisionID: "decision-1", ActorID: "actor-1", ActionType: "publish", Platform: "youtube", Verdict: engine.VerdictBlock, Score: 0.9, Reasons: []engine.Reason{{Code: "burst", Rule: "r1"}}, Client: "vp"})

	deadline := time.After(500 * time.Millisecond)
	for len(publisher.payloads) == 0 {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for publish")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	var event DecisionEvent
	if err := json.Unmarshal(publisher.payloads[0], &event); err != nil {
		t.Fatal(err)
	}
	if event.TopicVersion != "pds.decisions.v1" || event.ActorID != "actor-1" || event.Verdict != "block" {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestKafkaDecisionSinkDropsWhenQueueFull(t *testing.T) {
	sink := NewKafkaDecisionSink(KafkaDecisionSinkConfig{Topic: "pds.decisions.v1", QueueSize: 1, Publisher: &recordingPublisher{}})
	sink.Enqueue(context.Background(), engine.AuditRecord{DecisionID: "one"})
	sink.Enqueue(context.Background(), engine.AuditRecord{DecisionID: "two"})
	if sink.Dropped() != 1 {
		t.Fatalf("expected one dropped event, got %d", sink.Dropped())
	}
}
