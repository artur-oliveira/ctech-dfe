// Command rekey-companies copies every organization-partitioned table in
// ctech-dfe from its legacy CNPJ_/CPF_ key onto the platform company id.
//
// It has three verbs and deliberately no fourth. There is NO -delete: removing
// the old partitions is a separate decision, taken after a full numbering cycle
// has passed under the new key, and a tool that can do it is a tool that does
// it by a mistyped flag. The old partitions are the rollback.
//
//	rekey-companies -plan    -dfe-table-prefix prod_dfe -account-companies prod_account_companies
//	rekey-companies -apply   ...
//	rekey-companies -verify  ...
//
// See docs/specs/2026-08-29-company-rekey.md for the ordering this belongs to:
// deploy, pass one in ctech-account, freeze writes, apply, verify, flip, and
// only much later delete.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/awsconfig"
)

func main() {
	var (
		dfePrefix    = flag.String("dfe-table-prefix", "", "table prefix of the ctech-dfe tables (e.g. prod_dfe)")
		accountTable = flag.String("account-companies", "", "physical name of ctech-account's companies table (e.g. prod_account_companies)")
		region       = flag.String("region", "us-east-1", "AWS region")
		doPlan       = flag.Bool("plan", false, "report what would move, write nothing")
		doApply      = flag.Bool("apply", false, "copy every row onto the company key")
		doVerify     = flag.Bool("verify", false, "compare both partitions, exit non-zero on any mismatch")
		only         = flag.String("org", "", "act on a single legacy organization (e.g. CNPJ_11222333000191)")
	)
	flag.Parse()

	if *dfePrefix == "" || *accountTable == "" {
		fmt.Fprintln(os.Stderr, "both -dfe-table-prefix and -account-companies are required")
		os.Exit(2)
	}
	// Exactly one verb. Two would be ambiguous about ordering, and defaulting
	// to apply is how a rehearsal becomes a migration.
	if countTrue(*doPlan, *doApply, *doVerify) != 1 {
		fmt.Fprintln(os.Stderr, "pass exactly one of -plan, -apply or -verify")
		os.Exit(2)
	}

	ctx := context.Background()
	if err := run(ctx, *region, *dfePrefix, *accountTable, *only, *doPlan, *doApply, *doVerify); err != nil {
		fmt.Fprintf(os.Stderr, "\nrekey failed: %v\n", err)
		os.Exit(1)
	}
}

func countTrue(vs ...bool) int {
	n := 0
	for _, v := range vs {
		if v {
			n++
		}
	}
	return n
}

func run(ctx context.Context, region, dfePrefix, accountTable, only string, doPlan, doApply, doVerify bool) error {
	cfg, err := awsconfig.Load(ctx, region)
	if err != nil {
		return fmt.Errorf("loading aws config: %w", err)
	}
	s := &store{db: dynamodb.NewFromConfig(cfg), dfePrefix: dfePrefix, accountTable: accountTable}

	legacyPKs := []string{only}
	if only == "" {
		legacyPKs, err = s.listLegacyOrganizations(ctx)
		if err != nil {
			return err
		}
	}

	companies := map[string]*mapping{}
	for _, pk := range legacyPKs {
		m, err := s.readMapping(ctx, pk)
		if err != nil {
			return err
		}
		if m != nil {
			companies[pk] = m
		}
	}

	decisions := plan(legacyPKs, companies)
	fmt.Print(report(decisions))

	if doPlan {
		return exitOnReview(decisions)
	}

	// A refusal stops everything, not just its own organization. The reviews
	// are usually one cause across several rows, and copying the rest first
	// makes the eventual re-run harder to reason about.
	if err := exitOnReview(decisions); err != nil {
		return err
	}

	for _, d := range decisions {
		if doApply {
			if err := applyOne(ctx, s, d.Mapping); err != nil {
				return err
			}
			continue
		}
		if doVerify {
			if err := verifyOne(ctx, s, d.Mapping); err != nil {
				return err
			}
		}
	}
	return nil
}

func exitOnReview(decisions []decision) error {
	for _, d := range decisions {
		if d.NeedsHuman() {
			return fmt.Errorf("%d organization(s) need a human; nothing was written", countReviews(decisions))
		}
	}
	return nil
}

func countReviews(decisions []decision) int {
	n := 0
	for _, d := range decisions {
		if d.NeedsHuman() {
			n++
		}
	}
	return n
}

func applyOne(ctx context.Context, s *store, m *mapping) error {
	fmt.Printf("\n%s → %s\n", m.LegacyPK, m.CompanyID)
	for _, t := range tables {
		// The company record carries the platform identity on top of its copy —
		// that record IS the thing the issuer document is read from after the
		// flip. Every other table is copied as-is.
		var extra map[string]types.AttributeValue
		if t.Name == "organizations" {
			extra = identityAttrs(m)
		}
		for i, from := range partitionsFor(t, m.LegacyPK) {
			to := partitionsFor(t, m.CompanyID)[i]
			n, err := s.copyPartition(ctx, t.Name, from, to, extra)
			if err != nil {
				return err
			}
			if n > 0 {
				fmt.Printf("  %-34s %s → %s: %d row(s)\n", t.Name, from, to, n)
			}
		}
	}
	return nil
}

func verifyOne(ctx context.Context, s *store, m *mapping) error {
	var all []mismatch
	for _, t := range tables {
		for i, from := range partitionsFor(t, m.LegacyPK) {
			to := partitionsFor(t, m.CompanyID)[i]
			source, err := s.readPartition(ctx, t.Name, from)
			if err != nil {
				return err
			}
			copied, err := s.readPartition(ctx, t.Name, to)
			if err != nil {
				return err
			}
			all = append(all, verify(t.Name, to, source, copied)...)
		}
	}
	if len(all) == 0 {
		fmt.Printf("  %s → %s: verified\n", m.LegacyPK, m.CompanyID)
		return nil
	}
	for _, mm := range all {
		fmt.Printf("  MISMATCH %-34s %s — %s\n", mm.Table, mm.Partition, mm.Reason)
	}
	return fmt.Errorf("%s: %d mismatch(es); do NOT flip", m.LegacyPK, len(all))
}
