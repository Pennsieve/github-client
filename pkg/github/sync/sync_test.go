package sync

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	gosync "sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockFetcher struct {
	files map[string]string
}

func (m *mockFetcher) GetContent(_ string, filePath string, _ string) (*ContentResponse, error) {
	content, ok := m.files[filePath]
	if !ok {
		return nil, nil
	}
	return &ContentResponse{
		Content:  base64.StdEncoding.EncodeToString([]byte(content)),
		Encoding: "base64",
	}, nil
}

type mockDestination struct {
	mu           gosync.Mutex
	written      map[string][]byte
	contentTypes map[string]string
	err          error
}

func newMockDestination() *mockDestination {
	return &mockDestination{
		written:      make(map[string][]byte),
		contentTypes: make(map[string]string),
	}
}

func (m *mockDestination) Write(_ context.Context, key string, data []byte, contentType string) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.written[key] = data
	m.contentTypes[key] = contentType
	return nil
}

func (m *mockDestination) Read(_ context.Context, key string) ([]byte, string, error) {
	if m.err != nil {
		return nil, "", m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.written[key]
	if !ok {
		return nil, "", fmt.Errorf("key not found: %s", key)
	}
	return data, m.contentTypes[key], nil
}

func TestSyncContent_Success(t *testing.T) {
	fetcher := &mockFetcher{
		files: map[string]string{
			"README.md":  "# Test Repo",
			"config.yml": "key: value",
		},
	}

	dest := newMockDestination()
	config := Config{
		RepoUrl:   "https://github.com/testorg/testrepo",
		Tag:       "v1.0.0",
		Namespace: "testorg/testrepo/v1.0.0",
		Files:     []string{"README.md", "config.yml"},
	}

	results := SyncContent(context.Background(), slog.Default(), fetcher, config, dest)

	assert.Len(t, results, 2)
	for _, r := range results {
		assert.NoError(t, r.Error)
	}

	assert.Equal(t, []byte("# Test Repo"), dest.written["testorg/testrepo/v1.0.0/README.md"])
	assert.Equal(t, []byte("key: value"), dest.written["testorg/testrepo/v1.0.0/config.yml"])
}

func TestSyncContent_FileNotFound(t *testing.T) {
	fetcher := &mockFetcher{files: map[string]string{}}

	dest := newMockDestination()
	config := Config{
		RepoUrl:   "https://github.com/testorg/testrepo",
		Tag:       "v1.0.0",
		Namespace: "testorg/testrepo/v1.0.0",
		Files:     []string{"missing.txt"},
	}

	results := SyncContent(context.Background(), slog.Default(), fetcher, config, dest)

	assert.Len(t, results, 1)
	assert.Error(t, results[0].Error)
	assert.Contains(t, results[0].Error.Error(), "file not found")
	assert.Empty(t, dest.written)
}

func TestSyncContent_DestinationError(t *testing.T) {
	fetcher := &mockFetcher{
		files: map[string]string{"README.md": "# Test"},
	}

	dest := &mockDestination{
		written:      make(map[string][]byte),
		contentTypes: make(map[string]string),
		err:          fmt.Errorf("s3 write failed"),
	}
	config := Config{
		RepoUrl:   "https://github.com/testorg/testrepo",
		Tag:       "v1.0.0",
		Namespace: "testorg/testrepo/v1.0.0",
		Files:     []string{"README.md"},
	}

	results := SyncContent(context.Background(), slog.Default(), fetcher, config, dest)

	assert.Len(t, results, 1)
	assert.Error(t, results[0].Error)
	assert.Contains(t, results[0].Error.Error(), "failed to write")
}

func TestSyncContent_EmptyFileList(t *testing.T) {
	fetcher := &mockFetcher{files: map[string]string{}}

	dest := newMockDestination()
	config := Config{
		RepoUrl:   "https://github.com/testorg/testrepo",
		Tag:       "v1.0.0",
		Namespace: "testorg/testrepo/v1.0.0",
		Files:     []string{},
	}

	results := SyncContent(context.Background(), slog.Default(), fetcher, config, dest)

	assert.Len(t, results, 0)
	assert.Empty(t, dest.written)
}

func TestSyncContent_PartialFailure(t *testing.T) {
	fetcher := &mockFetcher{
		files: map[string]string{"README.md": "# Test"},
	}

	dest := newMockDestination()
	config := Config{
		RepoUrl:   "https://github.com/testorg/testrepo",
		Tag:       "v1.0.0",
		Namespace: "testorg/testrepo/v1.0.0",
		Files:     []string{"README.md", "missing.txt"},
	}

	results := SyncContent(context.Background(), slog.Default(), fetcher, config, dest)

	assert.Len(t, results, 2)

	var succeeded, failed int
	for _, r := range results {
		if r.Error != nil {
			failed++
		} else {
			succeeded++
		}
	}
	assert.Equal(t, 1, succeeded)
	assert.Equal(t, 1, failed)
	assert.Equal(t, []byte("# Test"), dest.written["testorg/testrepo/v1.0.0/README.md"])
}

func TestDestination_ReadAfterWrite(t *testing.T) {
	dest := newMockDestination()
	ctx := context.Background()

	err := dest.Write(ctx, "org/repo/v1.0.0/README.md", []byte("# Hello"), "text/markdown")
	assert.NoError(t, err)

	data, contentType, err := dest.Read(ctx, "org/repo/v1.0.0/README.md")
	assert.NoError(t, err)
	assert.Equal(t, []byte("# Hello"), data)
	assert.Equal(t, "text/markdown", contentType)
}

func TestDestination_ReadNotFound(t *testing.T) {
	dest := newMockDestination()

	data, contentType, err := dest.Read(context.Background(), "nonexistent/key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
	assert.Nil(t, data)
	assert.Empty(t, contentType)
}

func TestDestination_ReadError(t *testing.T) {
	dest := &mockDestination{
		written:      make(map[string][]byte),
		contentTypes: make(map[string]string),
		err:          fmt.Errorf("s3 unavailable"),
	}

	data, contentType, err := dest.Read(context.Background(), "some/key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "s3 unavailable")
	assert.Nil(t, data)
	assert.Empty(t, contentType)
}

func TestDetectContentType(t *testing.T) {
	tests := map[string]struct {
		filePath string
	}{
		"markdown": {filePath: "README.md"},
		"yaml":     {filePath: "config.yml"},
		"json":     {filePath: "data.json"},
		"unknown":  {filePath: "file.xyz"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := detectContentType(tc.filePath)
			assert.NotEmpty(t, result)
		})
	}
}
