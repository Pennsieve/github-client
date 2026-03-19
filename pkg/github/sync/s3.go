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
	Client S3Client
	Bucket string
}

func (d *S3Destination) Write(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := d.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(d.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	return err
}

func (d *S3Destination) Read(ctx context.Context, key string) ([]byte, string, error) {
	out, err := d.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(d.Bucket),
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
