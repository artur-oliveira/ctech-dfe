// Package certcrypt decrypts certificate PFX passwords that the API encrypted
// with KMS before persisting/publishing them (backlog B4).
//
// Format: "kms1:" + base64(std, KMS ciphertext blob). A value without the
// prefix is a legacy plaintext password and is returned unchanged. This is
// the decrypt-only mirror of api/internal/certcrypt — keep the format in
// sync (same cross-module drift class as WorkerMessage, backlog B17).
package certcrypt

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// Prefix marks a KMS-encrypted password value.
const Prefix = "kms1:"

// KMSClient is the KMS subset used by the decoder.
type KMSClient interface {
	Decrypt(ctx context.Context, params *kms.DecryptInput, optFns ...func(*kms.Options)) (*kms.DecryptOutput, error)
}

// Decoder resolves stored password values, memoizing KMS decrypts per process
// (Lambda containers are reused, so steady-state adds no KMS calls).
type Decoder struct {
	kms KMSClient

	mu    sync.Mutex
	cache map[string]string // ciphertext value → plaintext
	// ponytail: unbounded cache; bounded in practice by the number of distinct
	// certificates a container touches. Move to an LRU if that ever grows.
}

func NewDecoder(kmsClient KMSClient) *Decoder {
	return &Decoder{kms: kmsClient, cache: map[string]string{}}
}

// Decrypt resolves a stored password value: "kms1:"-prefixed values are
// decrypted via KMS (the ciphertext blob embeds the key id); anything else is
// legacy plaintext and returned as-is.
func (d *Decoder) Decrypt(ctx context.Context, value string) (string, error) {
	if len(value) < len(Prefix) || value[:len(Prefix)] != Prefix {
		return value, nil
	}
	d.mu.Lock()
	plain, ok := d.cache[value]
	d.mu.Unlock()
	if ok {
		return plain, nil
	}
	blob, err := base64.StdEncoding.DecodeString(value[len(Prefix):])
	if err != nil {
		return "", fmt.Errorf("certcrypt: bad ciphertext encoding: %w", err)
	}
	out, err := d.kms.Decrypt(ctx, &kms.DecryptInput{CiphertextBlob: blob})
	if err != nil {
		return "", fmt.Errorf("certcrypt: kms decrypt: %w", err)
	}
	plain = string(out.Plaintext)
	d.mu.Lock()
	d.cache[value] = plain
	d.mu.Unlock()
	return plain, nil
}
