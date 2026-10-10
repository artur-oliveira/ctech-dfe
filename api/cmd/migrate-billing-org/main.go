// Command migrate-billing-org moves the DF-e subscriptions from the owner's user
// (USER_{sub}) to the ctech-account organizations that hold the companies
// (ORG_{organization_id}). docs/specs/2026-10-10-organization-subscription.md § 4.
//
// Dry run by default; -apply writes. Idempotent: a second -apply creates
// nothing. A user whose subscription has any non-zero price is listed and left
// alone, never duplicated.
//
//	AWS_REGION=… BILLING_API_URL=… BILLING_CLIENT_ID=… BILLING_CLIENT_SECRET=… CTECH_URL=… \
//	ACCOUNT_CLIENT_ID=… ACCOUNT_CLIENT_SECRET=… \
//	ACCOUNT_WORKSPACE_CLIENT_ID=… ACCOUNT_WORKSPACE_CLIENT_SECRET=… \
//	go run ./cmd/migrate-billing-org -table-prefix prod_dfe [-report-levels-all] [-apply]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"gopkg.aoctech.app/api-commons/accountorgs"
	"gopkg.aoctech.app/api-commons/awsconfig"
	"gopkg.aoctech.app/api-commons/cache"

	"gopkg.aoctech.app/dfe/api/internal/accountclient"
	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/billingclient"
	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	exitError  = 1
	exitUsage  = 2
	exitReview = 3
)

func main() {
	prefix := flag.String("table-prefix", "", "ctech-dfe table prefix (e.g. prod_dfe)")
	region := flag.String("region", "us-east-1", "AWS region")
	apply := flag.Bool("apply", false, "write; without it nothing is written")
	reportLevelsAll := flag.Bool("report-levels-all", false, "also report the dfe_companies level of every organization holding DF-e companies")
	flag.Parse()
	if *prefix == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: migrate-billing-org -table-prefix PREFIX [-region R] [-report-levels-all] [-apply]")
		os.Exit(exitUsage)
	}

	ctx := context.Background()
	d, err := wire(ctx, *prefix, *region)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate-billing-org: %v\n", err)
		os.Exit(exitError)
	}
	d.reportLevelsAll = *reportLevelsAll
	rep, err := run(ctx, d, *apply)
	rep.Print(os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nmigrate-billing-org failed: %v\n", err)
		os.Exit(exitError)
	}
	if rep.NeedsReview() {
		os.Exit(exitReview)
	}
}

func wire(ctx context.Context, prefix, region string) (deps, error) {
	awsCfg, err := awsconfig.Load(ctx, region)
	if err != nil {
		return deps{}, fmt.Errorf("loading aws config: %w", err)
	}
	db := dynamodb.NewFromConfig(awsCfg)
	cfg := &config.Config{TablePrefix: prefix, AWSRegion: region}
	mem := cache.NewMemoryBackend(1000)
	ctechURL := os.Getenv("CTECH_URL")
	tokenURL := billingclient.TokenURLFor(ctechURL)

	bill := billingclient.New(billingclient.Config{
		BaseURL: os.Getenv("BILLING_API_URL"), TokenURL: tokenURL,
		ClientID: os.Getenv("BILLING_CLIENT_ID"), ClientSecret: os.Getenv("BILLING_CLIENT_SECRET"), Cache: mem,
	})
	if bill == nil {
		return deps{}, fmt.Errorf("BILLING_API_URL, BILLING_CLIENT_ID, BILLING_CLIENT_SECRET and CTECH_URL are required")
	}
	workspaces := accountorgs.New(accountorgs.Config{
		BaseURL: ctechURL, TokenURL: tokenURL,
		ClientID: os.Getenv("ACCOUNT_WORKSPACE_CLIENT_ID"), ClientSecret: os.Getenv("ACCOUNT_WORKSPACE_CLIENT_SECRET"), Cache: mem,
	})
	if workspaces == nil {
		return deps{}, fmt.Errorf("ACCOUNT_WORKSPACE_CLIENT_ID and ACCOUNT_WORKSPACE_CLIENT_SECRET are required")
	}
	var reach reacher
	if c := accountclient.New(accountclient.Config{
		BaseURL: ctechURL, TokenURL: tokenURL,
		ClientID: os.Getenv("ACCOUNT_CLIENT_ID"), ClientSecret: os.Getenv("ACCOUNT_CLIENT_SECRET"), Cache: mem,
	}); c != nil {
		reach = c
	}

	billingRepo := repositories.NewAccountBillingRepository(db, cfg)
	orgRepo := repositories.NewOrganizationRepository(db, cfg)
	auditRepo := repositories.NewAuditLogRepository(db, cfg)
	certRepo := repositories.NewCertificateRepository(db, cfg)
	orgUserRepo := repositories.NewOrgUserRepository(db, cfg)
	roleRepo := repositories.NewRoleRepository(db, cfg)
	memberSvc := services.NewMembershipService(orgUserRepo, auditRepo, roleRepo, mem)
	certSvc := services.NewCertificateService(certRepo, auditRepo, &awsclient.Clients{}, "")
	orgSvc := services.NewOrganizationService(orgRepo, auditRepo, certRepo, orgUserRepo, certSvc, memberSvc, mem)
	// The enablement source is required: the level reported is the count of
	// ENABLED companies, and without it companiesUsed counts every linked one.
	billingSvc := services.NewBillingService(billingRepo, bill, nil, orgSvc, mem).
		WithEnablement(services.NewFiscalConfigEnablement(
			repositories.NewNfeConfigRepository(db, cfg), repositories.NewNfceConfigRepository(db, cfg),
			repositories.NewCteConfigRepository(db, cfg), repositories.NewMdfeConfigRepository(db, cfg),
			repositories.NewNfseConfigRepository(db, cfg)))
	levels := services.NewLevelReporter(billingRepo, bill, billingSvc)

	return deps{
		snaps: billingRepo, billing: bill, orgs: billingSvc, workspaces: workspaces,
		companies: orgRepo, levels: levels, reach: reach, now: time.Now,
	}, nil
}
