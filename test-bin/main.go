package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/google/go-cmp/cmp"
)

var (
	fakes3host = getenv("HOST", "172.17.0.1")
	fakes3port = getenv("PORT", "4567")
	bucket     = "testbucket-plntr"
)

func getenv(key, defval string) string {
	val := os.Getenv(key)
	if val == "" {
		return defval
	}
	return val
}

func main() {
	svc := s3.New(session.Must(session.NewSession(&aws.Config{
		Region:           aws.String("fakes3"),
		Endpoint:         aws.String(fmt.Sprintf("http://%s:%s", fakes3host, fakes3port)),
		DisableSSL:       aws.Bool(true),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials("key", "secret", ""),
	})))

	_, err := svc.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucket),
		ACL:    aws.String("bucket-owner-full-control"),
	})
	if err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok || aerr.Code() != s3.ErrCodeBucketAlreadyOwnedByYou {
			panic(err.Error())
		}
	}

	_, err = svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Delimiter: aws.String("/"),
	})
	assertOK(err)

	testPutGetDelete(svc)
	testCopyObject(svc)
	testCopyObject404(svc)
	testListObjects(svc)
	testDeleteObjects(svc)
	testBigUploadDownload(svc)
}

func testPutGetDelete(svc *s3.S3) {
	const (
		key         = "file.txt"
		content     = "content"
		contentType = "text/plain"
	)

	put, err := svc.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
		ACL:         aws.String("bucket-owner-full-control"),
		Body:        bytes.NewReader([]byte(content)),
	})
	assertOK(err)
	assertAwsStringNotEmpty("ETag", put.ETag)

	obj, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	assertOK(err)
	defer obj.Body.Close()
	assertAwsString("ETag", obj.ETag, *put.ETag)
	assertAwsString("Content-Type", obj.ContentType, contentType)
	assertReader("content", obj.Body, content)

	_, err = svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	assertOK(err)

	_, err = svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		panic(fmt.Sprintf("Object /%s/%s should not exist", bucket, key))
	}
	if aerr, ok := err.(awserr.Error); !ok || aerr.Code() != "NoSuchKey" {
		panic(fmt.Sprintf("Object /%s/%s should not exist: %s", bucket, key, err))
	}
}

func testCopyObject(svc *s3.S3) {
	const (
		srcKey         = "copy/src.txt"
		srcContent     = "source"
		srcContentType = "text/plain"
		dstKey         = "copy/dst.txt"
	)

	defer func() {
		_, _ = svc.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(srcKey),
		})
		_, _ = svc.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(dstKey),
		})
	}()

	put, err := svc.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(srcKey),
		ContentType: aws.String(srcContentType),
		ACL:         aws.String("bucket-owner-full-control"),
		Body:        bytes.NewReader([]byte(srcContent)),
	})
	assertOK(err)
	assertAwsStringNotEmpty("ETag", put.ETag)

	res, err := svc.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(dstKey),
		ACL:        aws.String("bucket-owner-full-control"),
		CopySource: aws.String(fmt.Sprintf("/%s/%s", bucket, srcKey)),
	})
	assertOK(err)
	assertAwsString("ETag", res.CopyObjectResult.ETag, *put.ETag)

	obj, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(dstKey),
	})
	assertOK(err)
	defer obj.Body.Close()
	assertAwsString("ETag", obj.ETag, *put.ETag)
	assertAwsString("Content-Type", obj.ContentType, srcContentType)
	assertReader("content", obj.Body, srcContent)
}

func testCopyObject404(svc *s3.S3) {
	const (
		srcKey = "copy-404/@@@unknown-file@@@"
		dstKey = "copy-404/dst.txt"
	)

	_, err := svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(srcKey),
	})
	if err == nil {
		panic(fmt.Sprintf("Object /%s/%s should not exist", bucket, srcKey))
	}
	if aerr, ok := err.(awserr.Error); !ok || aerr.Code() != "NotFound" {
		panic(fmt.Sprintf("Object /%s/%s should not exist: %s", bucket, srcKey, err))
	}

	_, err = svc.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(dstKey),
		ACL:        aws.String("bucket-owner-full-control"),
		CopySource: aws.String(fmt.Sprintf("/%s/%s", bucket, srcKey)),
	})
	if err == nil {
		panic(fmt.Sprintf("Copy of /%s/%s should fail", bucket, srcKey))
	}
	if aerr, ok := err.(awserr.Error); !ok || aerr.Code() != s3.ErrCodeNoSuchKey {
		panic(fmt.Sprintf("Copy of /%s/%s should fail with NoSuchKey: %s",
			bucket, srcKey, err))
	}
}

func testListObjects(svc *s3.S3) {
	var err error

	mustPutTextFiles(svc, bucket, []textFile{
		{"list/a.txt", "1"},
		{"list/one/b.txt", "2"},
		{"list/one/c.txt", "3"},
		{"list/one/two/d.txt", "4"},
		{"list/f/o/u/r/e.txt", "5"},
		{"list/f/o/u/r/f.txt", "6"},
		{"list/g.txt", "7"},
	})
	defer deleteAllKeys(svc, bucket, []string{
		"list/a.txt",
		"list/one/b.txt",
		"list/one/c.txt",
		"list/one/two/d.txt",
		"list/f/o/u/r/e.txt",
		"list/f/o/u/r/f.txt",
		"list/g.txt",
	})

	res, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	assertOK(err)
	assertListReponse(res, []string{
		"list/a.txt",
		"list/f/o/u/r/e.txt",
		"list/f/o/u/r/f.txt",
		"list/g.txt",
		"list/one/b.txt",
		"list/one/c.txt",
		"list/one/two/d.txt",
	}, nil)

	res, err = svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Delimiter: aws.String("/"),
	})
	assertOK(err)
	assertListReponse(res, nil, []string{"list/"})

	res, err = svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String("list/"),
		Delimiter: aws.String("/"),
	})
	assertOK(err)
	assertListReponse(res, []string{"list/a.txt", "list/g.txt"},
		[]string{"list/f/", "list/one/"})

	res, err = svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String("list"),
		Delimiter: aws.String("/"),
	})
	assertOK(err)
	assertListReponse(res, nil, []string{"list/"})

	res, err = svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String("list"),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int64(2),
	})
	assertOK(err)
	assertListReponse(res, nil, []string{"list/"})

	res, err = svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String("unknown/prefix/"),
	})
	assertOK(err)
	assertListReponse(res, nil, nil)
}

func testDeleteObjects(svc *s3.S3) {
	var err error

	mustPutTextFiles(svc, bucket, []textFile{
		{"multi-del/a.txt", "1"},
		{"multi-del/b.txt", "2"},
		{"multi-del/c.txt", "3"},
		{"multi-del/d.txt", "4"},
		{"multi-del/e.txt", "5"},
	})
	defer deleteAllKeys(svc, bucket, []string{
		"multi-del/a.txt",
		"multi-del/b.txt",
		"multi-del/c.txt",
		"multi-del/d.txt",
		"multi-del/e.txt",
	})

	res, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	assertOK(err)
	assertListReponse(res, []string{
		"multi-del/a.txt",
		"multi-del/b.txt",
		"multi-del/c.txt",
		"multi-del/d.txt",
		"multi-del/e.txt",
	}, nil)

	// Try to delete multi-del/b.txt and non-existent multi-del/z.txt.
	del, err := svc.DeleteObjects(&s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &s3.Delete{
			Objects: []*s3.ObjectIdentifier{
				{Key: aws.String("multi-del/b.txt")},
				{Key: aws.String("multi-del/z.txt")},
			},
		},
	})
	assertOK(err)
	assertDeleteRespKeys(del, []string{
		"multi-del/b.txt",
		"multi-del/z.txt",
	})

	res, err = svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	assertOK(err)
	assertListReponse(res, []string{
		"multi-del/a.txt",
		"multi-del/c.txt",
		"multi-del/d.txt",
		"multi-del/e.txt",
	}, nil)

	// Try to delete multi-del/a.txt, multi-del/e.txt and already removed
	// multi-del/b.txt.
	// Success should be reported, multi-del/c.txt should be gone.
	del, err = svc.DeleteObjects(&s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &s3.Delete{
			Objects: []*s3.ObjectIdentifier{
				{Key: aws.String("multi-del/a.txt")},
				{Key: aws.String("multi-del/b.txt")},
				{Key: aws.String("multi-del/e.txt")},
			},
		},
	})
	assertOK(err)
	assertDeleteRespKeys(del, []string{
		"multi-del/a.txt",
		"multi-del/b.txt",
		"multi-del/e.txt",
	})

	res, err = svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	assertOK(err)
	assertListReponse(res, []string{
		"multi-del/c.txt",
		"multi-del/d.txt",
	}, nil)

	// Remove the rest of the files. Success should be reported.
	del, err = svc.DeleteObjects(&s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &s3.Delete{
			Objects: []*s3.ObjectIdentifier{
				{Key: aws.String("multi-del/a.txt")},
				{Key: aws.String("multi-del/b.txt")},
				{Key: aws.String("multi-del/c.txt")},
				{Key: aws.String("multi-del/d.txt")},
				{Key: aws.String("multi-del/e.txt")},
			},
		},
	})
	assertOK(err)
	assertDeleteRespKeys(del, []string{
		"multi-del/a.txt",
		"multi-del/b.txt",
		"multi-del/c.txt",
		"multi-del/d.txt",
		"multi-del/e.txt",
	})

	res, err = svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	assertOK(err)
	assertListReponse(res, nil, nil)
}

func testBigUploadDownload(svc *s3.S3) {
	const (
		key         = "big-file"
		contentType = "text/plain"
	)
	// 50MB
	content := bytes.Repeat([]byte("123456789_"), 7*1024*1024)

	u := s3manager.NewUploaderWithClient(svc)
	_, err := u.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
		Body:        bytes.NewReader(content),
		ACL:         aws.String("bucket-owner-full-control"),
	})
	assertOK(err)
	defer deleteAllKeys(svc, bucket, []string{key})

	res, err := svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	assertOK(err)
	// XXX Not supported by fakes3
	// assertAwsString("Content-Type", res.ContentType, contentType)
	assertAwsInt64("Content-Length", res.ContentLength, int64(len(content)))

	buf := aws.NewWriteAtBuffer([]byte{})
	d := s3manager.NewDownloaderWithClient(svc)
	_, err = d.Download(buf, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	assertOK(err)
	if e, a := len(content), len(buf.Bytes()); e != a {
		panic(fmt.Sprintf("Wrong length: expected %d, got %d", e, a))
	}
	if !bytes.Equal(content, buf.Bytes()) {
		panic(fmt.Sprintf("Wrong content"))
	}
}

type textFile struct {
	Key, Content string
}

func mustPutTextFiles(svc *s3.S3, bucket string, files []textFile) {
	for _, f := range files {
		_, err := svc.PutObject(&s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(f.Key),
			ContentType: aws.String("text/plain"),
			ACL:         aws.String("bucket-owner-full-control"),
			Body:        bytes.NewReader([]byte(f.Content)),
		})
		assertOK(err)
	}
}

func deleteAllKeys(svc *s3.S3, bucket string, keys []string) {
	for _, k := range keys {
		_, err := svc.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(k),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to delete '/%s/%s'\n", bucket, k)
		}
	}
}

func assertListReponse(res *s3.ListObjectsV2Output, expKeys, expPrefixes []string) {
	var keys []string
	for _, c := range res.Contents {
		keys = append(keys, *c.Key)
	}
	assertStringList(keys, expKeys, "unexpected keys in list result")

	var prefixes []string
	for _, p := range res.CommonPrefixes {
		prefixes = append(prefixes, *p.Prefix)
	}
	assertStringList(prefixes, expPrefixes, "unexpected prefixes in list result")
}

func assertDeleteRespKeys(res *s3.DeleteObjectsOutput, expKeys []string) {
	var keys []string
	for _, c := range res.Deleted {
		keys = append(keys, *c.Key)
	}

	// the order does not matter
	sort.StringSlice(keys).Sort()
	sort.StringSlice(expKeys).Sort()

	assertStringList(keys, expKeys, "unexpected keys in delete result")
}

func assertOK(err error) {
	if err != nil {
		panic(fmt.Sprintf("Expected no error, got %q", err.Error()))
	}
}

func assertAwsStringNotEmpty(name string, actual *string) {
	if actual == nil {
		panic(fmt.Sprintf("%s is missing", name))
	}

	if *actual == "" {
		panic(fmt.Sprintf("%s is empty", name))
	}
}

func assertAwsString(name string, actual *string, expected string) {
	if actual == nil {
		panic(fmt.Sprintf(`%s is empty:
- expected: %q
- actual:   <nil>
`, name, expected))
	}

	if *actual != expected {
		panic(fmt.Sprintf(`%s is wrong:
- expected: %q
- actual:   %q
`, name, expected, *actual))
	}
}

func assertAwsInt64(name string, actual *int64, expected int64) {
	if actual == nil {
		panic(fmt.Sprintf(`%s is empty:
- expected: %q
- actual:   <nil>
`, name, expected))
	}

	if *actual != expected {
		panic(fmt.Sprintf(`%s is wrong:
- expected: %q
- actual:   %q
`, name, expected, *actual))
	}
}

func assertReader(name string, r io.Reader, expected string) {
	bs, err := ioutil.ReadAll(r)
	if err != nil {
		panic(fmt.Sprintf(`failed to read %s: %q`, name, err.Error()))
	}

	if !bytes.Equal(bs, []byte(expected)) {
		panic(fmt.Sprintf(`%s doesn't match:
- expected: %q
- actual:   %q
`, name, string(bs), expected))
	}
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
