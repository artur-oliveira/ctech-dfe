package repositories

import (
	"context"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

type PersonRepository struct {
	CRUDRepository[map[string]any]
}

func NewPersonRepository(db *dynamodb.Client, cfg *config.Config) *PersonRepository {
	return &PersonRepository{
		CRUDRepository: NewCRUDRepository[map[string]any](db, cfg, "organization_persons"),
	}
}

func (r *PersonRepository) Create(ctx context.Context, orgPK, sk string, fields map[string]any) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Create(ctx, orgPK, sk, fields)
}

func (r *PersonRepository) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Get(ctx, orgPK, sk)
}

const (
	// PersonsNameIndex is the pre-existing GSI (pk, name), ProjectionType.ALL —
	// which is what makes contains(roles, :role) evaluable on it.
	PersonsNameIndex = "org-name-index"
	PersonsNameField = "name"
	// PersonRolesField is the list attribute filtered by Role. Mirrors
	// services.PersonRolesField; kept here so the repository has no service dep.
	PersonRolesField = "roles"
	// personDocPrefixCNPJ / personDocPrefixCPF are the two SK prefixes a
	// document search has to cover — the SK already *is* the document.
	personDocPrefixCNPJ = "CNPJ_"
	personDocPrefixCPF  = "CPF_"
)

var digitsOnlyRe = regexp.MustCompile(`^\d+$`)

type PersonListOpts struct {
	// NamePrefix is the legacy ?name= parameter — name-index prefix, untouched.
	NamePrefix string
	// Role filters by membership in the person's `roles` list. Empty leaves the
	// query byte-for-byte identical to what it was before roles existed.
	Role string
	// Q is the unified search term: digits search the document (the SK), any
	// other text searches the name prefix.
	Q        string
	Sort     string
	Limit    int
	StartKey map[string]types.AttributeValue
}

// List resolves one of three query shapes. The Role-less shape is deliberately
// identical to the pre-roles code so nothing existing can regress.
//
// A Role adds a FilterExpression, which DynamoDB applies *after* the key
// condition: the page may come back shorter than Limit alongside a
// LastEvaluatedKey. Filling the page is PersonService.List's job, not this one's.
func (r *PersonRepository) List(ctx context.Context, orgPK string, opts PersonListOpts) (*QueryResult, error) {
	forward := opts.Sort != "desc"
	q := QueryOpts{
		PK: orgPK, ScanIndexForward: forward,
		Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
	}
	if opts.Role != "" {
		q.FilterContainsField = PersonRolesField
		q.FilterContainsValue = opts.Role
	}

	if digitsOnlyRe.MatchString(opts.Q) {
		return r.queryByDocument(ctx, q, opts.Q, forward)
	}
	if term := opts.Q; term != "" || opts.NamePrefix != "" {
		if term == "" {
			term = opts.NamePrefix
		}
		q.IndexName = PersonsNameIndex
		q.SKField = PersonsNameField
		q.SKPrefix = term
	}
	return r.Query(ctx, q)
}

// queryByDocument searches the base table by document prefix. Because the SK is
// "CNPJ_…" or "CPF_…", one begins_with cannot span both, so the search runs as
// two ordered phases over the same partition (all CNPJ_ rows sort before all
// CPF_ rows). The returned cursor's own sk prefix says which phase to resume in,
// so no synthetic cursor state is needed.
func (r *PersonRepository) queryByDocument(ctx context.Context, base QueryOpts, digits string, forward bool) (*QueryResult, error) {
	prefixes := []string{personDocPrefixCNPJ + digits, personDocPrefixCPF + digits}
	if !forward {
		prefixes[0], prefixes[1] = prefixes[1], prefixes[0]
	}

	phase := 0
	if sk, ok := base.ExclusiveStartKey["sk"].(*types.AttributeValueMemberS); ok &&
		!strings.HasPrefix(sk.Value, prefixes[0]) {
		phase = 1
	}

	res := &QueryResult{}
	for i := phase; i < len(prefixes); i++ {
		opts := base
		opts.SKPrefix = prefixes[i]
		if i != phase {
			opts.ExclusiveStartKey = nil
		}
		if base.Limit > 0 {
			opts.Limit = base.Limit - len(res.Items)
			if opts.Limit <= 0 {
				// The page filled exactly on an earlier phase. Hand back a cursor
				// pointing at the last item read so the next call resumes here
				// instead of silently dropping the remaining phase.
				res.LastEvaluatedKey = itemKey(res.Items[len(res.Items)-1])
				break
			}
		}
		page, err := r.Query(ctx, opts)
		if err != nil {
			return nil, err
		}
		res.Items = append(res.Items, page.Items...)
		res.LastEvaluatedKey = page.LastEvaluatedKey
		if page.LastEvaluatedKey != nil {
			break // this phase still has rows; resume from its cursor
		}
	}
	return res, nil
}

// itemKey extracts the base-table primary key from a returned item.
func itemKey(item map[string]types.AttributeValue) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"pk": item["pk"], "sk": item["sk"]}
}

func (r *PersonRepository) Update(ctx context.Context, orgPK, sk string, updates map[string]any) (bool, error) {
	return r.CRUDRepository.Update(ctx, orgPK, sk, updates)
}

func (r *PersonRepository) Delete(ctx context.Context, orgPK, sk string) (bool, error) {
	return r.CRUDRepository.Delete(ctx, orgPK, sk)
}

func (r *PersonRepository) BuildCreateTxItem(orgPK, sk string, fields map[string]any) (types.TransactWriteItem, map[string]types.AttributeValue) {
	tx, item, _ := r.CRUDRepository.BuildCreateTxItemIfAbsent(orgPK, sk, fields)
	return tx, item
}

func (r *PersonRepository) BuildUpdateTxItem(orgPK, sk string, updates map[string]any) (types.TransactWriteItem, error) {
	return r.CRUDRepository.BuildUpdateTxItem(orgPK, sk, updates)
}

func (r *PersonRepository) BuildDeleteTxItem(orgPK, sk string) types.TransactWriteItem {
	return r.CRUDRepository.BuildDeleteTxItem(orgPK, sk)
}
