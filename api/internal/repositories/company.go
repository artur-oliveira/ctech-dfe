package repositories

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Attribute names for the platform identity carried on a company record.
//
// Constants because the migration writes them and the read path reads them, and
// a typo in either is a silently empty name rather than a failure.
const (
	AttrOrganizationID   = "organization_id"
	AttrTaxID            = "tax_id"
	AttrTaxIDKind        = "tax_id_kind"
	AttrLegalName        = "legal_name"
	AttrIdentitySyncedAt = "identity_synced_at"
)

// Tax id kinds, matching what ctech-account stores.
//
// Deliberately different strings from the XSD element names (TagCNPJ/TagCPF in
// services): one is a stored enum shared with another system, the other is a
// fiscal schema's vocabulary, and collapsing them would couple a wire format to
// a database value.
const (
	TaxKindCNPJ = "cnpj"
	TaxKindCPF  = "cpf"
)

// CNPJLength and CNPJRootLength describe a CNPJ's shape. The raiz is its first
// eight positions, which identify the company before its branch order and check
// digits — so matriz and filial share it.
//
// Exported because the branch rule in the services package needs the same
// numbers, and two copies of "the raiz is eight long" is one copy too many.
const (
	CNPJLength     = 14
	CNPJRootLength = 8
)

// LocalCompany is the DF-e's projection of a platform company.
//
// The identity fields are a CACHE. ctech-account owns them (ctech-billing ADR
// 0022); this is what was read last, so that authorization and issuance are a
// GetItem rather than a call across the network. Nothing here may treat them as
// authoritative, and a rename in accounts is not an error on this side.
//
// The fiscal configuration lives on the same DynamoDB item and is NOT a cache —
// this repo owns it. That is why the identity is written with an UpdateItem on
// named attributes rather than a Put.
type LocalCompany struct {
	CompanyID      string
	OrganizationID string
	// TaxID is canonical: mask stripped, letters uppercased. A CNPJ has been
	// alphanumeric in its first twelve positions since the Receita Federal's
	// 2026 change, so this is not digits.
	TaxID            string
	TaxIDKind        string
	LegalName        string
	IdentitySyncedAt string
}

// Why there is no staleness check here.
//
// The identity is written by the re-key migration and by the handoff that links
// a company (ctech-account's organization-handoff spec). Nothing re-reads it,
// because nothing can: ctech-account's company routes sit behind
// RequireClientID(SelfClientID), so a dfe-issued token is refused, and there is
// no service credential for this direction. Inventing one is a cross-service
// auth decision, not a detail of this re-key.
//
// It costs little, which is the point. TaxID and TaxIDKind never change — a
// company whose tax id was wrong is a different company, register that one —
// and LegalName is display-only here; the xNome on a document comes from this
// repo's own `name` field, not from accounts. A refresher can be added when
// there is a rename problem to solve and a credential to solve it with.

// CNPJRoot returns the eight-position raiz, or "" when there is none.
//
// It reads the record, never the partition key — that is the whole point of the
// re-key. A CPF has no branch concept, and returning a prefix of one would make
// two unrelated people look like matriz and filial.
func (c *LocalCompany) CNPJRoot() string {
	if c == nil || c.TaxIDKind != TaxKindCNPJ || len(c.TaxID) < CNPJRootLength {
		return ""
	}
	return c.TaxID[:CNPJRootLength]
}

// GetCompany reads the local record. A legacy CNPJ_/CPF_ key still resolves, so
// this works either side of the flip — and during a rollback.
func (r *OrganizationRepository) GetCompany(ctx context.Context, orgPK string) (*LocalCompany, error) {
	pk, err := ParseOrgPK(orgPK)
	if err != nil {
		return nil, err
	}
	item, err := r.GetItem(ctx, pk)
	if err != nil || item == nil {
		return nil, err
	}
	return CompanyFromItem(pk, item), nil
}

// CompanyFromItem reads a company off an already-fetched item, so a caller that
// has one (from the organization cache, say) does not fetch it twice.
func CompanyFromItem(pk string, item map[string]types.AttributeValue) *LocalCompany {
	return &LocalCompany{
		CompanyID:        pk,
		OrganizationID:   itemString(item, AttrOrganizationID),
		TaxID:            itemString(item, AttrTaxID),
		TaxIDKind:        itemString(item, AttrTaxIDKind),
		LegalName:        itemString(item, AttrLegalName),
		IdentitySyncedAt: itemString(item, AttrIdentitySyncedAt),
	}
}

// PutIdentity refreshes the cached identity in place.
//
// An UpdateItem on named attributes, never a Put: the fiscal configuration
// lives on this same item, and a whole-item write would race a customer editing
// their série against a background identity refresh — and the refresh would
// win, silently.
func (r *OrganizationRepository) PutIdentity(ctx context.Context, c *LocalCompany) error {
	pk, err := ParseOrgPK(c.CompanyID)
	if err != nil {
		return err
	}
	_, err = r.UpdateItem(ctx, pk, nil, map[string]any{
		AttrOrganizationID:   c.OrganizationID,
		AttrTaxID:            c.TaxID,
		AttrTaxIDKind:        c.TaxIDKind,
		AttrLegalName:        c.LegalName,
		AttrIdentitySyncedAt: NowStr(),
	})
	return err
}

func itemString(item map[string]types.AttributeValue, key string) string {
	if av, ok := item[key].(*types.AttributeValueMemberS); ok {
		return av.Value
	}
	return ""
}
