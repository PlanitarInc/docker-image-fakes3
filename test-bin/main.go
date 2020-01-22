package main

import (
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
}
