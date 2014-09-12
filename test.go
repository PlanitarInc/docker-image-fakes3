package main

import (
	"fmt"

	"github.com/crowdmob/goamz/aws"
	"github.com/crowdmob/goamz/s3"
)

var (
	fakes3host = "localhost"
	fakes3port = "4567"
	bucketname = "testbucket"
)

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
