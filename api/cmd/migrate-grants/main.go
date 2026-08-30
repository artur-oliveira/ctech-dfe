// Command migrate-grants records, in ctech-account, the reach that ctech-dfe's
// own authorization rows already assume.
//
// It runs AFTER the company re-key and after the platform can express which
// companies an invitation grants — ctech-billing ADR 0023 fixes that order:
// decide the owner, extend the invitation, then move the grants. Moving them
// first hands them to nobody empowered to change them.
//
//	migrate-grants -plan   -dfe-table-prefix prod_dfe -account-companies prod_account_companies
//	migrate-grants -apply  ...
//	migrate-grants -verify ...
//
// **It writes nothing to ctech-dfe.** The role and the permissions are already
// where ADR 0023 says they belong: this product owns the verbs. What -apply
// writes is the missing EDGES in ctech-account — the platform's record that a
// person may reach the company they are already authorized in.
//
// There is deliberately no verb that removes anything. An overlay row whose edge
// was revoked is an orphan, and collecting orphans is a separate decision with
// its own reasoning; a tool that could do it here would do it by a mistyped flag.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"gopkg.aoctech.app/api-commons/awsconfig"
)

func main() {
	var (
		dfePrefix    = flag.String("dfe-table-prefix", "", "table prefix of the ctech-dfe tables (e.g. prod_dfe)")
		accountTable = flag.String("account-companies", "", "physical name of ctech-account's companies table")
		region       = flag.String("region", "us-east-1", "AWS region")
		doPlan       = flag.Bool("plan", false, "report what would be granted, write nothing")
		doApply      = flag.Bool("apply", false, "write the missing edges in ctech-account")
		doVerify     = flag.Bool("verify", false, "check every authorized row has its edge, exit non-zero otherwise")
	)
	flag.Parse()

	if *dfePrefix == "" || *accountTable == "" {
		fmt.Fprintln(os.Stderr, "both -dfe-table-prefix and -account-companies are required")
		os.Exit(2)
	}
	if countTrue(*doPlan, *doApply, *doVerify) != 1 {
		fmt.Fprintln(os.Stderr, "pass exactly one of -plan, -apply or -verify")
		os.Exit(2)
	}

	ctx := context.Background()
	if err := run(ctx, *region, *dfePrefix, *accountTable, *doPlan, *doApply, *doVerify); err != nil {
		fmt.Fprintf(os.Stderr, "\nmigrate-grants failed: %v\n", err)
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

func run(ctx context.Context, region, dfePrefix, accountTable string, doPlan, doApply, doVerify bool) error {
	cfg, err := awsconfig.Load(ctx, region)
	if err != nil {
		return fmt.Errorf("loading aws config: %w", err)
	}
	s := &store{db: dynamodb.NewFromConfig(cfg), dfePrefix: dfePrefix, accountTable: accountTable}

	overlays, err := s.listOverlays(ctx)
	if err != nil {
		return err
	}
	companyOrg, err := s.companyOrganizations(ctx)
	if err != nil {
		return err
	}
	reaches, err := s.reaches(ctx, overlays)
	if err != nil {
		return err
	}

	decisions := plan(overlays, reaches, companyOrg)
	fmt.Print(report(decisions))

	if n := countReviews(decisions); n > 0 {
		// A refusal stops everything, not just its own row. The reviews are
		// usually one cause across several rows, and granting the rest first
		// makes the re-run harder to reason about.
		return fmt.Errorf("%d row(s) need a human; nothing was written", n)
	}
	if doPlan {
		return nil
	}

	if doVerify {
		// Every authorized row must have its edge. A row without one grants
		// nothing after the flip, which is a person who could work yesterday
		// and cannot today — and the whole reason to verify before believing
		// the migration.
		missing := 0
		for _, d := range decisions {
			if d.GrantEdge {
				fmt.Printf("  MISSING %s / %s\n", d.Overlay.CompanyID, d.Overlay.UserID)
				missing++
			}
		}
		if missing > 0 {
			return fmt.Errorf("%d authorized row(s) have no edge; do NOT rely on the unification yet", missing)
		}
		fmt.Println("  every authorized row has its edge")
		return nil
	}

	if doApply {
		for _, d := range decisions {
			if !d.GrantEdge {
				continue
			}
			orgID := companyOrg[d.Overlay.CompanyID]
			if err := s.grantEdge(ctx, orgID, d.Overlay.CompanyID, d.Overlay.UserID); err != nil {
				return err
			}
			fmt.Printf("  granted %s / %s in %s\n", d.Overlay.CompanyID, d.Overlay.UserID, orgID)
		}
	}
	return nil
}
