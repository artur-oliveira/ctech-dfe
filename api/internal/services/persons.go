package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/artur-oliveira/ctech-dfe/api/internal/cache"
	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"
	"github.com/artur-oliveira/ctech-dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const personCacheTTL = 300

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

// StateRegistrationEntry is the plain-value shape of a state_registrations
// entry, shared by RequireOrgIE and any caller that already has decoded data.
type StateRegistrationEntry struct {
	UF                string
	StateRegistration string
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
}

func NewPersonService(repo *repositories.PersonRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *PersonService {
	return &PersonService{repo: repo, auditRepo: auditRepo, cache: c}
}

func personCacheKey(orgPK, sk string) string {
	return fmt.Sprintf("res:%s:persons:%s", orgPK, sk)
}

func personListCachePrefix(orgPK string) string {
	return fmt.Sprintf("res:%s:persons:", orgPK)
}

func (s *PersonService) Get(ctx context.Context, orgPK, cpfCNPJ string) (map[string]types.AttributeValue, error) {
	sk, err := BuildPersonSK(cpfCNPJ)
	if err != nil {
		return nil, err
	}
	key := personCacheKey(orgPK, sk)
	if v, ok := cacheGetItem(ctx, s.cache, key); ok {
		return v, nil
	}
	item, err := s.repo.Get(ctx, orgPK, sk)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("person not found")
	}
	cacheSetItem(ctx, s.cache, key, item, personCacheTTL)
	return item, nil
}

func (s *PersonService) List(ctx context.Context, orgPK string, opts repositories.PersonListOpts) (*repositories.QueryResult, error) {
	return s.repo.List(ctx, orgPK, opts)
}

// Create writes the person and its CREATE audit row atomically.
func (s *PersonService) Create(ctx context.Context, orgPK string, cpfCNPJ string, fields map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	sk, err := BuildPersonSK(cpfCNPJ)
	if err != nil {
		return nil, err
	}

	personTx, finalItem := s.repo.BuildCreateTxItem(orgPK, sk, fields)

	afterMap, err := attributeMapToPlain(finalItem)
	if err != nil {
		return nil, err
	}
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourcePerson, sk, repositories.AuditActionCreate,
		userID, userName, Diff(nil, afterMap),
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{personTx, auditTx}); err != nil {
		if repositories.IsConditionFailed(err) {
			return nil, problem.Conflict("pessoa com este CPF/CNPJ já cadastrada")
		}
		return nil, err
	}
	_ = s.cache.DeletePrefix(ctx, personListCachePrefix(orgPK))
	return finalItem, nil
}

// Update writes the person change and its UPDATE audit row atomically.
// Fetches the current item first so only actually-changed fields are logged.
func (s *PersonService) Update(ctx context.Context, orgPK, cpfCNPJ string, updates map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	sk, err := BuildPersonSK(cpfCNPJ)
	if err != nil {
		return nil, err
	}

	current, err := s.repo.Get(ctx, orgPK, sk)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, problem.NotFound("person not found")
	}
	beforeMap, err := attributeMapToPlain(current)
	if err != nil {
		return nil, err
	}

	personTx, err := s.repo.BuildUpdateTxItem(orgPK, sk, updates)
	if err != nil {
		return nil, err
	}
	// updates is a partial map (only the fields the caller wants to change).
	// Merge it over beforeMap so Diff only reports fields that actually
	// changed, instead of treating every omitted field as "changed to nil".
	afterMap := make(map[string]any, len(beforeMap))
	for k, v := range beforeMap {
		afterMap[k] = v
	}
	for k, v := range updates {
		afterMap[k] = v
	}
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourcePerson, sk, repositories.AuditActionUpdate,
		userID, userName, Diff(beforeMap, afterMap),
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{personTx, auditTx}); err != nil {
		return nil, err
	}
	_ = s.cache.Delete(ctx, personCacheKey(orgPK, sk))
	_ = s.cache.DeletePrefix(ctx, personListCachePrefix(orgPK))
	return s.repo.Get(ctx, orgPK, sk)
}

// Delete removes the person and writes its DELETE audit row atomically.
func (s *PersonService) Delete(ctx context.Context, orgPK, cpfCNPJ, userID, userName string) error {
	sk, err := BuildPersonSK(cpfCNPJ)
	if err != nil {
		return err
	}

	current, err := s.repo.Get(ctx, orgPK, sk)
	if err != nil {
		return err
	}
	if current == nil {
		return problem.NotFound("person not found")
	}
	beforeMap, err := attributeMapToPlain(current)
	if err != nil {
		return err
	}

	personTx := s.repo.BuildDeleteTxItem(orgPK, sk)
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourcePerson, sk, repositories.AuditActionDelete,
		userID, userName, Diff(beforeMap, nil),
	)
	if err != nil {
		return err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{personTx, auditTx}); err != nil {
		return err
	}
	_ = s.cache.Delete(ctx, personCacheKey(orgPK, sk))
	_ = s.cache.DeletePrefix(ctx, personListCachePrefix(orgPK))
	return nil
}
