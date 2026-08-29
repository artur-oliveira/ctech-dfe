package documents

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type fakeObjectStore struct {
	exists bool
	puts   int
	put    *s3.PutObjectInput
}

func (f *fakeObjectStore) HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if !f.exists {
		return nil, &smithy.GenericAPIError{Code: s3ErrorNotFound, Message: "missing"}
	}
	return &s3.HeadObjectOutput{}, nil
}

func (f *fakeObjectStore) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.puts++
	f.put = input
	f.exists = true
	_, _ = io.ReadAll(input.Body)
	return &s3.PutObjectOutput{}, nil
}

type fakePresigner struct{}

func (fakePresigner) PresignGetObject(context.Context, *s3.GetObjectInput, ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	return &v4.PresignedHTTPRequest{URL: "https://s3.example.invalid/document.pdf"}, nil
}

type fakeRenderer struct{ calls int }

func (f *fakeRenderer) Render(context.Context, string, []byte, bool) ([]byte, error) {
	f.calls++
	return []byte("%PDF-test"), nil
}

func TestServiceCachesThenReturnsPresignedURL(t *testing.T) {
	store := &fakeObjectStore{}
	renderer := &fakeRenderer{}
	service := newService(store, fakePresigner{}, renderer, "documents")
	fixedNow := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	first, err := service.GetURL(context.Background(), "CNPJ_12345678000190", DocTypeNFe, testNFeKey, []byte("<xml/>"), false)
	if err != nil {
		t.Fatal(err)
	}
	if first.Cached || renderer.calls != 1 || store.puts != 1 {
		t.Fatalf("first request = %+v, render calls=%d, puts=%d", first, renderer.calls, store.puts)
	}
	if aws.ToString(store.put.Tagging) != cacheTagging || aws.ToString(store.put.IfNoneMatch) != putIfAbsent {
		t.Fatalf("unexpected PutObject cache controls: %+v", store.put)
	}

	second, err := service.GetURL(context.Background(), "CNPJ_12345678000190", DocTypeNFe, testNFeKey, []byte("<xml/>"), false)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Cached || renderer.calls != 1 || store.puts != 1 {
		t.Fatalf("second request = %+v, render calls=%d, puts=%d", second, renderer.calls, store.puts)
	}
	if !second.ExpiresAt.Equal(fixedNow.Add(presignedURLTTL)) {
		t.Fatalf("expires_at = %s", second.ExpiresAt)
	}
}

func TestCacheKeySeparatesCanceledDocuments(t *testing.T) {
	active := cacheKey("CNPJ_12345678000190", DocTypeNFe, testNFeKey, false)
	canceled := cacheKey("CNPJ_12345678000190", DocTypeNFe, testNFeKey, true)
	if active == canceled {
		t.Fatal("active and canceled cache keys must differ")
	}
	want := "pdfs/nfe/CNPJ_12345678000190/v1/" + testNFeKey + "-active.pdf"
	if active != want {
		t.Fatalf("active cache key = %q, want %q", active, want)
	}
}
