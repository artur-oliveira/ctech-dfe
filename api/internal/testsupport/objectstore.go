// Package testsupport holds test doubles shared by unit and integration tests.
package testsupport

import (
	"context"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type version struct {
	id      string
	body    string
	deleted bool
}

// ObjectStore is an in-memory, versioned S3 double.
type ObjectStore struct {
	mu      sync.Mutex
	next    int
	objects map[string][]*version // "bucket/key" → versions, oldest first
}

func NewObjectStore() *ObjectStore { return &ObjectStore{objects: map[string][]*version{}} }

// Put adds a new version of key.
func (s *ObjectStore) Put(bucket, key, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	k := bucket + "/" + key
	s.objects[k] = append(s.objects[k], &version{id: strconv.Itoa(s.next), body: body})
}

// Versions counts the versions of key that were not deleted.
func (s *ObjectStore) Versions(bucket, key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, v := range s.objects[bucket+"/"+key] {
		if !v.deleted {
			n++
		}
	}
	return n
}

// Keys lists the keys of bucket that still have a version.
func (s *ObjectStore) Keys(bucket string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for k, vs := range s.objects {
		key, ok := strings.CutPrefix(k, bucket+"/")
		if !ok {
			continue
		}
		for _, v := range vs {
			if !v.deleted {
				out = append(out, key)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func (s *ObjectStore) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vs := s.objects[aws.ToString(in.Bucket)+"/"+aws.ToString(in.Key)]
	for i := len(vs) - 1; i >= 0; i-- {
		if !vs[i].deleted {
			return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(vs[i].body))}, nil
		}
	}
	return nil, &s3types.NoSuchKey{}
}

func (s *ObjectStore) ListObjectVersions(_ context.Context, in *s3.ListObjectVersionsInput, _ ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket, prefix := aws.ToString(in.Bucket), aws.ToString(in.Prefix)
	out := &s3.ListObjectVersionsOutput{IsTruncated: aws.Bool(false)}
	keys := make([]string, 0, len(s.objects))
	for k := range s.objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		key, ok := strings.CutPrefix(k, bucket+"/")
		if !ok || !strings.HasPrefix(key, prefix) {
			continue
		}
		for _, v := range s.objects[k] {
			if !v.deleted {
				out.Versions = append(out.Versions, s3types.ObjectVersion{Key: aws.String(key), VersionId: aws.String(v.id)})
			}
		}
	}
	return out, nil
}

func (s *ObjectStore) DeleteObjects(_ context.Context, in *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, o := range in.Delete.Objects {
		for _, v := range s.objects[aws.ToString(in.Bucket)+"/"+aws.ToString(o.Key)] {
			if v.id == aws.ToString(o.VersionId) {
				v.deleted = true
			}
		}
	}
	return &s3.DeleteObjectsOutput{}, nil
}
