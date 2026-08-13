package config

import (
	"fmt"

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
	CtechIssuerURL  string `env:"CTECH_ISSUER_URL"`
	CtechJWKSURL    string `env:"CTECH_JWKS_URL"`
	ServiceAudience string `env:"SERVICE_AUDIENCE" envDefault:"https://dfe.aoctech.app"` // expected aud claim; empty = no audience check (transition only)

	// Cache / WebSocket
	RedisURL string `env:"VALKEY_URL"` // Redis/Valkey URL — optional; falls back to in-memory

	// Technical issuer data (used in some SEFAZ messages)
	TechnicalCNPJ  string `env:"TECHNICAL_CNPJ" envDefault:"62787449000107"`
	TechnicalName  string `env:"TECHNICAL_NAME" envDefault:"ARTUR OLIVEIRA CARVALHO"`
	TechnicalEmail string `env:"TECHNICAL_EMAIL" envDefault:"dev@aoctech.app"`
	TechnicalPhone string `env:"TECHNICAL_PHONE" envDefault:"86988033430"`
}

func (c *Config) prodValidation() error {
	if c.Env != "prod" {
		return nil
	}
	if c.ServiceAudience == "" {
		// Fail closed: without an audience check, any RS256 token the identity
		// provider signs for any client (id_tokens, api-key tokens, tokens minted
		// for other resource servers) would be accepted here. That is never a
		// safe prod posture, so refuse to start rather than warn.
		return fmt.Errorf("config: SERVICE_AUDIENCE must be set in prod so the aud claim is verified")
	}
	if c.WorkerTopicARN == "" {
		return fmt.Errorf("config: WORKER_TOPIC_ARN must be set in prod so the requests to sefaz will be sent")
	}
	if c.CtechIssuerURL == "" {
		return fmt.Errorf("config: CTECH_ISSUER_URL must be set in prod so the iss claim is verified")
	}
	if len(c.CorsAllowedOrigins) == 0 {
		return fmt.Errorf("config: CORS_ALLOWED_ORIGINS must be set in prod")
	}
	return nil
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
	if err := cfg.prodValidation(); err != nil {
		return nil, err
	}
	return cfg, nil
}
