package documents

import (
	"context"
	"encoding/json"
	"io"
	"strings"
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

type fakePresigner struct {
	input   *s3.GetObjectInput
	expires time.Duration
}

func (f *fakePresigner) PresignGetObject(_ context.Context, input *s3.GetObjectInput, options ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	f.input = input
	var opts s3.PresignOptions
	for _, option := range options {
		option(&opts)
	}
	f.expires = opts.Expires
	return &v4.PresignedHTTPRequest{URL: "https://s3.example.invalid/document.pdf"}, nil
}

type fakeRenderer struct{ calls int }

func (f *fakeRenderer) Render(context.Context, string, []byte, DocumentState) ([]byte, error) {
	f.calls++
	return []byte("%PDF-test"), nil
}

func TestServiceCachesThenReturnsPresignedURL(t *testing.T) {
	store := &fakeObjectStore{}
	renderer := &fakeRenderer{}
	service := newService(store, &fakePresigner{}, renderer, "documents")
	fixedNow := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	first, err := service.GetURL(context.Background(), "CNPJ_12345678000190", DocTypeNFe, testNFeKey, []byte("<xml/>"), StateActive)
	if err != nil {
		t.Fatal(err)
	}
	if first.Cached == nil || *first.Cached || renderer.calls != 1 || store.puts != 1 {
		t.Fatalf("first request = %+v, render calls=%d, puts=%d", first, renderer.calls, store.puts)
	}
	if aws.ToString(store.put.Tagging) != cacheTagging || aws.ToString(store.put.IfNoneMatch) != putIfAbsent {
		t.Fatalf("unexpected PutObject cache controls: %+v", store.put)
	}

	second, err := service.GetURL(context.Background(), "CNPJ_12345678000190", DocTypeNFe, testNFeKey, []byte("<xml/>"), StateActive)
	if err != nil {
		t.Fatal(err)
	}
	if second.Cached == nil || !*second.Cached || renderer.calls != 1 || store.puts != 1 {
		t.Fatalf("second request = %+v, render calls=%d, puts=%d", second, renderer.calls, store.puts)
	}
	if !second.ExpiresAt.Equal(fixedNow.Add(presignedURLTTL)) {
		t.Fatalf("expires_at = %s", second.ExpiresAt)
	}
}

func TestSignFileSetsSafeDownloadMetadataAndOmitsCacheForSourceXML(t *testing.T) {
	presigner := &fakePresigner{}
	service := newService(&fakeObjectStore{}, presigner, &fakeRenderer{}, "documents")
	fixedNow := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	download, err := service.SignFile(context.Background(), "org/nfse/source.xml", "../evento\r\n.xml", ContentTypeXML)
	if err != nil {
		t.Fatal(err)
	}
	if download.Filename != "evento.xml" || download.ContentType != ContentTypeXML {
		t.Fatalf("download metadata = %+v", download)
	}
	if presigner.expires != presignedURLTTL {
		t.Fatalf("presign expiry = %s", presigner.expires)
	}
	if aws.ToString(presigner.input.Bucket) != "documents" || aws.ToString(presigner.input.Key) != "org/nfse/source.xml" {
		t.Fatalf("presigned object = %+v", presigner.input)
	}
	if aws.ToString(presigner.input.ResponseContentType) != ContentTypeXML {
		t.Fatalf("response content type = %q", aws.ToString(presigner.input.ResponseContentType))
	}
	disposition := aws.ToString(presigner.input.ResponseContentDisposition)
	if !strings.Contains(disposition, "evento.xml") || strings.ContainsAny(disposition, "\r\n") {
		t.Fatalf("unsafe content disposition = %q", disposition)
	}
	body, err := json.Marshal(download)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"cached"`) {
		t.Fatalf("source XML must omit cached: %s", body)
	}
}

func TestCacheKeySeparatesDocumentStates(t *testing.T) {
	keys := map[string]DocumentState{}
	for _, state := range []DocumentState{StateActive, StateCancelled, StateSubstituted} {
		key := cacheKey("CNPJ_12345678000190", DocTypeNFe, testNFeKey, state)
		if previous, clash := keys[key]; clash {
			t.Fatalf("estados %s e %s compartilham a chave de cache %s", previous, state, key)
		}
		keys[key] = state
	}
	active := cacheKey("CNPJ_12345678000190", DocTypeNFe, testNFeKey, StateActive)
	want := "pdfs/nfe/CNPJ_12345678000190/v1/" + testNFeKey + "-active.pdf"
	if active != want {
		t.Fatalf("active cache key = %q, want %q", active, want)
	}
}
