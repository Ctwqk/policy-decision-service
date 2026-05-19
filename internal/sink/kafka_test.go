package sink

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
	"github.com/Ctwqk/policy-decision-service/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type recordingPublisher struct {
	mu                   sync.Mutex
	payloads             [][]byte
	err                  error
	published            chan struct{}
	publishStarted       chan struct{}
	continuePublish      chan struct{}
	failCanceledCtx      bool
	canceledCtxPublishes int
}

func (p *recordingPublisher) Publish(ctx context.Context, topic string, key []byte, value []byte) error {
	p.mu.Lock()
	p.payloads = append(p.payloads, append([]byte(nil), value...))
	published := p.published
	publishStarted := p.publishStarted
	continuePublish := p.continuePublish
	p.mu.Unlock()
	if publishStarted != nil {
		select {
		case publishStarted <- struct{}{}:
		default:
		}
	}
	if continuePublish != nil {
		<-continuePublish
	}
	p.mu.Lock()
	ctxErr := ctx.Err()
	if ctxErr != nil {
		p.canceledCtxPublishes++
	}
	p.mu.Unlock()
	if published != nil {
		select {
		case published <- struct{}{}:
		default:
		}
	}
	if p.failCanceledCtx && ctxErr != nil {
		return ctxErr
	}
	return p.err
}

func (p *recordingPublisher) waitForPayload(t *testing.T, timeout time.Duration) []byte {
	t.Helper()
	if p.published == nil {
		t.Fatalf("recordingPublisher.published channel is required for waitForPayload")
	}
	select {
	case <-p.published:
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for publish")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]byte(nil), p.payloads[len(p.payloads)-1]...)
}

func (p *recordingPublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.payloads)
}

func (p *recordingPublisher) canceledContextPublishes() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.canceledCtxPublishes
}

func TestKafkaDecisionSinkSerializesDecisionEvent(t *testing.T) {
	publisher := &recordingPublisher{published: make(chan struct{}, 1)}
	sink := NewKafkaDecisionSink(KafkaDecisionSinkConfig{Topic: "pds.decisions.v1", QueueSize: 2, Publisher: publisher})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sink.Run(ctx)

	sink.Enqueue(ctx, engine.AuditRecord{DecisionID: "decision-1", ActorID: "actor-1", ActionType: "publish", Platform: "youtube", Verdict: engine.VerdictBlock, Score: 0.9, Reasons: []engine.Reason{{Code: "burst", Rule: "r1"}}, Client: "vp"})

	var event DecisionEvent
	if err := json.Unmarshal(publisher.waitForPayload(t, 500*time.Millisecond), &event); err != nil {
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

func TestKafkaDecisionSinkDrainsQueuedRecordsOnCancel(t *testing.T) {
	for i := 0; i < 100; i++ {
		publisher := &recordingPublisher{failCanceledCtx: true}
		sink := NewKafkaDecisionSink(KafkaDecisionSinkConfig{Topic: "pds.decisions.v1", QueueSize: 2, Publisher: publisher})
		ctx, cancel := context.WithCancel(context.Background())
		sink.Enqueue(context.Background(), engine.AuditRecord{DecisionID: "one", ActorID: "actor-1"})
		sink.Enqueue(context.Background(), engine.AuditRecord{DecisionID: "two", ActorID: "actor-2"})

		cancel()
		sink.Run(ctx)

		if publisher.count() != 2 {
			t.Fatalf("expected queued records to drain on cancel, got %d", publisher.count())
		}
		if publisher.canceledContextPublishes() != 0 {
			t.Fatalf("expected drain not to publish with canceled service context, got %d canceled publishes", publisher.canceledContextPublishes())
		}
	}
}

func TestKafkaDecisionSinkPublishIsIndependentFromLifecycleCancel(t *testing.T) {
	publisher := &recordingPublisher{
		failCanceledCtx: true,
		publishStarted:  make(chan struct{}, 1),
		continuePublish: make(chan struct{}),
	}
	sink := NewKafkaDecisionSink(KafkaDecisionSinkConfig{Topic: "pds.decisions.v1", QueueSize: 1, Publisher: publisher})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		sink.Run(ctx)
	}()

	sink.Enqueue(context.Background(), engine.AuditRecord{DecisionID: "one", ActorID: "actor-1"})
	select {
	case <-publisher.publishStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for publish to start")
	}
	cancel()
	close(publisher.continuePublish)
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for sink to stop")
	}
	if publisher.canceledContextPublishes() != 0 {
		t.Fatalf("expected in-flight publish not to use canceled lifecycle context, got %d canceled publishes", publisher.canceledContextPublishes())
	}
}

func TestKafkaDecisionSinkRejectsEnqueueAfterRunStops(t *testing.T) {
	publisher := &recordingPublisher{}
	sink := NewKafkaDecisionSink(KafkaDecisionSinkConfig{Topic: "pds.decisions.v1", QueueSize: 1, Publisher: publisher})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sink.Run(ctx)

	sink.Enqueue(context.Background(), engine.AuditRecord{DecisionID: "late", ActorID: "actor-1"})

	if queued := len(sink.queue); queued != 0 {
		t.Fatalf("expected enqueue after shutdown to be rejected, queued=%d", queued)
	}
	if publisher.count() != 0 {
		t.Fatalf("expected no publish after shutdown, got %d", publisher.count())
	}
}

func TestKafkaDecisionSinkCountsPublishErrors(t *testing.T) {
	sink := NewKafkaDecisionSink(KafkaDecisionSinkConfig{
		Topic:     "pds.decisions.v1",
		QueueSize: 1,
		Publisher: &recordingPublisher{err: errors.New("publish failed")},
	})
	counter := telemetry.KafkaSinkPublishErrorsTotal
	before := testutil.ToFloat64(counter)

	if err := sink.publish(context.Background(), engine.AuditRecord{DecisionID: "one", ActorID: "actor-1"}); err == nil {
		t.Fatalf("expected publish error")
	}
	after := testutil.ToFloat64(counter)
	if after-before != 1 {
		t.Fatalf("expected publish error metric increment of 1, got before=%v after=%v", before, after)
	}
}
