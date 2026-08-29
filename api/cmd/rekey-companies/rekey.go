package main

import (
	"fmt"
	"sort"
	"strings"
)

// sourceSystem is the namespace ctech-account's companies were imported under.
// The mapping from a legacy key to a company id is read from there, never held
// in a file: a file is a thing that has to survive, and this does not.
const sourceSystem = "dfe"

// tableKind says how a table's rows are addressed, which is all the copier
// needs to know about it.
type tableKind int

const (
	// kindOrgPK: pk is the organization key itself.
	kindOrgPK tableKind = iota
	// kindEnvOrgPK: pk is "{env}#{org_pk}" — the document tables, which carry
	// the SEFAZ environment in front of the tenant.
	kindEnvOrgPK
)

// table names one table to re-key.
//
// Ordering matters and is not alphabetical: configuration first, documents
// last. A run that dies halfway then leaves companies that cannot emit — which
// is bad and visible — rather than companies that emit with the wrong
// configuration, which is worse and silent.
type table struct {
	Name string
	Kind tableKind
}

// tables is every table partitioned by the organization key, in copy order.
//
// The list is explicit rather than derived from a prefix scan: a table that
// appears in production and not here must fail the verification loudly, and a
// derived list would quietly include whatever it found.
var tables = []table{
	// The company record itself, so a partial run leaves something to look at.
	{"organizations", kindOrgPK},

	// Fiscal configuration. Singletons, one row per company.
	{"organization_nfe_configs", kindOrgPK},
	{"organization_nfce_configs", kindOrgPK},
	{"organization_cte_configs", kindOrgPK},
	{"organization_mdfe_configs", kindOrgPK},
	{"organization_nfse_configs", kindOrgPK},

	// The certificate rows. The PFX objects in S3 do NOT move: they resolve
	// through the s3_key stored on these rows, which is copied verbatim.
	{"organization_certificates", kindOrgPK},

	// Reusable registries.
	{"organization_products", kindOrgPK},
	{"organization_services", kindOrgPK},
	{"organization_persons", kindOrgPK},
	{"organization_vehicles", kindOrgPK},
	{"organization_tax_profiles", kindOrgPK},
	{"organization_operations", kindOrgPK},
	{"organization_payment_terms", kindOrgPK},
	{"organization_payment_terminals", kindOrgPK},
	{"organization_toll_providers", kindOrgPK},
	{"organization_vehicle_sets", kindOrgPK},
	{"organization_cargo_units", kindOrgPK},
	{"organization_import_declarations", kindOrgPK},
	{"organization_insurance_policies", kindOrgPK},
	{"organization_product_lots", kindOrgPK},
	{"organization_fuel_pumps", kindOrgPK},

	// Documents and what hangs off them. The largest by far, and the only ones
	// where the copy is not instant.
	{"nfes", kindEnvOrgPK},
	{"nfces", kindEnvOrgPK},
	{"ctes", kindEnvOrgPK},
	{"mdfes", kindEnvOrgPK},
	{"nfses", kindEnvOrgPK},
	{"nfe_events", kindEnvOrgPK},
	{"nfce_events", kindEnvOrgPK},
	{"cte_events", kindEnvOrgPK},
	{"mdfe_events", kindEnvOrgPK},
	{"nfse_events", kindEnvOrgPK},
	{"nfe_distributions", kindEnvOrgPK},
	{"cte_distributions", kindEnvOrgPK},
	{"mdfe_distributions", kindEnvOrgPK},
	{"nfse_distributions", kindEnvOrgPK},
}

// environments are the SEFAZ environment prefixes the document tables carry.
// Both are copied: a company's homologação history is its own, and leaving it
// behind would make the test environment look empty after the flip.
var environments = []string{"prod", "hom"}

// partitionsFor lists the partition keys a table holds for one organization.
//
// A document table holds one per environment; everything else holds one.
func partitionsFor(t table, orgKey string) []string {
	if t.Kind == kindEnvOrgPK {
		out := make([]string, 0, len(environments))
		for _, env := range environments {
			out = append(out, env+"#"+orgKey)
		}
		return out
	}
	return []string{orgKey}
}

// mapping is one organization's move.
type mapping struct {
	LegacyPK  string
	CompanyID string
	// OrganizationID is the workspace the company belongs to, written onto the
	// company record so authorization can read both ids in one GetItem.
	OrganizationID string
	TaxID          string
	TaxIDKind      string
	LegalName      string
}

// decision is what the planner concluded about one organization.
type decision struct {
	LegacyPK string
	Mapping  *mapping
	// Review is set when a human has to look. A run with any of these exits
	// non-zero and writes nothing for that organization.
	Review string
}

// NeedsHuman reports whether this organization was left alone.
func (d decision) NeedsHuman() bool { return d.Review != "" }

// plan turns the legacy organizations and the company mapping into decisions.
//
// An organization with no company in ctech-account is NOT migrated silently: it
// is reported and the run exits non-zero. A partition nobody can reach is worse
// than a migration that stopped, and "it was not in the mapping" is a fact
// somebody has to explain before the flip, not after.
func plan(legacyPKs []string, companies map[string]*mapping) []decision {
	out := make([]decision, 0, len(legacyPKs))
	for _, pk := range legacyPKs {
		m, ok := companies[pk]
		switch {
		case !ok:
			out = append(out, decision{LegacyPK: pk, Review: "no company in ctech-account carries source_ref=" + pk})
		case m.CompanyID == "":
			out = append(out, decision{LegacyPK: pk, Review: "the company for " + pk + " has no id"})
		case m.OrganizationID == "":
			// Without it the company record cannot answer "which workspace",
			// and every authorization check on it would fail after the flip.
			out = append(out, decision{LegacyPK: pk, Review: "the company for " + pk + " has no organization_id"})
		case m.TaxID == "":
			// The issuer document comes off this field after the flip. A
			// company without one cannot emit, and IssuerDoc would answer empty
			// — which fails loudly, but only once somebody tries to issue.
			out = append(out, decision{LegacyPK: pk, Review: "the company for " + pk + " has no tax_id"})
		default:
			out = append(out, decision{LegacyPK: pk, Mapping: m})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LegacyPK < out[j].LegacyPK })
	return out
}

// report renders the decisions for a human. It is the whole output of a dry
// run, and the thing somebody reads before allowing the flip.
func report(decisions []decision) string {
	var b strings.Builder
	var ready, review int
	for _, d := range decisions {
		if d.NeedsHuman() {
			review++
			fmt.Fprintf(&b, "  REVIEW  %s — %s\n", d.LegacyPK, d.Review)
			continue
		}
		ready++
		fmt.Fprintf(&b, "  ready   %s → %s (org %s, %s)\n",
			d.LegacyPK, d.Mapping.CompanyID, d.Mapping.OrganizationID, d.Mapping.TaxID)
	}
	fmt.Fprintf(&b, "\n%d ready, %d need a human\n", ready, review)
	return b.String()
}

// mismatch is one table failing verification for one organization.
type mismatch struct {
	Table     string
	Partition string
	Reason    string
}

// enrichedTable is the one table whose copy is deliberately NOT identical to
// its source: the company record gains the platform identity, which is what
// every issuer document is read from after the flip.
//
// Its rows are compared with the identity removed from the copy, so the check
// still catches a row whose real content changed. What it cannot catch is a
// wrong identity — identityVerified does that, separately, because "the copy
// matches" and "the identity is right" are two questions and one answer for
// both would hide either.
const enrichedTable = "organizations"

// verify compares what was copied against what was there.
//
// Counts AND bodies: a count-only check passes on a copy that wrote every row
// with the wrong content, and the flip is irreversible in practice once
// customers have emitted under the new key.
func verify(tableName, partition string, source, copied map[string]string) []mismatch {
	var out []mismatch
	if len(source) != len(copied) {
		out = append(out, mismatch{tableName, partition,
			fmt.Sprintf("row count %d in the copy, %d at the source", len(copied), len(source))})
	}
	missing := make([]string, 0)
	differs := make([]string, 0)
	for sk, body := range source {
		got, ok := copied[sk]
		switch {
		case !ok:
			missing = append(missing, sk)
		case got != body:
			differs = append(differs, sk)
		}
	}
	sort.Strings(missing)
	sort.Strings(differs)
	for _, sk := range missing {
		out = append(out, mismatch{tableName, partition, "missing row " + sk})
	}
	for _, sk := range differs {
		out = append(out, mismatch{tableName, partition, "row " + sk + " differs from the source"})
	}
	// A row in the copy that is not at the source is not an error to fail on —
	// it is a row somebody wrote under the new key after the copy started,
	// which is exactly what the write freeze exists to prevent. Reported, so a
	// broken freeze is visible.
	for sk := range copied {
		if _, ok := source[sk]; !ok {
			out = append(out, mismatch{tableName, partition,
				"row " + sk + " exists only in the copy — was the write freeze in place?"})
		}
	}
	return out
}
