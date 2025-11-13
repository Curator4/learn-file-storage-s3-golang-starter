package main

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ch6l6 4. Create a new generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) function.
func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	// 1. Use the SDK to create a s3.PresignClient with s3.NewPresignClient
	presignS3Client := s3.NewPresignClient(s3Client)

	// 2. Use the client's .PresignGetObject() method with s3.WithPresignExpires as a functional option.
	getObjectParams := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	r, err := presignS3Client.PresignGetObject(
		context.Background(),
		&getObjectParams,
		s3.WithPresignExpires(expireTime),
	)
	if err != nil {
		return "", err
	}

	// 3. Return the .URL field of the v4.PresignedHTTPRequest created by .PresignGetObject()
	return r.URL, nil
}
