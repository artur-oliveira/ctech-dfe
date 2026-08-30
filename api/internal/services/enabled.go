package services

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// enablementSource reports which document types a company has a fiscal
// configuration for.
//
// An interface so the counting rule is testable without five config tables, and
// so what "enabled" means lives in one place rather than being re-derived at
// each meter.
type enablementSource interface {
	ConfiguredDocTypes(ctx context.Context, orgPK string) ([]string, error)
}

// countEnabled counts the companies that can actually emit.
//
// **This is what the company quota applies to, and it is not what the quota
// counted before.** ownedOrganizations lists the companies an account OWNS, and
// counting those refuses the customer this model exists for — an accountant with
// forty CNPJs and one of them issuing — at forty while they use one.
// ctech-billing ADR 0021 says it plainly: two counters, never one, and the quota
// applies to the second.
//
// "Enabled" means a fiscal configuration exists. Those rows are written when
// somebody sets a série, which is exactly the moment a company becomes able to
// emit — and a company registered and never configured emits nothing and costs
// nothing. That is what lets one organization hold forty CNPJs and pay for one.
//
// One company with four document types is ONE company. Counting configurations
// would bill somebody twice for issuing two kinds of document, which is not what
// the plan sells.
//
// An unreadable enablement is an error, never a zero. Zero would let somebody
// past the quota during an outage, and this is the check that decides whether
// they may add another company.
func countEnabled(ctx context.Context, src enablementSource, companies []string) (int, error) {
	enabled := 0
	for _, orgPK := range companies {
		docTypes, err := src.ConfiguredDocTypes(ctx, orgPK)
		if err != nil {
			return 0, fmt.Errorf("reading the enablement of %s: %w", orgPK, err)
		}
		if len(docTypes) > 0 {
			enabled++
		}
	}
	return enabled, nil
}

// FiscalConfigEnablement reads the five fiscal config tables to answer which
// document types a company is configured for.
//
// One type over five repositories rather than a method on each: "is this company
// enabled" is one question, and answering it in five places is five places for
// one to be forgotten when a sixth document type arrives.
type FiscalConfigEnablement struct {
	repos map[string]fiscalConfigReader
}

// fiscalConfigReader is the one method this needs from a config repository.
type fiscalConfigReader interface {
	Get(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error)
}

// NewFiscalConfigEnablement wires the readers. A document type missing from this
// map is a document type that never counts toward the quota, which is why the
// map is built here and not discovered.
func NewFiscalConfigEnablement(
	nfe, nfce, cte, mdfe, nfse fiscalConfigReader,
) *FiscalConfigEnablement {
	return &FiscalConfigEnablement{repos: map[string]fiscalConfigReader{
		DocTypeNFe:  nfe,
		DocTypeNFCe: nfce,
		DocTypeCTe:  cte,
		DocTypeMDFe: mdfe,
		DocTypeNfse: nfse,
	}}
}

// ConfiguredDocTypes lists the document types this company has a fiscal
// configuration for.
//
// A read failure is an error rather than an omission: silently reporting fewer
// document types would undercount the quota, and undercounting a limit is
// letting somebody past it.
func (e *FiscalConfigEnablement) ConfiguredDocTypes(ctx context.Context, orgPK string) ([]string, error) {
	out := make([]string, 0, len(e.repos))
	for docType, repo := range e.repos {
		if repo == nil {
			continue
		}
		item, err := repo.Get(ctx, orgPK)
		if err != nil {
			return nil, fmt.Errorf("reading the %s config of %s: %w", docType, orgPK, err)
		}
		if item != nil {
			out = append(out, docType)
		}
	}
	sort.Strings(out)
	return out, nil
}
