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
	mu      gosync.Mutex
	written map[string][]byte
	err     error
}

func newMockDestination() *mockDestination {
	return &mockDestination{written: make(map[string][]byte)}
}

func (m *mockDestination) Write(_ context.Context, key string, data []byte, _ string) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.written[key] = data
	return nil
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
		written: make(map[string][]byte),
		err:     fmt.Errorf("s3 write failed"),
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
