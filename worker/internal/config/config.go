package config

import (
	"fmt"
	"os"
)

// Config holds all environment-driven settings for the worker Lambda.
type Config struct {
	AWSRegion            string
	TablePrefix          string
	DocumentsBucket      string
	CertsBucket          string
	DfeLambdaName        string
	ResultsTopicARN      string
	EventBusTopicARN     string // optional; distribution worker uses this for auto-Ciência
	DistributionQueueURL string // optional; dispatcher uses this to enqueue jobs
}

// Load reads config from environment variables (DFe worker + distribution worker).
func Load() (*Config, error) {
	cfg := &Config{
		AWSRegion:            getenv("AWS_REGION", "us-east-1"),
		TablePrefix:          getenv("TABLE_PREFIX", "dev"),
		DocumentsBucket:      os.Getenv("DOCUMENTS_BUCKET"),
		CertsBucket:          os.Getenv("CERTIFICATES_BUCKET"),
		DfeLambdaName:        os.Getenv("DFE_LAMBDA_NAME"),
		ResultsTopicARN:      os.Getenv("RESULTS_TOPIC_ARN"),
		EventBusTopicARN:     os.Getenv("EVENT_BUS_TOPIC_ARN"),
		DistributionQueueURL: os.Getenv("DISTRIBUTION_QUEUE_URL"),
	}
	if cfg.DocumentsBucket == "" {
		return nil, fmt.Errorf("DOCUMENTS_BUCKET is required")
	}
	if cfg.CertsBucket == "" {
		return nil, fmt.Errorf("CERTIFICATES_BUCKET is required")
	}
	if cfg.DfeLambdaName == "" {
		return nil, fmt.Errorf("DFE_LAMBDA_NAME is required")
	}
	return cfg, nil
}

// LoadDispatcher reads the minimal config needed by the distribution dispatcher.
func LoadDispatcher() (*Config, error) {
	cfg := &Config{
		AWSRegion:            getenv("AWS_REGION", "us-east-1"),
		TablePrefix:          getenv("TABLE_PREFIX", "dev"),
		DistributionQueueURL: os.Getenv("DISTRIBUTION_QUEUE_URL"),
	}
	if cfg.DistributionQueueURL == "" {
		return nil, fmt.Errorf("DISTRIBUTION_QUEUE_URL is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
