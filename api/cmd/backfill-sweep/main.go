// Command backfill-sweep stamps sweep = "1" on the rows that predate the
// `sweep-index` GSI (fiscal configs of nfe/cte/mdfe/nfse, and certificates), so
// the distribution dispatcher and the account-erasure e-CPF search can list those
// tables with a Query. New rows get the attribute from the repositories.
//
//	backfill-sweep -dfe-table-prefix prod_dfe          # plan: counts, writes nothing
//	backfill-sweep -dfe-table-prefix prod_dfe -apply   # stamp
//
// This is a migration tool: it Scans on purpose (it must visit rows the index
// cannot see yet) and needs dynamodb:Scan, which the service roles no longer
// have. Run it with operator credentials. Safe to rerun; it never removes anything.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/awsconfig"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

var tables = []string{
	"organization_nfe_configs",
	"organization_cte_configs",
	"organization_mdfe_configs",
	"organization_nfse_configs",
	"organization_certificates",
}

func main() {
	prefix := flag.String("dfe-table-prefix", "", "table prefix of the ctech-dfe tables (e.g. prod_dfe)")
	region := flag.String("region", "us-east-1", "AWS region")
	apply := flag.Bool("apply", false, "stamp the rows (default: report only)")
	flag.Parse()
	if *prefix == "" {
		fmt.Fprintln(os.Stderr, "-dfe-table-prefix is required")
		os.Exit(2)
	}
	ctx := context.Background()
	cfg, err := awsconfig.Load(ctx, *region)
	if err != nil {
		fmt.Fprintln(os.Stderr, "aws config:", err)
		os.Exit(1)
	}
	db := dynamodb.NewFromConfig(cfg)
	for _, t := range tables {
		n, err := backfill(ctx, db, *prefix+"_"+t, *apply)
		fmt.Printf("%s: %d row(s) without sweep (apply=%v)\n", *prefix+"_"+t, n, *apply)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	}
}

func backfill(ctx context.Context, db *dynamodb.Client, table string, apply bool) (int, error) {
	n := 0
	var start map[string]types.AttributeValue
	for {
		res, err := db.Scan(ctx, &dynamodb.ScanInput{
			TableName:                aws.String(table),
			FilterExpression:         aws.String("attribute_not_exists(#s)"),
			ProjectionExpression:     aws.String("pk, sk"),
			ExpressionAttributeNames: map[string]string{"#s": repositories.AttrSweep},
			ExclusiveStartKey:        start,
		})
		if err != nil {
			return n, fmt.Errorf("scan %s: %w", table, err)
		}
		for _, key := range res.Items {
			n++
			if !apply {
				continue
			}
			if _, err := db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
				TableName:                 aws.String(table),
				Key:                       key,
				UpdateExpression:          aws.String("SET #s = :v"),
				ConditionExpression:       aws.String("attribute_exists(pk)"),
				ExpressionAttributeNames:  map[string]string{"#s": repositories.AttrSweep},
				ExpressionAttributeValues: map[string]types.AttributeValue{":v": &types.AttributeValueMemberS{Value: repositories.SweepValue}},
			}); err != nil {
				return n, fmt.Errorf("stamp %s: %w", table, err)
			}
		}
		if len(res.LastEvaluatedKey) == 0 {
			return n, nil
		}
		start = res.LastEvaluatedKey
	}
}
