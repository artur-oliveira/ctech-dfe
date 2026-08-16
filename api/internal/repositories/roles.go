package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// System role names. These are the values stored in a membership's `role`
// attribute and the identity (`ROLE_{NAME}`) of each row in the roles table.
const (
	RoleOwner  = "OWNER"
	RoleAdmin  = "ADMIN"
	RoleUser   = "USER"
	RoleViewer = "VIEWER"
)

// GrantableRoles are the roles member management may hand out. OWNER is absent,
// and its absence is the invariant: **an organization has exactly one OWNER, and
// it is whoever created it.**
//
// Ownership is therefore not a role somebody is given — it is a fact about how
// the organization came to exist, written once by OrganizationService in the
// same transaction as the organization row and never by any other path. Everyone
// else who needs full access gets ADMIN, which carries the same permission set
// (see SystemRoles) and none of the meaning.
//
// The meaning is what has to stay singular. Ownership is what will decide which
// account's subscription pays for the organization, and "the OWNER's plan" is
// only an answer while there is one OWNER. Two of them makes it a question with
// two answers, and billing would pick whichever row came back first.
//
// Transferring ownership is a real feature and deliberately does not exist yet.
// When it does, it moves the single OWNER — it does not add a second one, and it
// is not this list.
var GrantableRoles = []string{RoleAdmin, RoleUser, RoleViewer}

// IsGrantableRole reports whether member management may assign this role.
//
// One predicate rather than a check per surface: invitations, role changes and
// direct membership writes all ask the same question, and three copies of the
// answer is how one of them comes to allow a second OWNER.
func IsGrantableRole(role string) bool {
	for _, r := range GrantableRoles {
		if r == role {
			return true
		}
	}
	return false
}

const (
	actionList   = "list"
	actionGet    = "get"
	actionCreate = "create"
	actionUpdate = "update"
	actionDelete = "delete"
)

var actions = []string{actionList, actionGet, actionCreate, actionUpdate, actionDelete}
var resources = []string{
	"ctes", "cte_distributions", "cte_events",
	"mdfes", "mdfe_distributions", "mdfe_events",
	"nfces", "nfce_events",
	"nfes", "nfe_distributions", "nfe_events",
	"nfses", "nfse_distributions", "nfse_events",
	"organizations",
	"organization_products", "organization_vehicles", "organization_persons",
	"organization_services",
	"organization_tax_profiles", "organization_operations",
	"organization_payment_terms", "organization_vehicle_sets",
	"organization_nfe_configs", "organization_nfce_configs",
	"organization_cte_configs", "organization_mdfe_configs",
	"organization_nfse_configs",
	"organization_certificates",
}

// AllPermissions is the full permission set (action.resource pairs).
var AllPermissions []string

// ViewerPermissions is read-only: every list.* and get.* pair.
var ViewerPermissions []string

// UserPermissions is the operator set: full CRUD on day-to-day fiscal resources
// (documents, products, persons, vehicles) but no destructive or
// org-administration actions — no delete of anything, no update.organizations,
// and no access to certificates (which carry the private key material). Members
// who need those belong in ADMIN/OWNER.
var UserPermissions []string

func init() {
	for _, a := range actions {
		for _, r := range resources {
			perm := fmt.Sprintf("%s.%s", a, r)
			AllPermissions = append(AllPermissions, perm)
			if a == actionList || a == actionGet {
				ViewerPermissions = append(ViewerPermissions, perm)
			}
			if userPermitted(a, r) {
				UserPermissions = append(UserPermissions, perm)
			}
		}
	}
}

// userPermitted reports whether the USER role gets the given action.resource.
func userPermitted(action, resource string) bool {
	if action == actionDelete {
		return false
	}
	if resource == "organization_certificates" {
		return false
	}
	if resource == "organizations" && action == actionUpdate {
		return false
	}
	return true
}

// SystemRole is a seed definition for a built-in RBAC role.
type SystemRole struct {
	Name        string
	Description string
	Permissions []string
}

// SystemRoles returns the four built-in roles seeded at boot. OWNER and ADMIN
// get the full permission set (they also bypass permission-string checks in the
// RBAC middleware); USER and VIEWER carry explicit, narrower sets.
func SystemRoles() []SystemRole {
	return []SystemRole{
		{RoleOwner, "Proprietário — acesso total à organização", AllPermissions},
		{RoleAdmin, "Administrador — acesso total à organização", AllPermissions},
		{RoleUser, "Operador — emissão e cadastros, sem exclusões nem certificados", UserPermissions},
		{RoleViewer, "Visualizador — somente leitura", ViewerPermissions},
	}
}

type RoleRepository struct {
	Base
	// db is kept alongside Base for ListAll's Scan call, which Base does not
	// expose (Base's db field is unexported in the shared api-commons/dynamo
	// package).
	db *dynamodb.Client
}

func NewRoleRepository(db *dynamodb.Client, cfg *config.Config) *RoleRepository {
	return &RoleRepository{Base: NewBase(db, cfg, "roles"), db: db}
}

func (r *RoleRepository) Get(ctx context.Context, name string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, fmt.Sprintf("ROLE_%s", strings.ToUpper(name)))
}

// ListAll scans the roles table.
// Justified: table contains at most 4 rows (fixed system roles); no access pattern warrants a GSI.
func (r *RoleRepository) ListAll(ctx context.Context) ([]map[string]types.AttributeValue, error) {
	out, err := r.db.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(r.TableName),
	})
	if err != nil {
		return nil, wrapDynamoErr(err)
	}
	return out.Items, nil
}

func (r *RoleRepository) Upsert(ctx context.Context, name, description string, permissions []string) (map[string]types.AttributeValue, error) {
	now := NowStr()
	pk := fmt.Sprintf("ROLE_%s", strings.ToUpper(name))

	existing, _ := r.GetItem(ctx, pk)

	permAV, _ := attributevalue.MarshalList(permissions)
	item := map[string]types.AttributeValue{
		"pk":          &types.AttributeValueMemberS{Value: pk},
		"name":        &types.AttributeValueMemberS{Value: strings.ToUpper(name)},
		"description": &types.AttributeValueMemberS{Value: description},
		"permissions": &types.AttributeValueMemberL{Value: permAV},
		"updated_at":  &types.AttributeValueMemberS{Value: now},
	}
	if existing == nil {
		item["created_at"] = &types.AttributeValueMemberS{Value: now}
	} else if ca, ok := existing["created_at"]; ok {
		item["created_at"] = ca
	}

	return item, r.PutItem(ctx, item)
}
