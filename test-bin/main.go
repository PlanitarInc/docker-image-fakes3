package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/AdRoll/goamz/aws"
	"github.com/AdRoll/goamz/s3"
	"github.com/google/go-cmp/cmp"
)

var (
	fakes3host = getenv("HOST", "172.17.0.1")
	fakes3port = getenv("PORT", "4567")
	bucketname = "testbucket"
)

func getenv(key, defval string) string {
	val := os.Getenv(key)
	if val == "" {
		return defval
	}
	return val
}

func main() {
	auth := aws.Auth{
		AccessKey: "abc",
		SecretKey: "123",
	}
	fakeRegion := aws.Region{
		Name:       "fakes3",
		S3Endpoint: fmt.Sprintf("http://%s:%s", fakes3host, fakes3port),
	}
	s := s3.New(auth, fakeRegion)
	bucket := s.Bucket(bucketname)
	err := bucket.PutBucket(s3.BucketOwnerFull)
	if err != nil {
		panic(err.Error())
	}

	_, err = bucket.List("", "/", "", 20)
	if err != nil {
		panic(err.Error())
	}

	if err := testPutGet(bucket); err != nil {
		panic(err.Error())
	}

	if err := testCopyObject(bucket); err != nil {
		panic(err.Error())
	}

	if err := testCopyObject404(bucket); err != nil {
		panic(err.Error())
	}

	if err := testDeleteObjects(bucket); err != nil {
		panic(err.Error())
	}

	if err := testListEmptyDelimiter(bucket); err != nil {
		panic(err.Error())
	}
}

func testPutGet(b *s3.Bucket) error {
	key := "file.txt"
	content := []byte("content")

	err := b.Put(key, content, "text/plain", s3.BucketOwnerFull, s3.Options{})
	if err != nil {
		return fmt.Errorf("cannot write file: %w", err)
	}

	bs, err := b.Get(key)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	if !bytes.Equal(bs, content) {
		return fmt.Errorf("content doesn't match:\n- expected: '%s'\n- got: '%s'",
			content, bs)
	}

	return nil
}

func testCopyObject(b *s3.Bucket) error {
	srcKey := "copy/src.txt"
	srcContent := []byte("source")

	err := b.Put(srcKey, srcContent, "text/plain", s3.BucketOwnerFull, s3.Options{})
	if err != nil {
		return fmt.Errorf("cannot write src file: %w", err)
	}

	dstKey := "copy/dst.txt"

	source := fmt.Sprintf("/%s/%s", b.Name, srcKey)
	_, err = b.PutCopy(dstKey, s3.BucketOwnerFull, s3.CopyOptions{}, source)
	if err != nil {
		return fmt.Errorf("cannot copy file: %w", err)
	}

	bs, err := b.Get(dstKey)
	if err != nil {
		return fmt.Errorf("cannot read dst file: %w", err)
	}

	if !bytes.Equal(bs, srcContent) {
		return fmt.Errorf("content doesn't match:\n- expected: '%s'\n- got: '%s'",
			srcContent, bs)
	}

	return nil
}

func testCopyObject404(b *s3.Bucket) error {
	dstKey := "copy-404/dst.txt"

	source := fmt.Sprintf("/%s/copy-404/@@@unknown-file@@@", b.Name)
	_, err := b.PutCopy(dstKey, s3.BucketOwnerFull, s3.CopyOptions{}, source)
	if err == nil {
		return fmt.Errorf("expected copy to fail with 404, got <nil>")
	}

	s3err, ok := err.(*s3.Error)
	if !ok {
		return fmt.Errorf("expected copy to fail with 404, got (%T) %w", err, err)
	}

	if s3err.StatusCode != 404 || s3err.Code != "NoSuchKey" {
		return fmt.Errorf("expected copy to fail with 404, "+
			"got code=%s status=%d: %w", s3err.Code, s3err.StatusCode, err)
	}

	return nil
}

func testDeleteObjects(b *s3.Bucket) error {
	var err error

	err = b.Put("multi-del/a.txt", []byte("1"), "text/plain",
		s3.BucketOwnerFull, s3.Options{})
	if err != nil {
		return fmt.Errorf("cannot write 'a.txt': %w", err)
	}

	err = b.Put("multi-del/b.txt", []byte("2"), "text/plain",
		s3.BucketOwnerFull, s3.Options{})
	if err != nil {
		return fmt.Errorf("cannot write 'a.txt': %w", err)
	}

	err = b.Put("multi-del/c.txt", []byte("3"), "text/plain",
		s3.BucketOwnerFull, s3.Options{})
	if err != nil {
		return fmt.Errorf("cannot write 'a.txt': %w", err)
	}

	err = b.Put("multi-del/d.txt", []byte("4"), "text/plain",
		s3.BucketOwnerFull, s3.Options{})
	if err != nil {
		return fmt.Errorf("cannot write 'a.txt': %w", err)
	}

	err = b.Put("multi-del/e.txt", []byte("4"), "text/plain",
		s3.BucketOwnerFull, s3.Options{})
	if err != nil {
		return fmt.Errorf("cannot write 'a.txt': %w", err)
	}

	res, err := b.List("multi-del/", "/", "", 100)
	if err != nil {
		return fmt.Errorf("cannot list files: %w", err)
	}

	if err := assertListRespKeys(res, []string{
		"multi-del/a.txt",
		"multi-del/b.txt",
		"multi-del/c.txt",
		"multi-del/d.txt",
		"multi-del/e.txt",
	}); err != nil {
		return fmt.Errorf("list initial files: %w", err)
	}

	// Try to delete multi-del/b.txt and some version of multi-del/e.txt.
	// Only multi-del/b.txt should be gone, multi-del/e.txt is not deleted
	// because deleting versions is not supported.
	if err := b.DelMulti(s3.Delete{Objects: []s3.Object{
		{Key: "multi-del/b.txt"},
		{Key: "multi-del/e.txt", VersionId: "v1"},
	}}); err != nil {
		return fmt.Errorf("cannot delete files: %w", err)
	}

	res, err = b.List("multi-del/", "/", "", 100)
	if err != nil {
		return fmt.Errorf("cannot list files: %w", err)
	}

	if err := assertListRespKeys(res, []string{
		"multi-del/a.txt",
		"multi-del/c.txt",
		"multi-del/d.txt",
		"multi-del/e.txt",
	},
	); err != nil {
		return fmt.Errorf("list initial files: %w", err)
	}

	// Try to delete multi-del/a.txt, multi-del/e.txt and already removed
	// multi-del/b.txt.
	// Success should be reported, multi-del/c.txt should be gone.
	if err := b.DelMulti(s3.Delete{Objects: []s3.Object{
		{Key: "multi-del/a.txt"},
		{Key: "multi-del/b.txt"},
		{Key: "multi-del/e.txt"},
	}}); err != nil {
		return fmt.Errorf("cannot delete files: %w", err)
	}

	res, err = b.List("multi-del/", "/", "", 100)
	if err != nil {
		return fmt.Errorf("cannot list files: %w", err)
	}

	if err := assertListRespKeys(res, []string{
		"multi-del/c.txt",
		"multi-del/d.txt",
	}); err != nil {
		return fmt.Errorf("list remaining files: %w", err)
	}

	// Remove the rest of the files. Success should be reported.
	if err := b.DelMulti(s3.Delete{Objects: []s3.Object{
		{Key: "multi-del/a.txt"},
		{Key: "multi-del/b.txt"},
		{Key: "multi-del/c.txt"},
		{Key: "multi-del/d.txt"},
		{Key: "multi-del/e.txt"},
	}}); err != nil {
		return fmt.Errorf("cannot delete files: %w", err)
	}

	res, err = b.List("multi-del/", "/", "", 100)
	if err != nil {
		return fmt.Errorf("cannot list files: %w", err)
	}

	if err := assertListRespKeys(res, []string{}); err != nil {
		return fmt.Errorf("list final files: %w", err)
	}

	return nil
}

func testListEmptyDelimiter(b *s3.Bucket) error {
	var err error

	err = b.Put("empty-del/a.txt", []byte("1"), "text/plain",
		s3.BucketOwnerFull, s3.Options{})
	if err != nil {
		return fmt.Errorf("cannot write 'a.txt': %w", err)
	}

	err = b.Put("empty-del/one/b.txt", []byte("2"), "text/plain",
		s3.BucketOwnerFull, s3.Options{})
	if err != nil {
		return fmt.Errorf("cannot write 'a.txt': %w", err)
	}

	err = b.Put("empty-del/one/c.txt", []byte("3"), "text/plain",
		s3.BucketOwnerFull, s3.Options{})
	if err != nil {
		return fmt.Errorf("cannot write 'a.txt': %w", err)
	}

	err = b.Put("empty-del/one/two/d.txt", []byte("4"), "text/plain",
		s3.BucketOwnerFull, s3.Options{})
	if err != nil {
		return fmt.Errorf("cannot write 'a.txt': %w", err)
	}

	err = b.Put("empty-del/f/o/u/r/e.txt", []byte("5"), "text/plain",
		s3.BucketOwnerFull, s3.Options{})
	if err != nil {
		return fmt.Errorf("cannot write 'a.txt': %w", err)
	}

	err = b.Put("empty-del/f/o/u/r/f.txt", []byte("6"), "text/plain",
		s3.BucketOwnerFull, s3.Options{})
	if err != nil {
		return fmt.Errorf("cannot write 'a.txt': %w", err)
	}

	err = b.Put("empty-del/g.txt", []byte("7"), "text/plain",
		s3.BucketOwnerFull, s3.Options{})
	if err != nil {
		return fmt.Errorf("cannot write 'a.txt': %w", err)
	}

	// Delimiter is set to '/', only elements
	res, err := b.List("empty-del/", "/", "", 100)
	if err != nil {
		return fmt.Errorf("cannot list files: %w", err)
	}

	if err := assertListRespKeys(res, []string{
		"empty-del/a.txt",
		"empty-del/g.txt",
	}); err != nil {
		return fmt.Errorf("list initial files: %w", err)
	}

	if err := assertListRespCommonPrefixes(res, []string{
		"empty-del/f/",
		"empty-del/one/",
	}); err != nil {
		return fmt.Errorf("list initial files: %w", err)
	}

	res, err = b.List("empty-del/", "", "", 100)
	if err != nil {
		return fmt.Errorf("cannot list files: %w", err)
	}

	if err := assertListRespKeys(res, []string{
		"empty-del/a.txt",
		"empty-del/f/o/u/r/e.txt",
		"empty-del/f/o/u/r/f.txt",
		"empty-del/g.txt",
		"empty-del/one/b.txt",
		"empty-del/one/c.txt",
		"empty-del/one/two/d.txt",
	}); err != nil {
		return fmt.Errorf("list initial files: %w", err)
	}

	if err := assertListRespCommonPrefixes(res, nil); err != nil {
		return fmt.Errorf("list initial files: %w", err)
	}

	return nil
}

func assertListRespKeys(res *s3.ListResp, expKeys []string) error {
	keys := make([]string, 0, len(res.Contents))
	for _, k := range res.Contents {
		keys = append(keys, k.Key)
	}

	d := cmp.Diff(keys, expKeys)
	if d == "" {
		return nil
	}

	return fmt.Errorf(`unexpected keys in list result:

  actual:   %v
  expected: %v

  diff:
%s
`, keys, expKeys, d)
}

func assertListRespCommonPrefixes(res *s3.ListResp, expPrefixes []string) error {
	d := cmp.Diff(res.CommonPrefixes, expPrefixes)
	if d == "" {
		return nil
	}

	return fmt.Errorf(`unexpected common-prefixes in list result:

  actual:   %v
  expected: %v

  diff:
%s
`, res.CommonPrefixes, expPrefixes, d)
}
