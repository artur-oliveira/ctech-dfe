// Package certcrypt encrypts certificate PFX passwords with KMS before they
// are persisted in DynamoDB or published on SNS/SQS (backlog B4 — plaintext
// PFX password exposure).
//
// Stored/message format: "kms1:" + base64(std, KMS ciphertext blob). A value
// without the prefix is a legacy plaintext row and is returned unchanged —
// re-uploading the certificate migrates it. The worker carries a mirror of
// the decrypt half (worker/internal/certcrypt) — keep the format in sync
// (same cross-module drift class as WorkerMessage, backlog B17).
package certcrypt

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// Prefix marks a KMS-encrypted password value.
const Prefix = "kms1:"

// Codec encrypts and decrypts certificate passwords via KMS. The zero-value
// keyID disables encryption (dev/local); decryption always works regardless.
type Codec struct {
	kms   *kms.Client
	keyID string

	mu    sync.Mutex
	cache map[string]string // ciphertext value → plaintext
	// ponytail: unbounded cache; bounded in practice by the number of distinct
	// certificates a process touches. Move to an LRU if that ever grows.
}

func New(kmsClient *kms.Client, keyID string) *Codec {
	return &Codec{kms: kmsClient, keyID: keyID, cache: map[string]string{}}
}

// Encrypt returns the "kms1:"-prefixed ciphertext of plain, or plain itself
// when no key is configured (dev/local).
func (c *Codec) Encrypt(ctx context.Context, plain string) (string, error) {
	if c.keyID == "" {
		return plain, nil
	}
	out, err := c.kms.Encrypt(ctx, &kms.EncryptInput{
		KeyId:     aws.String(c.keyID),
		Plaintext: []byte(plain),
	})
	if err != nil {
		return "", fmt.Errorf("certcrypt: kms encrypt: %w", err)
	}
	return Prefix + base64.StdEncoding.EncodeToString(out.CiphertextBlob), nil
}

// Decrypt resolves a stored password value: "kms1:"-prefixed values are
// decrypted via KMS (memoized per process — the KMS ciphertext blob embeds
// the key id, so no key configuration is needed); anything else is legacy
// plaintext and returned as-is.
func (c *Codec) Decrypt(ctx context.Context, value string) (string, error) {
	if len(value) < len(Prefix) || value[:len(Prefix)] != Prefix {
		return value, nil
	}
	c.mu.Lock()
	plain, ok := c.cache[value]
	c.mu.Unlock()
	if ok {
		return plain, nil
	}
	blob, err := base64.StdEncoding.DecodeString(value[len(Prefix):])
	if err != nil {
		return "", fmt.Errorf("certcrypt: bad ciphertext encoding: %w", err)
	}
	out, err := c.kms.Decrypt(ctx, &kms.DecryptInput{CiphertextBlob: blob})
	if err != nil {
		return "", fmt.Errorf("certcrypt: kms decrypt: %w", err)
	}
	plain = string(out.Plaintext)
	c.mu.Lock()
	c.cache[value] = plain
	c.mu.Unlock()
	return plain, nil
}
