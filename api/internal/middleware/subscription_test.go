package middleware

import (
	"testing"

	"github.com/gofiber/fiber/v3"
)

// The gate's whole value is that it is default-deny: a route added tomorrow is
// blocked without anybody remembering to block it. These tests pin the two
// halves of that — what it lets through, and that everything else it does not.

func TestReadsAreNeverBlocked(t *testing.T) {
	// The customer paid for the documents they already have, and fiscal custody
	// is a five-year legal obligation. Withholding somebody's own XML over an
	// unpaid invoice is not a lever this product pulls.
	for _, method := range []string{fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions} {
		if isMutating(method) {
			t.Errorf("%s must never be gated", method)
		}
	}
	for _, method := range []string{fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete} {
		if !isMutating(method) {
			t.Errorf("%s must be gated", method)
		}
	}
}

// TestExemptPathsAreTheOnesThatMustStayOpen walks the carve-outs of D2, each of
// which is an obligation rather than a new sale.
func TestExemptPathsAreTheOnesThatMustStayOpen(t *testing.T) {
	for _, tc := range []struct{ name, path string }{
		// The way out of the block. An account that cannot manage its
		// subscription because its subscription lapsed has no way to pay.
		{"choosing a plan", "/v1.0/billing/subscription"},
		{"changing plan", "/v1.0/billing/subscription/change"},
		{"accepting the terms", "/v1.0/auth/terms-addendum/accept"},
		{"accepting an invitation", "/v1.0/invitations/abc123/accept"},
		// Cancelling an NF-e has a 24-hour legal deadline. Blocking it over an
		// unpaid invoice turns an overdue bill into a fiscal problem the customer
		// cannot fix at any price.
		{"cancelling an issued document", "/v1.0/nfes/4126.../cancel"},
		{"carta de correção", "/v1.0/nfes/4126.../correction-letter"},
		{"closing an MDF-e in transit", "/v1.0/mdfes/5026.../close"},
		{"including a driver mid-trip", "/v1.0/mdfes/5026.../include-condutor"},
		{"including a document mid-trip", "/v1.0/mdfes/5026.../include-dfe"},
		{"an NFS-e event", "/v1.0/nfses/dps-1/events"},
		// Answering documents other people issued against your own CNPJ. Blocking
		// this punishes the customer for somebody else's actions.
		{"manifestação", "/v1.0/nfes/4126.../manifestation"},
		{"sincronizar distribuição", "/v1.0/distributions/nfe/sync"},
		{"importar XML de terceiro", "/v1.0/distributions/nfe/import-xml"},
		{"consultar chave", "/v1.0/distributions/nfe/key"},
		// Computes and returns; writes nothing, issues nothing.
		{"prévia de carga", "/v1.0/mdfes/cargo-preview"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !isExempt(tc.path) {
				t.Fatalf("%s must not be gated", tc.path)
			}
		})
	}
}

// TestIssuanceAndRegistryWritesAreGated is the other half, and the one that
// would rot silently: every path here must stay blocked, including `substitute`,
// which issues a **new** document and would otherwise be the way around the
// whole gate.
func TestIssuanceAndRegistryWritesAreGated(t *testing.T) {
	for _, tc := range []struct{ name, path string }{
		{"issuing an NF-e", "/v1.0/nfes"},
		{"issuing an NFC-e", "/v1.0/nfces"},
		{"issuing an MDF-e", "/v1.0/mdfes"},
		{"issuing an NFS-e", "/v1.0/nfses"},
		// The one that looks like an event and is not: it issues a replacement.
		{"substituting a document", "/v1.0/nfces/4126.../substitute"},
		{"creating a product", "/v1.0/products"},
		{"creating a person", "/v1.0/persons"},
		{"creating a vehicle", "/v1.0/vehicles"},
		{"uploading a certificate", "/v1.0/organizations/CNPJ_1/certificates"},
		{"inviting a member", "/v1.0/organizations/CNPJ_1/invitations"},
		{"editing the organization", "/v1.0/organizations/CNPJ_1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if isExempt(tc.path) {
				t.Fatalf("%s must be gated", tc.path)
			}
		})
	}
}

// TestExemptSuffixesDoNotMatchTooMuch: the suffix list is matched against whole
// paths, so a route that merely *ends* in one of these words is exempt. That is
// intended for document events, and this pins the shape so a future suffix like
// "/create" — which would exempt every creation route — fails here rather than
// in production.
func TestExemptSuffixesDoNotMatchTooMuch(t *testing.T) {
	for _, suffix := range exemptSuffixes {
		switch suffix {
		case "/cancel", "/correction-letter", "/manifestation", "/close",
			"/include-condutor", "/include-dfe", "/events", "/sync",
			"/import-xml", "/nfe/key", "/cargo-preview":
		default:
			t.Fatalf("new exempt suffix %q — is it a mutation on a document that "+
				"already exists, or does it let somebody issue one for free?", suffix)
		}
	}
}
