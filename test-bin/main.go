package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/AdRoll/goamz/aws"
	"github.com/AdRoll/goamz/s3"
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
