package services

import (
	"context"
	"regexp"
	"strings"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	cnpjRe = regexp.MustCompile(`^\d{14}$`)
	cpfRe  = regexp.MustCompile(`^\d{11}$`)
)

// BuildPersonSK returns "CNPJ_14digits" or "CPF_11digits". Strips punctuation first.
func BuildPersonSK(cpfCNPJ string) (string, error) {
	v := strings.NewReplacer(".", "", "-", "", "/", "").Replace(cpfCNPJ)
	if cnpjRe.MatchString(v) {
		return "CNPJ_" + v, nil
	}
	if cpfRe.MatchString(v) {
		return "CPF_" + v, nil
	}
	return "", problem.BadRequest("invalid CPF/CNPJ: " + cpfCNPJ)
}

// RequirePJFields returns a BadRequest problem if cpfOrCNPJ is a CNPJ (14
// digits) and crt is nil. CPF documents (pessoa física) never require CRT.
// Applies to both persons and organizations — every pessoa jurídica has a
// fixed tax regime regardless of which fiscal documents it appears on.
func RequirePJFields(cpfOrCNPJ string, crt *int) error {
	v := strings.NewReplacer(".", "", "-", "", "/", "").Replace(cpfOrCNPJ)
	if cnpjRe.MatchString(v) && crt == nil {
		return problem.BadRequest("crt é obrigatório para pessoa jurídica")
	}
	return nil
}

// PersonService mirrors api/app/services/persons.py.
type PersonService struct {
	repo      *repositories.PersonRepository
	auditRepo *repositories.AuditLogRepository
	cache     cache.Backend
	crud      *CRUDMutationHelper
}

func NewPersonService(repo *repositories.PersonRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *PersonService {
	return &PersonService{
		repo:      repo,
		auditRepo: auditRepo,
		cache:     c,
		crud:      NewCRUDMutationHelper(auditRepo, c),
	}
}

func (s *PersonService) Get(ctx context.Context, orgPK, cpfCNPJ string) (map[string]types.AttributeValue, error) {
	sk, err := BuildPersonSK(cpfCNPJ)
	if err != nil {
		return nil, err
	}
	key := BuildItemCacheKey(orgPK, "persons", sk)
	return GetCachedItem(ctx, s.cache, key, func(ctx context.Context) (map[string]types.AttributeValue, error) {
		return s.repo.Get(ctx, orgPK, sk)
	}, "person not found")
}

// MaxFilteredPageRoundTrips bounds how many DynamoDB calls one filtered listing
// request may make while trying to fill a page. A FilterExpression is applied
// after the read, so a rare role in a large organization would otherwise sweep
// the whole partition just to hand back an empty page.
//
// Hitting the cap is a latency degradation, never a correctness bug, because
// end-of-list is signalled by an absent LastEvaluatedKey — never by an item
// count below Limit. Callers (route and UI alike) must follow that same rule.
const MaxFilteredPageRoundTrips = 5

func (s *PersonService) List(ctx context.Context, orgPK string, opts repositories.PersonListOpts) (*repositories.QueryResult, error) {
	return GetCachedList(ctx, s.cache, orgPK, "persons", opts, func(ctx context.Context) (*repositories.QueryResult, error) {
		if opts.Role == "" {
			return s.repo.List(ctx, orgPK, opts)
		}
		return s.listFilled(ctx, orgPK, opts)
	})
}

// listFilled pages through the filtered query until the requested page is full,
// the cursor is exhausted, or MaxFilteredPageRoundTrips is spent.
func (s *PersonService) listFilled(ctx context.Context, orgPK string, opts repositories.PersonListOpts) (*repositories.QueryResult, error) {
	want := opts.Limit
	acc := &repositories.QueryResult{}
	page := opts

	for i := 0; i < MaxFilteredPageRoundTrips; i++ {
		res, err := s.repo.List(ctx, orgPK, page)
		if err != nil {
			return nil, err
		}
		acc.Items = append(acc.Items, res.Items...)
		acc.LastEvaluatedKey = res.LastEvaluatedKey

		if res.LastEvaluatedKey == nil {
			break // end of list — the only end-of-list signal there is
		}
		if want > 0 && len(acc.Items) >= want {
			break
		}
		page.StartKey = res.LastEvaluatedKey
		if want > 0 {
			// Shrink the ask so a later round trip cannot overshoot the page and
			// force a truncation the cursor could no longer describe.
			page.Limit = want - len(acc.Items)
		}
	}
	return acc, nil
}

// Create writes the person and its CREATE audit row atomically.
func (s *PersonService) Create(ctx context.Context, orgPK string, cpfCNPJ string, fields map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	sk, err := BuildPersonSK(cpfCNPJ)
	if err != nil {
		return nil, err
	}

	tw := func(ctx context.Context, items []types.TransactWriteItem) error {
		if err := s.repo.TransactWrite(ctx, items); err != nil {
			if repositories.IsConditionFailed(err) {
				return problem.Conflict("pessoa com este CPF/CNPJ já cadastrada")
			}
			return err
		}
		return nil
	}

	return s.crud.Create(ctx, orgPK, repositories.AuditResourcePerson, userID, userName, func() (types.TransactWriteItem, map[string]types.AttributeValue, error) {
		tx, item := s.repo.BuildCreateTxItem(orgPK, sk, fields)
		return tx, item, nil
	}, tw)
}

// Update writes the person change and its UPDATE audit row atomically.
func (s *PersonService) Update(ctx context.Context, orgPK, cpfCNPJ string, updates map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	sk, err := BuildPersonSK(cpfCNPJ)
	if err != nil {
		return nil, err
	}

	return s.crud.Update(ctx, orgPK, sk, repositories.AuditResourcePerson, updates, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildUpdateTxItem(orgPK, sk, updates)
	}, s.repo.TransactWrite)
}

// Delete removes the person and writes its DELETE audit row atomically.
func (s *PersonService) Delete(ctx context.Context, orgPK, cpfCNPJ, userID, userName string) error {
	sk, err := BuildPersonSK(cpfCNPJ)
	if err != nil {
		return err
	}

	return s.crud.Delete(ctx, orgPK, sk, repositories.AuditResourcePerson, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildDeleteTxItem(orgPK, sk), nil
	}, s.repo.TransactWrite)
}
