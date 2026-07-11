package config

import (
	"fmt"
	"log/slog"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	// Server
	AppVersion string `env:"APP_VERSION" envDefault:"0.0.1"`

	Port int    `env:"PORT" envDefault:"8000"`
	Env  string `env:"ENVIRONMENT" envDefault:"dev"`

	ReadTimeout        int64    `env:"READ_TIMEOUT" envDefault:"10"`
	IdleTimeout        int64    `env:"IDLE_TIMEOUT" envDefault:"60"`
	WriteTimeout       int64    `env:"WRITE_TIMEOUT" envDefault:"10"`
	TrustedProxies     []string `env:"TRUSTED_PROXIES"`
	CorsAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS"`
	// AWS
	AWSRegion         string `env:"AWS_REGION" envDefault:"us-east-1"`
	TablePrefix       string `env:"TABLE_PREFIX,required"`
	S3BucketCerts     string `env:"S3_BUCKET_CERTIFICATES,required"`
	S3BucketDocuments string `env:"S3_BUCKET_DOCUMENTS,required"`
	SefazFunctionName string `env:"SEFAZ_FUNCTION_NAME,required"`
	DynamoDBEndpoint  string `env:"DYNAMODB_ENDPOINT"` // local override

	// SNS
	WorkerTopicARN string `env:"DFE_TOPIC_ARN"`

	// SQS
	ResultsQueueURL      string `env:"DFE_RESULTS_QUEUE_URL"`
	DistributionQueueURL string `env:"DFE_DISTRIBUTION_QUEUE_URL"`

	// Auth
	CtechURL        string `env:"CTECH_URL"`
	CtechJWKSURL    string `env:"CTECH_JWKS_URL"`
	ServiceAudience string `env:"SERVICE_AUDIENCE" envDefault:"https://dfe-api.aoctech.app"` // expected aud claim; empty = no audience check (transition only)

	// Cache / WebSocket
	RedisURL string `env:"VALKEY_URL"` // Redis/Valkey URL — optional; falls back to in-memory

	// Technical issuer data (used in some SEFAZ messages)
	TechnicalCNPJ  string `env:"TECHNICAL_CNPJ" envDefault:"62787449000107"`
	TechnicalName  string `env:"TECHNICAL_NAME" envDefault:"ARTUR OLIVEIRA CARVALHO"`
	TechnicalEmail string `env:"TECHNICAL_EMAIL" envDefault:"dev@aoctech.app"`
	TechnicalPhone string `env:"TECHNICAL_PHONE" envDefault:"86988033430"`
}

// Load reads config from environment variables.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if cfg.CtechJWKSURL == "" && cfg.CtechURL != "" {
		cfg.CtechJWKSURL = cfg.CtechURL + "/.well-known/jwks.json"
	}
	if cfg.ServiceAudience == "" && cfg.Env == "production" {
		slog.Warn("SERVICE_AUDIENCE is empty in production — the aud claim is not being checked; " +
			"any RS256 token signed by the identity provider will be accepted by this service")
	}
	if cfg.CtechURL == "" && cfg.Env == "production" {
		slog.Warn("CTECH_URL is empty in production — the iss claim is not being checked")
	}
	return cfg, nil
}
