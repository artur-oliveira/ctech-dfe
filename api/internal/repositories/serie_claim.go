package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// ErrSerieTaken is the claim losing its conditional write: another company
// already emits under this tax id, on this modelo, série and ambiente.
var ErrSerieTaken = errors.New("série já em uso para este CNPJ")

// SerieClaimRepository enforces what ctech-billing ADR 0022 upgraded from an
// accepted limit into an enforced rule.
//
// An NF-e is unique by (CNPJ, modelo, série, número, ambiente). ctech-account
// lets two organizations hold the same CNPJ on purpose — a CNPJ is public data,
// and registering it is a claim rather than a capability — so without this the
// collision surfaces at the SEFAZ, as a duplicate rejection or as a gap in
// numbering somebody has to justify. The refusal belongs here, where issuance
// is, and at enablement rather than at emission: a refusal at emission means
// somebody already believed they could emit, and the gap may already exist.
type SerieClaimRepository struct {
	Base
}

func NewSerieClaimRepository(db *dynamodb.Client, cfg *config.Config) *SerieClaimRepository {
	return &SerieClaimRepository{Base: NewBase(db, cfg, serieClaimsTable)}
}

const serieClaimsTable = "serie_claims"

// attrClaimCompanyID names the holder. A constant because the conditional
// expressions on both Claim and Release read it, and a typo in either turns a
// guard into a no-op.
const attrClaimCompanyID = "company_id"

// SerieClaimPK keys a claim by exactly what the SEFAZ keys uniqueness by, minus
// the número — which is the sequence this claim protects.
//
// Global, not scoped to an organization. Scoping it would defeat the purpose:
// the whole point is that two *different* organizations must not both hold it.
//
// The tax id is canonical (mask stripped, letters uppercased): a CNPJ has been
// alphanumeric in its first twelve positions since 2026, so two spellings of
// one document must not claim two séries.
//
// '#' is unambiguous here only because no component can contain one — every
// component is alphanumeric by construction. It is worth knowing that the
// separator is otherwise ambiguous: (tax "1#5", modelo "5") and (tax "1",
// modelo "5#5") both build SERIE#1#5#5#1#1, which would let one company claim
// another's série. A guard would be dead code today, and
// TestClaimComponentsAreAlphanumericByConstruction is what fails if that stops
// being true.
func SerieClaimPK(taxID, modelo, ambiente string, serie int) string {
	return fmt.Sprintf("SERIE#%s#%s#%s#%d", taxID, modelo, ambiente, serie)
}

// Claim takes the série for this company, or reports that somebody else holds
// it.
//
// A conditional write, never a read-then-write: two enablements racing would
// both find the série free and both proceed, which is the exact outcome this
// exists to prevent.
//
// Re-claiming a série this company already holds succeeds. Enablement is
// idempotent, and a retry must not read as somebody else's collision.
func (r *SerieClaimRepository) Claim(ctx context.Context, taxID, modelo, ambiente string, serie int, companyID string) error {
	pk := SerieClaimPK(taxID, modelo, ambiente, serie)
	_, err := r.PutItemRaw(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.Base.TableName),
		Item: map[string]types.AttributeValue{
			"pk":               &types.AttributeValueMemberS{Value: pk},
			attrClaimCompanyID: &types.AttributeValueMemberS{Value: companyID},
			"created_at":       &types.AttributeValueMemberS{Value: NowStr()},
		},
		ConditionExpression: aws.String("attribute_not_exists(pk) OR #company_id = :self"),
		ExpressionAttributeNames: map[string]string{
			"#company_id": attrClaimCompanyID,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":self": &types.AttributeValueMemberS{Value: companyID},
		},
	})
	if IsConditionFailed(err) {
		return ErrSerieTaken
	}
	if err != nil {
		return fmt.Errorf("claiming série: %w", err)
	}
	return nil
}

// Release gives the série up, and only to the company that holds it: a
// conditional delete, so a stale request cannot free somebody else's claim.
//
// Releasing a claim that is not there succeeds. The caller is trying to reach a
// state where it does not exist, and it does not.
func (r *SerieClaimRepository) Release(ctx context.Context, taxID, modelo, ambiente string, serie int, companyID string) error {
	pk := SerieClaimPK(taxID, modelo, ambiente, serie)
	_, err := r.DeleteItemRaw(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.Base.TableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
		},
		ConditionExpression: aws.String("attribute_not_exists(pk) OR #company_id = :self"),
		ExpressionAttributeNames: map[string]string{
			"#company_id": attrClaimCompanyID,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":self": &types.AttributeValueMemberS{Value: companyID},
		},
	})
	if IsConditionFailed(err) {
		return ErrSerieTaken
	}
	if err != nil {
		return fmt.Errorf("releasing série: %w", err)
	}
	return nil
}

// Holder returns the company that holds this série, or "" when nobody does.
//
// It exists for diagnostics and for a caller that wants to check before it
// writes. It is NOT how Claim decides: reading first and writing second is the
// race this repository avoids.
func (r *SerieClaimRepository) Holder(ctx context.Context, taxID, modelo, ambiente string, serie int) (string, error) {
	item, err := r.GetItem(ctx, SerieClaimPK(taxID, modelo, ambiente, serie))
	if err != nil {
		return "", fmt.Errorf("reading série claim: %w", err)
	}
	if item == nil {
		return "", nil
	}
	return itemString(item, attrClaimCompanyID), nil
}
