package sink

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/Ctwqk/policy-decision-service/internal/engine"
	"github.com/Ctwqk/policy-decision-service/internal/telemetry"
	"github.com/google/uuid"
)

const (
	defaultDecisionTopic     = "pds.decisions.v1"
	defaultDecisionQueueSize = 10000
	publishTimeout           = 5 * time.Second
	drainTimeout             = 5 * time.Second
)

type Publisher interface {
	Publish(ctx context.Context, topic string, key []byte, value []byte) error
}

type DecisionEvent struct {
	EventID      string          `json:"event_id"`
	TopicVersion string          `json:"topic_version"`
	ActorID      string          `json:"actor_id"`
	ActionType   string          `json:"action_type"`
	Platform     string          `json:"platform,omitempty"`
	Verdict      string          `json:"verdict"`
	Score        float64         `json:"score"`
	Reasons      []engine.Reason `json:"reasons"`
	DecisionID   string          `json:"decision_id"`
	Client       string          `json:"client,omitempty"`
	OccurredAt   string          `json:"occurred_at"`
}

type KafkaDecisionSinkConfig struct {
	Topic     string
	QueueSize int
	Publisher Publisher
}

type KafkaDecisionSink struct {
	topic     string
	queue     chan engine.AuditRecord
	publisher Publisher
	dropped   atomic.Int64
}

func NewKafkaDecisionSink(cfg KafkaDecisionSinkConfig) *KafkaDecisionSink {
	topic := cfg.Topic
	if topic == "" {
		topic = defaultDecisionTopic
	}
	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = defaultDecisionQueueSize
	}
	s := &KafkaDecisionSink{
		topic:     topic,
		queue:     make(chan engine.AuditRecord, queueSize),
		publisher: cfg.Publisher,
	}
	s.updateQueueDepth()
	return s
}

func (s *KafkaDecisionSink) Enqueue(ctx context.Context, record engine.AuditRecord) {
	if s == nil || s.queue == nil {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	case s.queue <- record:
		s.updateQueueDepth()
	default:
		s.dropped.Add(1)
		telemetry.KafkaSinkDroppedTotal.Inc()
		s.updateQueueDepth()
	}
}

func (s *KafkaDecisionSink) Dropped() int64 {
	if s == nil {
		return 0
	}
	return s.dropped.Load()
}

func (s *KafkaDecisionSink) Run(ctx context.Context) {
	if s == nil || s.queue == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			s.drain()
			return
		case record := <-s.queue:
			s.updateQueueDepth()
			if ctx.Err() != nil {
				s.drain(record)
				return
			}
			s.publishWithTimeout(record)
			s.updateQueueDepth()
		}
	}
}

func (s *KafkaDecisionSink) publishWithTimeout(record engine.AuditRecord) {
	publishCtx, cancel := context.WithTimeout(context.Background(), publishTimeout)
	defer cancel()
	s.publishAndCount(publishCtx, record)
}

func (s *KafkaDecisionSink) drain(initial ...engine.AuditRecord) {
	queued := len(s.queue)
	drainCtx, cancel := context.WithTimeout(context.Background(), drainTimeout)
	defer cancel()
	for _, record := range initial {
		s.publishAndCount(drainCtx, record)
		s.updateQueueDepth()
	}
	for i := 0; i < queued; i++ {
		select {
		case record := <-s.queue:
			s.updateQueueDepth()
			s.publishAndCount(drainCtx, record)
			s.updateQueueDepth()
		case <-drainCtx.Done():
			s.updateQueueDepth()
			return
		default:
			s.updateQueueDepth()
			return
		}
	}
	s.updateQueueDepth()
}

func (s *KafkaDecisionSink) publishAndCount(ctx context.Context, record engine.AuditRecord) {
	_ = s.publish(ctx, record)
}

func (s *KafkaDecisionSink) publish(ctx context.Context, record engine.AuditRecord) error {
	if s == nil || s.publisher == nil {
		return nil
	}
	event := DecisionEvent{
		EventID:      uuid.NewString(),
		TopicVersion: s.topic,
		ActorID:      record.ActorID,
		ActionType:   record.ActionType,
		Platform:     record.Platform,
		Verdict:      string(record.Verdict),
		Score:        record.Score,
		Reasons:      record.Reasons,
		DecisionID:   record.DecisionID,
		Client:       record.Client,
		OccurredAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		telemetry.KafkaSinkPublishErrorsTotal.Inc()
		return err
	}
	if err := s.publisher.Publish(ctx, s.topic, []byte(record.ActorID), payload); err != nil {
		telemetry.KafkaSinkPublishErrorsTotal.Inc()
		return err
	}
	return nil
}

func (s *KafkaDecisionSink) updateQueueDepth() {
	if s == nil || s.queue == nil {
		return
	}
	telemetry.KafkaSinkQueueDepth.Set(float64(len(s.queue)))
}
