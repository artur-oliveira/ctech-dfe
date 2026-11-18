// Command push-keygen is the operator's one-shot tool for Web Push keys. It generates a VAPID key pair and PRINTS the
// `aws ssm put-parameter` commands that store it; it writes nothing itself and talks to no AWS service, so running it
// changes no environment. The operator reviews the output and runs the commands (or pipes them to a shell) on purpose.
// docs/specs/2026-11-17-web-push.md, DOCS.md ("Web Push").
//
//	go run ./cmd/push-keygen -env prod [-rotate]
//
// -rotate prints the extra command that moves the key now in SSM to the `-previous` parameter first, so devices
// subscribed with the old key keep working until their clients resubscribe (30 days, then remove `-previous`).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"gopkg.aoctech.app/api-commons/webpush"
)

// paramPrefix is where the DF-e keeps its Web Push parameters: /ctech-dfe/{env}/push/...
const paramPrefix = "/ctech-dfe/%s/push/"

func main() {
	env := flag.String("env", "", "environment name (dev, stage, prod); required")
	rotate := flag.Bool("rotate", false, "also print the command that keeps the current private key as -previous")
	flag.Parse()
	if err := run(os.Stdout, *env, *rotate); err != nil {
		fmt.Fprintln(os.Stderr, "push-keygen:", err)
		os.Exit(2)
	}
}

func run(out interface{ Write([]byte) (int, error) }, env string, rotate bool) error {
	if strings.TrimSpace(env) == "" || strings.ContainsAny(env, " /'\"") {
		return fmt.Errorf("-env is required (dev, stage or prod)")
	}
	pem, key, err := webpush.GenerateKey()
	if err != nil {
		return err
	}
	prefix := fmt.Sprintf(paramPrefix, env)
	// The first write error is kept and returned: a broken pipe must not look like a printed key pair.
	var werr error
	w := func(format string, a ...any) {
		if werr == nil {
			_, werr = fmt.Fprintf(out, format, a...)
		}
	}
	w("# VAPID key pair %s for %s. Review, then run the commands below. Nothing has been written.\n", key.KID, env)
	w("# The private key is printed once and not kept anywhere else: store it or discard this output.\n\n")
	if rotate {
		w("# 1. Keep the current private key as -previous (clients resubscribe within 30 days; then delete it).\n")
		w("PREV=$(aws ssm get-parameter --with-decryption --name %svapid-private-key --query Parameter.Value --output text)\n", prefix)
		w("aws ssm put-parameter --type SecureString --overwrite --name %svapid-private-key-previous --value \"$PREV\"\n\n", prefix)
		w("# 2. Write the new pair.\n")
	}
	w("aws ssm put-parameter --type SecureString --overwrite --name %svapid-private-key --value '%s'\n", prefix, strings.TrimSpace(pem))
	w("aws ssm put-parameter --type String --overwrite --name %svapid-public-key --value '%s'\n", prefix, key.PublicBase64URL())
	w("\n# The API reads the public key at start (restart it); the sender reads all of them on its next cold start.\n")
	return werr
}
