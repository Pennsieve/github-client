package sync

import (
	"bytes"
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Client interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type S3Destination struct {
	client S3Client
	bucket string
}

func NewS3Destination(client S3Client, bucket string) *S3Destination {
	return &S3Destination{
		client: client,
		bucket: bucket,
	}
}

func (d *S3Destination) Write(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := d.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(d.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	return err
}

func (d *S3Destination) Read(ctx context.Context, key string) ([]byte, string, error) {
	out, err := d.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", err
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, "", err
	}

	contentType := ""
	if out.ContentType != nil {
		contentType = *out.ContentType
	}
	return data, contentType, nil
}
