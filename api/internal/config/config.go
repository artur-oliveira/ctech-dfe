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

	// Billing (ctech-billing). All four are optional, and their absence is a
	// supported deployment rather than a misconfiguration: without them the
	// product runs in **no-charge mode**, where every account is unlimited. That
	// is what a dev environment needs, and it is announced loudly at boot so a
	// production instance missing them cannot be mistaken for a working one.
	//
	// The values are SSM SecureStrings under /ctech-dfe/{env}/billing/*, read by
	// the userdata in cdk/lib/api-stack.ts — see DEPLOYMENT.md § Out-of-band
	// parameters. BillingAPIURL is not a secret and comes from
	// /ctech-billing/{env}/internal-base-url, published by ctech-cdk.
	BillingAPIURL string `env:"BILLING_API_URL"`
	// BillingClientID is `dfe-billing`, the client-credentials client
	// ctech-account issued. It becomes the `azp` of every token minted for
	// billing, and billing resolves the tenant from exactly that claim — point it
	// at another client and every call is a 403, not a wrong tenant.
	BillingClientID     string `env:"BILLING_CLIENT_ID"`
	BillingClientSecret string `env:"BILLING_CLIENT_SECRET"`
	// AccountClientID/Secret are the client-credentials client ctech-account
	// issued for the reach check — whether a person may act for a company
	// (ctech-billing ADR 0023). Separate from the billing credentials because
	// they carry a different scope and one being wrong must not disable the
	// other.
	//
	// Absent means the reach check is OFF and the product's own membership row
	// is still the access record. That is the pre-flip state, and it is a
	// deliberate default: turning this on is a live authorization change.
	AccountClientID     string `env:"ACCOUNT_CLIENT_ID"`
	AccountClientSecret string `env:"ACCOUNT_CLIENT_SECRET"`

	// BillingWebhookSecret verifies billing's outbound deliveries. It is separate
	// from the client credentials because it authenticates the opposite
	// direction, and holding one says nothing about holding the other.
	BillingWebhookSecret string `env:"BILLING_WEBHOOK_SECRET"`

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
