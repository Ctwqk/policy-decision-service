package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr       string
	GRPCAddr       string
	MetricsAddr    string
	DatabaseURL    string
	RedisURL       string
	RulesPath      string
	BlocklistPath  string
	AuditQueueSize int
	AuditBatchSize int
	FailOpen       bool
}

func Load() Config {
	return Config{
		HTTPAddr:       envString("PDS_HTTP_ADDR", ":8080"),
		GRPCAddr:       envString("PDS_GRPC_ADDR", ":9090"),
		MetricsAddr:    envString("PDS_METRICS_ADDR", ":8081"),
		DatabaseURL:    envString("PDS_DATABASE_URL", "postgres://vp:vp_secret@localhost:5435/videoprocess?sslmode=disable"),
		RedisURL:       envString("PDS_REDIS_URL", "redis://localhost:6380/1"),
		RulesPath:      envString("PDS_RULES_PATH", "config/rules.example.yaml"),
		BlocklistPath:  envString("PDS_BLOCKLIST_PATH", "config/blocklist.example.txt"),
		AuditQueueSize: envInt("PDS_AUDIT_QUEUE_SIZE", 10000),
		AuditBatchSize: envInt("PDS_AUDIT_BATCH_SIZE", 100),
		FailOpen:       envBool("PDS_FAIL_OPEN", true),
	}
}

func envString(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
