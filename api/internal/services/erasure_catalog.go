package services

import (
	"slices"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// ErasureScope says how the account-deletion purge reaches a table's rows.
type ErasureScope int

const (
	ScopeKept           ErasureScope = iota // global (roles) or the tombstone table itself
	ScopePerson                             // keyed by the user's sub; person purge
	ScopeAccountBilling                     // account_billing: person (legacy USER_) and organization (ORG_) keys
	ScopeOrganization                       // the organizations item; deleted last, after the settle window
	ScopeOrgItem                            // pk = company id, no sort key (fiscal configs)
	ScopeOrgPartition                       // pk = company id, sk
	ScopeEnvDocs                            // pk = {env}#{company}; each row owns events and an outbox row
	ScopeEnvPartition                       // pk = {env}#{company} (distributions)
	ScopeDocEvents                          // pk = a document's sk; reached through ScopeEnvDocs
	ScopeOutbox                             // pk = {doc table}#{doc sk}; reached through ScopeEnvDocs
	ScopeInvitations                        // org_pk on org-invite-index
	ScopeSerieClaims                        // global; derived from the fiscal configs
)

// ErasureTable is one DynamoDB table and how the purge reaches it.
type ErasureTable struct {
	CDK     string // member of TableName in cdk/lib/dynamodb-stack.ts
	Name    string // physical name after "{prefix}_"
	Scope   ErasureScope
	SortKey string // "" (pk only), "sk" or "nsu"
	Events  string // ScopeEnvDocs: the events table keyed by the document's sk
	Inut    bool   // ScopeDocEvents: also holds INUT#{env}#{company} partitions
}

const skKey = "sk"

var erasureCatalog = []ErasureTable{
	{CDK: "roles", Name: "roles", Scope: ScopeKept},
	{CDK: "erasure_state", Name: "erasure_state", Scope: ScopeKept},
	// Replayable answers, keyed by the caller's sub; every row expires by TTL (24 h), so the purge leaves them.
	{CDK: "idempotency", Name: "idempotency", Scope: ScopeKept},
	{CDK: "users", Name: "users", Scope: ScopePerson},
	{CDK: "account_billing", Name: "account_billing", Scope: ScopeAccountBilling}, // person (legacy USER_ rows) and organization (ORG_ rows) keys, R15
	{CDK: "organizations", Name: "organizations", Scope: ScopeOrganization},
	{CDK: "organization_users", Name: "organization_users", Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "organization_invitations", Name: "organization_invitations", Scope: ScopeInvitations},
	{CDK: "serie_claims", Name: "serie_claims", Scope: ScopeSerieClaims},
	{CDK: "audit_logs", Name: "audit_logs", Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "products", Name: "organization_products", Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "vehicles", Name: "organization_vehicles", Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "persons", Name: "organization_persons", Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "certificates", Name: "organization_certificates", Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "services", Name: "organization_services", Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "tax_profiles", Name: repositories.TableTaxProfiles, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "operations", Name: repositories.TableOperations, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "payment_terms", Name: repositories.TablePaymentTerms, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "payment_terminals", Name: repositories.TablePaymentTerminals, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "toll_providers", Name: repositories.TableTollProviders, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "cargo_units", Name: repositories.TableCargoUnits, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "import_declarations", Name: repositories.TableImportDeclarations, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "insurance_policies", Name: repositories.TableInsurancePolicies, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "product_lots", Name: repositories.TableProductLots, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "fuel_pumps", Name: repositories.TableFuelPumps, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "vehicle_sets", Name: repositories.TableVehicleSets, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "service_locations", Name: repositories.TableServiceLocations, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "reference_documents", Name: repositories.TableReferenceDocuments, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "nfe_configs", Name: "organization_nfe_configs", Scope: ScopeOrgItem},
	{CDK: "nfce_configs", Name: "organization_nfce_configs", Scope: ScopeOrgItem},
	{CDK: "cte_configs", Name: "organization_cte_configs", Scope: ScopeOrgItem},
	{CDK: "mdfe_configs", Name: "organization_mdfe_configs", Scope: ScopeOrgItem},
	{CDK: "nfse_configs", Name: "organization_nfse_configs", Scope: ScopeOrgItem},
	{CDK: "nfes", Name: "nfes", Scope: ScopeEnvDocs, SortKey: skKey, Events: "nfe_events"},
	{CDK: "nfces", Name: "nfces", Scope: ScopeEnvDocs, SortKey: skKey, Events: "nfce_events"},
	{CDK: "ctes", Name: "ctes", Scope: ScopeEnvDocs, SortKey: skKey, Events: "cte_events"},
	{CDK: "mdfes", Name: "mdfes", Scope: ScopeEnvDocs, SortKey: skKey, Events: "mdfe_events"},
	{CDK: "nfses", Name: repositories.TableNfses, Scope: ScopeEnvDocs, SortKey: skKey, Events: "nfse_events"},
	{CDK: "nfe_events", Name: "nfe_events", Scope: ScopeDocEvents, SortKey: skKey, Inut: true},
	{CDK: "nfce_events", Name: "nfce_events", Scope: ScopeDocEvents, SortKey: skKey, Inut: true},
	{CDK: "cte_events", Name: "cte_events", Scope: ScopeDocEvents, SortKey: skKey},
	{CDK: "mdfe_events", Name: "mdfe_events", Scope: ScopeDocEvents, SortKey: skKey},
	{CDK: "nfse_events", Name: "nfse_events", Scope: ScopeDocEvents, SortKey: skKey},
	{CDK: "nfe_distributions", Name: "nfe_distributions", Scope: ScopeEnvPartition, SortKey: "nsu"},
	{CDK: "cte_distributions", Name: "cte_distributions", Scope: ScopeEnvPartition, SortKey: "nsu"},
	{CDK: "mdfe_distributions", Name: "mdfe_distributions", Scope: ScopeEnvPartition, SortKey: "nsu"},
	{CDK: "nfse_distributions", Name: "nfse_distributions", Scope: ScopeEnvPartition, SortKey: "nsu"},
	{CDK: "worker_outbox", Name: "worker_outbox", Scope: ScopeOutbox, SortKey: skKey},
}

// ErasureCatalog returns a copy of the purge catalog.
func ErasureCatalog() []ErasureTable { return slices.Clone(erasureCatalog) }
