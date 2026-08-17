package consumer

import (
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

// The results queue carries three unrelated message shapes, and only one of them
// is a sale. These pin the boundary, because getting it wrong is not a bug that
// shows up as an error — it is a customer billed twice for one NF-e, or a plan
// that issues for free.

func TestOnlyAuthorisedDocumentsAreReported(t *testing.T) {
	for _, tc := range []struct {
		name   string
		event  map[string]any
		action billingAction
		meter  string
	}{
		{
			"an authorised NF-e is the sale",
			map[string]any{"result_kind": "document", "table_name": "nfes", "status": "authorized"},
			billingReport, services.MeterNFe,
		},
		{
			"an authorised NFC-e is its own meter",
			map[string]any{"result_kind": "document", "table_name": "nfces", "status": "authorized"},
			billingReport, services.MeterNFCe,
		},
		{
			"a rejection gives the slot back",
			map[string]any{"result_kind": "document", "table_name": "nfes", "status": "rejected"},
			billingRefund, services.MeterNFe,
		},
		{
			// The engine refused it before SEFAZ ever saw it — no document exists,
			// so the reservation has to come back just the same.
			"an engine failure gives the slot back",
			map[string]any{"result_kind": "document", "table_name": "nfses", "status": "failed"},
			billingRefund, services.MeterNFSe,
		},
		{
			// Still in flight: the next SQS delivery will finish it, and refunding
			// now would hand out a slot the document is about to use.
			"a retryable failure is not terminal",
			map[string]any{"result_kind": "document", "table_name": "nfes", "status": "retryable_failed"},
			billingNone, "",
		},
		{
			// The document was billed when it was issued. Billing the cancellation
			// too would charge twice for one emission.
			"a cancellation is not a new emission",
			map[string]any{"result_kind": "event", "table_name": "nfes", "status": "success"},
			billingNone, "",
		},
		{
			// Somebody else's NF-e arriving through distribuição. This account
			// never issued it.
			"a distribution notification is not an emission",
			map[string]any{"type": "new_distribution_nfe", "org_pk": "CNPJ_1"},
			billingNone, "",
		},
		{
			// A table nobody declared in MeterForTable bills nothing rather than
			// guessing a meter the plan may not grant.
			"an undeclared table bills nothing",
			map[string]any{"result_kind": "document", "table_name": "ctes_v2", "status": "authorized"},
			billingNone, "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			action, meter := billingActionFor(tc.event)
			if action != tc.action || meter != tc.meter {
				t.Fatalf("got (%d, %q), want (%d, %q)", action, meter, tc.action, tc.meter)
			}
		})
	}
}

// TestEveryDocumentMeterHasATable: a meter with no table is a document type that
// is quota-checked on the way in and never reported or refunded on the way out —
// the failure mode is silent, so it is checked here instead.
func TestEveryDocumentMeterHasATable(t *testing.T) {
	covered := map[string]bool{}
	for _, meter := range services.MeterForTable {
		covered[meter] = true
	}
	for _, meter := range services.DocumentMeters {
		if !covered[meter] {
			t.Errorf("meter %q is reserved on issuance but no table maps back to it", meter)
		}
	}
}
