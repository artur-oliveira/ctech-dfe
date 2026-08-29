package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// Organization is the DynamoDB item for an organization.
// PK format: CNPJ_{cnpj} or CPF_{cpf} — matches Python OrganizationRepository.parse_pk.
type Organization struct {
	PK        string `dynamodbav:"pk"`
	Name      string `dynamodbav:"name"`
	CreatedAt string `dynamodbav:"created_at"`
	UpdatedAt string `dynamodbav:"updated_at"`
}

// AttrOwnerUserID names the attribute holding the bare ctech-account subject of
// the account that owns the organization — the one whose subscription pays for
// it.
//
// A constant because two layers read it (the billing service, to find whose plan
// governs an org; the organization service, to write it) and a third will when
// ownership transfer arrives. It mirrors the single OWNER membership rather than
// replacing it: the membership is what grants access, this is what gets billed,
// and they are written in the same transaction so they cannot disagree.
const AttrOwnerUserID = "owner_user_id"

// OrganizationRepository manages organization persistence.
type OrganizationRepository struct {
	Base
}

func NewOrganizationRepository(db *dynamodb.Client, cfg *config.Config) *OrganizationRepository {
	return &OrganizationRepository{Base: NewBase(db, cfg, "organizations")}
}

// companyKeyLength is a UUID in its canonical 8-4-4-4-12 form.
const companyKeyLength = 36

// IsCompanyKey reports whether pk is a platform company id rather than a legacy
// CNPJ_/CPF_ key.
//
// One predicate, because "has this row been re-keyed" is asked from several
// places and two spellings of the question drift apart.
func IsCompanyKey(pk string) bool {
	// The shape is checked rather than parsed: this runs on every request, and
	// the only question is which key era the value belongs to. Lowercase hex
	// only — uuid.String() emits lowercase, and accepting both cases would let
	// one company hold two partitions.
	if len(pk) != companyKeyLength {
		return false
	}
	for i := 0; i < len(pk); i++ {
		c := pk[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return false
			}
		}
	}
	return true
}

// ParseOrgPK canonicalizes a partition key.
//
// Three accepted shapes, and the two legacy ones are not deprecation debt: the
// old partitions are the re-key migration's rollback, and a build that cannot
// read them cannot roll back.
//
//   - a platform company id (UUIDv7) — the key from ctech-billing ADR 0022 on
//   - an already-prefixed CNPJ_/CPF_ key — legacy, still readable
//   - a bare or masked CPF/CNPJ — normalized to the legacy prefix form
//
// Note what is deliberately absent: nothing reads a tax id back out of the
// result. A CNPJ has been alphanumeric in its first twelve positions since the
// Receita Federal's 2026 change, and the canonical tax id lives on the company
// record, not in the key.
func ParseOrgPK(cpfOrCNPJ string) (string, error) {
	if IsCompanyKey(cpfOrCNPJ) {
		return cpfOrCNPJ, nil
	}
	if strings.HasPrefix(cpfOrCNPJ, "CNPJ_") || strings.HasPrefix(cpfOrCNPJ, "CPF_") {
		return cpfOrCNPJ, nil
	}
	// Only a typed document reaches the legacy path, and a typed document holds
	// digits and mask punctuation and nothing else. Without this guard a
	// truncated company id falls through: "0199f3a1-8c42-7c31-9d5e" happens to
	// carry exactly fourteen digits, and stripping the rest turned it into
	// CNPJ_01993184273195 — a partition nobody meant to address.
	//
	// Refusing letters here is right even though a CNPJ is alphanumeric now:
	// this path only ever produced CNPJ_{digits} keys, so an alphanumeric CNPJ
	// has no legacy partition to name.
	if !isTypedDocument(cpfOrCNPJ) {
		return "", problem.BadRequest("organização inválida")
	}
	digits := stripNonDigits(cpfOrCNPJ)
	switch len(digits) {
	case 11:
		return fmt.Sprintf("CPF_%s", digits), nil
	case 14:
		return fmt.Sprintf("CNPJ_%s", digits), nil
	default:
		// No longer "deve começar com CNPJ_ ou CPF_": that is user-facing and
		// describes a shape the product no longer issues.
		return "", problem.BadRequest("organização inválida")
	}
}

// isTypedDocument reports whether raw is a CPF/CNPJ as somebody would type it:
// digits, and the punctuation a mask is made of.
func isTypedDocument(raw string) bool {
	if raw == "" {
		return false
	}
	for _, r := range raw {
		switch {
		case r >= '0' && r <= '9', r == '.', r == '/', r == '-', r == ' ':
		default:
			return false
		}
	}
	return true
}

// GetOrganization fetches an organization by its PK (prefixed or raw CNPJ/CPF).
func (r *OrganizationRepository) GetOrganization(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error) {
	pk, err := ParseOrgPK(orgPK)
	if err != nil {
		return nil, err
	}
	return r.GetItem(ctx, pk)
}

// CreateOrganization writes a new organization item.
func (r *OrganizationRepository) CreateOrganization(ctx context.Context, cpfOrCNPJ string, item map[string]types.AttributeValue) error {
	pk, err := ParseOrgPK(cpfOrCNPJ)
	if err != nil {
		return err
	}
	now := NowStr()
	item["pk"] = &types.AttributeValueMemberS{Value: pk}
	item["created_at"] = &types.AttributeValueMemberS{Value: now}
	item["updated_at"] = &types.AttributeValueMemberS{Value: now}
	return r.PutItem(ctx, item)
}

// BuildCreateTxItem returns a create-only TransactWriteItem (fails if the org
// already exists) for the organization, mirroring CreateOrganization's
// key/timestamp logic without writing. Also returns the finalized item.
func (r *OrganizationRepository) BuildCreateTxItem(cpfOrCNPJ string, item map[string]types.AttributeValue) (types.TransactWriteItem, map[string]types.AttributeValue, error) {
	pk, err := ParseOrgPK(cpfOrCNPJ)
	if err != nil {
		return types.TransactWriteItem{}, nil, err
	}
	if item == nil {
		item = map[string]types.AttributeValue{}
	}
	now := NowStr()
	item["pk"] = &types.AttributeValueMemberS{Value: pk}
	item["created_at"] = &types.AttributeValueMemberS{Value: now}
	item["updated_at"] = &types.AttributeValueMemberS{Value: now}
	return r.BuildPutTxItemIfAbsent(item), item, nil
}

// UpdateOrganization applies a partial update.
func (r *OrganizationRepository) UpdateOrganization(ctx context.Context, orgPK string, updates map[string]any) error {
	pk, err := ParseOrgPK(orgPK)
	if err != nil {
		return err
	}
	updates["updated_at"] = NowStr()
	_, err = r.UpdateItem(ctx, pk, nil, updates)
	return err
}

// BuildUpdateTxItem returns a TransactWriteItem for updating an existing
// organization, mirroring UpdateOrganization's key/timestamp logic, without writing.
func (r *OrganizationRepository) BuildUpdateTxItem(orgPK string, updates map[string]any) (types.TransactWriteItem, error) {
	pk, err := ParseOrgPK(orgPK)
	if err != nil {
		return types.TransactWriteItem{}, err
	}
	updates["updated_at"] = NowStr()
	return r.Base.BuildUpdateTxItem(pk, nil, updates)
}

func stripNonDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
