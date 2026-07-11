package nfes

// danfe.go — DANFE PDF generation via external providers.
// Mirrors api/app/services/nfes/_danfe.py.

import (
	"time"
)

const danfeTimeout = 30 * time.Second

// GetDanfe fetches the NF-e XML from S3 then converts it to a PDF via an
// external DANFE provider. Uses meudanfe when an API key is configured;
// falls back to consultadanfe (no key required).
