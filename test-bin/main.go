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

	mustPutTextFiles(b, []textFile{
		{"multi-del/a.txt", "1"},
		{"multi-del/b.txt", "2"},
		{"multi-del/c.txt", "3"},
		{"multi-del/d.txt", "4"},
		{"multi-del/e.txt", "5"},
	})

	res, err := b.List("multi-del/", "/", "", 100)
	if err != nil {
		return fmt.Errorf("cannot list files: %w", err)
	}

	assertListRespKeys(res, []string{
		"multi-del/a.txt",
		"multi-del/b.txt",
		"multi-del/c.txt",
		"multi-del/d.txt",
		"multi-del/e.txt",
	})

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

	assertListRespKeys(res, []string{
		"multi-del/a.txt",
		"multi-del/c.txt",
		"multi-del/d.txt",
		"multi-del/e.txt",
	})

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

	assertListRespKeys(res, []string{
		"multi-del/c.txt",
		"multi-del/d.txt",
	})

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

	assertListRespKeys(res, nil)

	return nil
}

func testListEmptyDelimiter(b *s3.Bucket) error {
	var err error

	mustPutTextFiles(b, []textFile{
		{"empty-del/a.txt", "1"},
		{"empty-del/one/b.txt", "2"},
		{"empty-del/one/c.txt", "3"},
		{"empty-del/one/two/d.txt", "4"},
		{"empty-del/f/o/u/r/e.txt", "5"},
		{"empty-del/f/o/u/r/f.txt", "6"},
		{"empty-del/g.txt", "7"},
	})

	// Delimiter is set to '/', only elements
	res, err := b.List("empty-del/", "/", "", 100)
	if err != nil {
		return fmt.Errorf("cannot list files: %w", err)
	}

	assertListRespKeysCommonPrefixes(res, []string{
		"empty-del/a.txt",
		"empty-del/g.txt",
	}, []string{
		"empty-del/f/",
		"empty-del/one/",
	})

	res, err = b.List("empty-del/", "", "", 100)
	if err != nil {
		return fmt.Errorf("cannot list files: %w", err)
	}

	assertListRespKeysCommonPrefixes(res, []string{
		"empty-del/a.txt",
		"empty-del/f/o/u/r/e.txt",
		"empty-del/f/o/u/r/f.txt",
		"empty-del/g.txt",
		"empty-del/one/b.txt",
		"empty-del/one/c.txt",
		"empty-del/one/two/d.txt",
	}, nil)

	return nil
}

type textFile struct {
	Key, Content string
}

func mustPutTextFiles(b *s3.Bucket, files []textFile) {
	for _, f := range files {
		err := b.Put(f.Key, []byte(f.Content), "text/plain",
			s3.BucketOwnerFull, s3.Options{})
		if err != nil {
			panic(fmt.Sprintf("cannot write '%s': %s", f.Key, err))
		}
	}
}

func assertListRespKeysCommonPrefixes(res *s3.ListResp, expKeys, expPrefixes []string) {
	assertListRespKeys(res, expKeys)
	assertStringList(res.CommonPrefixes, expPrefixes,
		"Unexpected common-prefixes in list result")
}

func assertListRespKeys(res *s3.ListResp, expKeys []string) {
	var keys []string
	for _, c := range res.Contents {
		keys = append(keys, c.Key)
	}

	assertStringList(keys, expKeys, "unexpected keys in list result")
}

func assertStringList(actual, expected []string, message string) {
	d := cmp.Diff(actual, expected)
	if d == "" {
		return
	}

	panic(fmt.Errorf(`%s:

  actual:   %v
  expected: %v

  diff:
%s
`, message, actual, expected, d))
}
