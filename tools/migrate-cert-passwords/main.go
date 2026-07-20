// Command migrate-cert-passwords is a TEMPORARY dev tool for B4 (certificate
// password encryption, Option A: static AES-256 key in SSM SecureString +
// AES-256-GCM in-app). It is NOT part of the runtime; it only prepares data at
// rest. See docs/specs/2026-07-20-cert-password-encryption.md.
//
// Subcommands:
//   genkey   generate a 32-byte AES-256 key, print base64 (store in SSM SecureString)
//   migrate  scan organization_certificates, encrypt plaintext `password` in place
//
// Backward compatible: rows left as plaintext keep working (worker warns + uses
// as-is). Idempotent: already-encrypted rows are skipped.
package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	encPrefix = "enc:v1:"
	keyLen    = 32
	saltLen   = 16
	ivLen     = 12 // AES-GCM standard nonce size
)

func randBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("rand: %v", err)
	}
	return b
}

// deriveKey derives a per-certificate AES key from the master key + salt via
// HKDF. The salt makes every certificate's key unique even though one master
// key lives in SSM — isolation at ~0 cost, no KMS per op.
func deriveKey(master, salt []byte) []byte {
	r := hkdf.New(sha256.New, master, nil, []byte("ctech-dfe-cert-password-v1"))
	cek := make([]byte, keyLen)
	if _, err := io.ReadFull(r, cek); err != nil {
		log.Fatalf("hkdf: %v", err)
	}
	return cek
}

func encrypt(plain, master []byte) (string, error) {
	salt := randBytes(saltLen)
	iv := randBytes(ivLen)
	cek := deriveKey(master, salt)
	block, err := aes.NewCipher(cek)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, iv, plain, nil)
	buf := append(append(append([]byte{}, salt...), iv...), ct...)
	return encPrefix + base64.StdEncoding.EncodeToString(buf), nil
}

func genkey() {
	key := randBytes(keyLen)
	b64 := base64.StdEncoding.EncodeToString(key)
	fmt.Println(b64)
	fmt.Println()
	fmt.Println("# Store in SSM SecureString (restrict policy to the dfe role):")
	fmt.Printf("# aws ssm put-parameter --name /ctech-dfe/$ENV/cert-encryption-key --type SecureString --value %s --overwrite\n", b64)
}

func migrate(table, keyB64 string) {
	master, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		log.Fatalf("invalid DFE_AES_KEY_B64: %v", err)
	}
	if len(master) != keyLen {
		log.Fatalf("key must be %d bytes, got %d", keyLen, len(master))
	}
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}
	db := dynamodb.NewFromConfig(cfg)

	ctx := context.Background()
	var scanned, encrypted, skipped, empt int
	var lastKey map[string]ddbtypes.AttributeValue
	for {
		out, err := db.Scan(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(table),
			ExclusiveStartKey: lastKey,
			Limit:             aws.Int32(100),
		})
		if err != nil {
			log.Fatalf("scan: %v", err)
		}
		for _, item := range out.Items {
			scanned++
			pk, sk := attrS(item, "pk"), attrS(item, "sk")
			if pk == "" || sk == "" {
				log.Printf("skip: missing pk/sk in item")
				skipped++
				continue
			}
			pw := attrS(item, "password")
			if pw == "" {
				empt++
				continue
			}
			if strings.HasPrefix(pw, encPrefix) {
				skipped++
				continue
			}
			enc, err := encrypt([]byte(pw), master)
			if err != nil {
				log.Fatalf("encrypt pk=%s sk=%s: %v", pk, sk, err)
			}
			_, err = db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
				TableName: aws.String(table),
				Key: map[string]ddbtypes.AttributeValue{
					"pk": &ddbtypes.AttributeValueMemberS{Value: pk},
					"sk": &ddbtypes.AttributeValueMemberS{Value: sk},
				},
				UpdateExpression:         aws.String("SET #p = :e"),
				ConditionExpression:      aws.String("attribute_exists(pk)"),
				ExpressionAttributeNames:  map[string]string{"#p": "password"},
				ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{":e": &ddbtypes.AttributeValueMemberS{Value: enc}},
			})
			if err != nil {
				log.Fatalf("update pk=%s sk=%s: %v", pk, sk, err)
			}
			encrypted++
		}
		lastKey = out.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}
	log.Printf("done: scanned=%d encrypted=%d skipped(already-or-missing-key)=%d empty=%d", scanned, encrypted, skipped, empt)
}

func attrS(item map[string]ddbtypes.AttributeValue, key string) string {
	if v, ok := item[key]; ok {
		if s, ok := v.(*ddbtypes.AttributeValueMemberS); ok {
			return s.Value
		}
	}
	return ""
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: migrate-cert-passwords <genkey|migrate> [flags]")
	}
	switch os.Args[1] {
	case "genkey":
		genkey()
	case "migrate":
		fs := flag.NewFlagSet("migrate", flag.ExitOnError)
		table := fs.String("table", os.Getenv("TABLE_NAME"), "full DynamoDB table name, e.g. devorganization_certificates")
		fs.Parse(os.Args[2:])
		if *table == "" {
			log.Fatal("--table (or TABLE_NAME env) required")
		}
		keyB64 := os.Getenv("DFE_AES_KEY_B64")
		if keyB64 == "" {
			log.Fatal("DFE_AES_KEY_B64 env required (output of genkey)")
		}
		migrate(*table, keyB64)
	default:
		log.Fatalf("unknown subcommand %q", os.Args[1])
	}
}
