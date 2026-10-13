package services

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// ObjectStore is the S3 surface the account-deletion purge and the XML export
// use. *s3.Client satisfies it; tests use testsupport.ObjectStore.
type ObjectStore interface {
	GetObject(ctx context.Context, in *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectVersions(ctx context.Context, in *s3.ListObjectVersionsInput, opts ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error)
	DeleteObjects(ctx context.Context, in *s3.DeleteObjectsInput, opts ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
}

const s3DeleteBatch = 1000 // DeleteObjects limit

// purgeObjects deletes every version and delete marker under prefix (inventory
// §2: versioned buckets keep old versions). exact limits it to the key equal to
// prefix, because a key is also a prefix of its siblings. An empty prefix is a
// no-op: nothing here may ever sweep a whole bucket.
func purgeObjects(ctx context.Context, store ObjectStore, bucket, prefix string, exact bool) (int, error) {
	if prefix == "" {
		return 0, nil
	}
	n := 0
	var keyMarker, versionMarker *string
	for {
		out, err := store.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
			Bucket: aws.String(bucket), Prefix: aws.String(prefix),
			KeyMarker: keyMarker, VersionIdMarker: versionMarker,
		})
		if err != nil {
			return n, fmt.Errorf("s3: list versions of %s: %w", prefix, err)
		}
		var ids []s3types.ObjectIdentifier
		add := func(key, versionID *string) {
			if exact && aws.ToString(key) != prefix {
				return
			}
			ids = append(ids, s3types.ObjectIdentifier{Key: key, VersionId: versionID})
		}
		for _, v := range out.Versions {
			add(v.Key, v.VersionId)
		}
		for _, d := range out.DeleteMarkers {
			add(d.Key, d.VersionId)
		}
		for len(ids) > 0 {
			batch := ids[:min(s3DeleteBatch, len(ids))]
			ids = ids[len(batch):]
			res, err := store.DeleteObjects(ctx, &s3.DeleteObjectsInput{
				Bucket: aws.String(bucket),
				Delete: &s3types.Delete{Objects: batch, Quiet: aws.Bool(true)},
			})
			if err != nil {
				return n, fmt.Errorf("s3: delete versions of %s: %w", prefix, err)
			}
			if len(res.Errors) > 0 {
				return n, fmt.Errorf("s3: delete versions of %s: %s", prefix, aws.ToString(res.Errors[0].Message))
			}
			n += len(batch)
		}
		if !aws.ToBool(out.IsTruncated) {
			return n, nil
		}
		keyMarker, versionMarker = out.NextKeyMarker, out.NextVersionIdMarker
	}
}
