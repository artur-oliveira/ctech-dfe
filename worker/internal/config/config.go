package config

import (
	"fmt"
	"os"
)

// Config holds all environment-driven settings for the worker Lambda.
type Config struct {
	AWSRegion                string
	TablePrefix              string
	DocumentsBucket          string
	CertsBucket              string
	DfeLambdaName            string
	DfeEgressRegion          string // region of the go-dfe-egress Lambda (sa-east-1)
	ResultsTopicARN          string
	EventBusTopicARN         string // optional; distribution worker uses this for auto-Ciência
	DistributionFunctionName string // dispatcher invokes this function (async) once per job
}

// Load reads config from environment variables (DFe worker + distribution worker).
func Load() (*Config, error) {
	cfg := &Config{
		AWSRegion:                getenv("AWS_REGION", "us-east-1"),
		TablePrefix:              getenv("TABLE_PREFIX", "dev"),
		DocumentsBucket:          os.Getenv("DOCUMENTS_BUCKET"),
		CertsBucket:              os.Getenv("CERTIFICATES_BUCKET"),
		DfeLambdaName:            os.Getenv("DFE_LAMBDA_NAME"),
		DfeEgressRegion:          os.Getenv("DFE_EGRESS_REGION"),
		ResultsTopicARN:          os.Getenv("RESULTS_TOPIC_ARN"),
		EventBusTopicARN:         os.Getenv("EVENT_BUS_TOPIC_ARN"),
		DistributionFunctionName: os.Getenv("DISTRIBUTION_FUNCTION_NAME"),
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
	if cfg.DfeEgressRegion == "" {
		return nil, fmt.Errorf("DFE_EGRESS_REGION is required")
	}
	return cfg, nil
}

// LoadDispatcher reads the minimal config needed by the distribution dispatcher.
func LoadDispatcher() (*Config, error) {
	cfg := &Config{
		AWSRegion:                getenv("AWS_REGION", "us-east-1"),
		TablePrefix:              getenv("TABLE_PREFIX", "dev"),
		DistributionFunctionName: os.Getenv("DISTRIBUTION_FUNCTION_NAME"),
	}
	if cfg.DistributionFunctionName == "" {
		return nil, fmt.Errorf("DISTRIBUTION_FUNCTION_NAME is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
