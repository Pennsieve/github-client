package sync

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"mime"
	"path/filepath"
	gosync "sync"
)

type ContentResponse struct {
	Content  string
	Encoding string
}

type ContentFetcher interface {
	GetContent(url string, filePath string, tag string) (*ContentResponse, error)
}

type Destination interface {
	Write(ctx context.Context, key string, data []byte, contentType string) error
	Read(ctx context.Context, key string) ([]byte, string, error)
}

type Config struct {
	RepoUrl   string
	Tag       string
	Namespace string
	Files     []string
}

type Result struct {
	File  string
	Key   string
	Error error
}

func SyncContent(ctx context.Context, logger *slog.Logger, fetcher ContentFetcher, config Config, dest Destination) []Result {
	results := make([]Result, len(config.Files))
	var wg gosync.WaitGroup

	for i, file := range config.Files {
		wg.Add(1)
		go func(idx int, filePath string) {
			defer wg.Done()

			key := fmt.Sprintf("%s/%s", config.Namespace, filePath)
			result := Result{File: filePath, Key: key}

			content, err := fetcher.GetContent(config.RepoUrl, filePath, config.Tag)
			if err != nil {
				result.Error = fmt.Errorf("failed to get content for %s: %w", filePath, err)
				logger.Error(result.Error.Error())
				results[idx] = result
				return
			}

			if content == nil {
				result.Error = fmt.Errorf("file not found: %s", filePath)
				logger.Warn(result.Error.Error())
				results[idx] = result
				return
			}

			decoded, err := base64.StdEncoding.DecodeString(content.Content)
			if err != nil {
				result.Error = fmt.Errorf("failed to decode content for %s: %w", filePath, err)
				logger.Error(result.Error.Error())
				results[idx] = result
				return
			}

			contentType := detectContentType(filePath)

			if err := dest.Write(ctx, key, decoded, contentType); err != nil {
				result.Error = fmt.Errorf("failed to write %s to destination: %w", filePath, err)
				logger.Error(result.Error.Error())
				results[idx] = result
				return
			}

			logger.Info(fmt.Sprintf("synced %s to %s", filePath, key))
			results[idx] = result
		}(i, file)
	}

	wg.Wait()
	return results
}

func detectContentType(filePath string) string {
	ext := filepath.Ext(filePath)
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
}
