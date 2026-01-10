package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr     string
	MySQLDSN     string
	RabbitMQURL  string
	RabbitExchange     string
	RabbitQueue        string
	RabbitRoutingKey   string
	RabbitConsumerTag  string
	RabbitPublishPrefix string
	SSEHeartbeat time.Duration
	HistoryLimit int
}

func New() *Config {
	_ = godotenv.Load()

	cfg := &Config{
		HTTPAddr:     ":8080",
		SSEHeartbeat: 15 * time.Second,
		HistoryLimit: 20,
		RabbitExchange:     "notifications",
		RabbitQueue:        "notifications.sse",
		RabbitRoutingKey:   "notification.*",
		RabbitConsumerTag:  "sse-consumer",
		RabbitPublishPrefix: "notification",
	}

	if addr := os.Getenv("HTTP_ADDR"); addr != "" {
		cfg.HTTPAddr = addr
	} else if port := os.Getenv("PORT"); port != "" {
		cfg.HTTPAddr = ":" + port
	}

	cfg.MySQLDSN = os.Getenv("MYSQL_DSN")
	cfg.RabbitMQURL = os.Getenv("RABBITMQ_URL")

	if v := os.Getenv("RABBITMQ_EXCHANGE"); v != "" {
		cfg.RabbitExchange = v
	}
	if v := os.Getenv("RABBITMQ_QUEUE"); v != "" {
		cfg.RabbitQueue = v
	}
	if v := os.Getenv("RABBITMQ_ROUTING_KEY"); v != "" {
		cfg.RabbitRoutingKey = v
	}
	if v := os.Getenv("RABBITMQ_CONSUMER_TAG"); v != "" {
		cfg.RabbitConsumerTag = v
	}
	if v := os.Getenv("RABBITMQ_PUBLISH_PREFIX"); v != "" {
		cfg.RabbitPublishPrefix = v
	}

	if v := os.Getenv("SSE_HEARTBEAT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.SSEHeartbeat = time.Duration(n) * time.Second
		}
	}

	if v := os.Getenv("HISTORY_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.HistoryLimit = n
		}
	}

	return cfg
}
