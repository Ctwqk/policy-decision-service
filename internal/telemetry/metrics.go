package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	DecisionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pds_decisions_total",
		Help: "Total policy decisions by verdict, action type, and client.",
	}, []string{"verdict", "action_type", "client"})

	DecisionLatencySeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "pds_decision_latency_seconds",
		Help:    "Policy decision evaluation latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"action_type"})

	RuleEvaluationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pds_rule_evaluations_total",
		Help: "Total rule evaluations by rule id and match status.",
	}, []string{"rule_id", "matched"})

	RuleEvalErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pds_rule_eval_errors_total",
		Help: "Total rule evaluation errors by rule id.",
	}, []string{"rule_id"})

	CombinerDepErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pds_combiner_dependency_errors_total",
		Help: "Total combiner dependency errors by combiner rule id and dependency id.",
	}, []string{"rule_id", "dep_id"})

	FeatureLookupLatencySeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "pds_feature_lookup_latency_seconds",
		Help:    "Feature provider lookup latency in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	FeatureLookupDegradedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pds_feature_lookup_degraded_total",
		Help: "Total degraded feature provider lookups.",
	})

	KafkaSinkQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pds_kafka_sink_queue_depth",
		Help: "Current queued Kafka decision sink records.",
	})

	KafkaSinkDroppedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pds_kafka_sink_dropped_total",
		Help: "Total Kafka decision sink records dropped because the queue was full.",
	})

	AuditWriteErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pds_audit_write_errors_total",
		Help: "Total audit write errors.",
	})
)
