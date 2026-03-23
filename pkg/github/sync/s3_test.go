package sync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
)

type mockS3Client struct {
	putErr  error
	getErr  error
	objects map[string]mockS3Object
}

type mockS3Object struct {
	data        []byte
	contentType string
}

func newMockS3Client() *mockS3Client {
	return &mockS3Client{
		objects: make(map[string]mockS3Object),
	}
}

func (m *mockS3Client) PutObject(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.putErr != nil {
		return nil, m.putErr
	}
	data, _ := io.ReadAll(params.Body)
	key := *params.Key
	ct := ""
	if params.ContentType != nil {
		ct = *params.ContentType
	}
	m.objects[key] = mockS3Object{data: data, contentType: ct}
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) GetObject(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	key := *params.Key
	obj, ok := m.objects[key]
	if !ok {
		return nil, fmt.Errorf("NoSuchKey: %s", key)
	}
	ct := obj.contentType
	return &s3.GetObjectOutput{
		Body:        io.NopCloser(bytes.NewReader(obj.data)),
		ContentType: &ct,
	}, nil
}

func TestNewS3Destination(t *testing.T) {
	client := newMockS3Client()
	dest := NewS3Destination(client, "my-bucket")

	assert.NotNil(t, dest)
	assert.Equal(t, client, dest.client)
	assert.Equal(t, "my-bucket", dest.bucket)
}

func TestS3Destination_Write(t *testing.T) {
	client := newMockS3Client()
	dest := NewS3Destination(client, "my-bucket")

	err := dest.Write(context.Background(), "path/to/file.md", []byte("# Hello"), "text/markdown")
	assert.NoError(t, err)

	obj, ok := client.objects["path/to/file.md"]
	assert.True(t, ok)
	assert.Equal(t, []byte("# Hello"), obj.data)
	assert.Equal(t, "text/markdown", obj.contentType)
}

func TestS3Destination_WriteError(t *testing.T) {
	client := newMockS3Client()
	client.putErr = fmt.Errorf("access denied")
	dest := NewS3Destination(client, "my-bucket")

	err := dest.Write(context.Background(), "path/to/file.md", []byte("data"), "text/plain")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestS3Destination_Read(t *testing.T) {
	client := newMockS3Client()
	client.objects["path/to/file.md"] = mockS3Object{
		data:        []byte("# Hello"),
		contentType: "text/markdown",
	}
	dest := NewS3Destination(client, "my-bucket")

	data, contentType, err := dest.Read(context.Background(), "path/to/file.md")
	assert.NoError(t, err)
	assert.Equal(t, []byte("# Hello"), data)
	assert.Equal(t, "text/markdown", contentType)
}

func TestS3Destination_ReadNotFound(t *testing.T) {
	client := newMockS3Client()
	dest := NewS3Destination(client, "my-bucket")

	data, contentType, err := dest.Read(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NoSuchKey")
	assert.Nil(t, data)
	assert.Empty(t, contentType)
}

func TestS3Destination_ReadError(t *testing.T) {
	client := newMockS3Client()
	client.getErr = fmt.Errorf("service unavailable")
	dest := NewS3Destination(client, "my-bucket")

	data, contentType, err := dest.Read(context.Background(), "some/key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service unavailable")
	assert.Nil(t, data)
	assert.Empty(t, contentType)
}

func TestS3Destination_ReadAfterWrite(t *testing.T) {
	client := newMockS3Client()
	dest := NewS3Destination(client, "my-bucket")
	ctx := context.Background()

	err := dest.Write(ctx, "org/repo/v1/README.md", []byte("content"), "text/plain")
	assert.NoError(t, err)

	data, contentType, err := dest.Read(ctx, "org/repo/v1/README.md")
	assert.NoError(t, err)
	assert.Equal(t, []byte("content"), data)
	assert.Equal(t, "text/plain", contentType)
}
