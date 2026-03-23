# github-client

A Go client library for the GitHub API, built for use within the Pennsieve platform. Provides a type-safe interface for GitHub App authentication, repository operations, and content synchronization to external storage.

## Features

- GitHub App authentication (JWT + installation access tokens)
- OAuth / personal access token support
- Repository metadata (tags, releases, contributors, licenses)
- File content retrieval and release asset downloads
- Parallel content sync to S3 with bounded concurrency
- In-memory token caching with TTL-based expiration

## Installation

```sh
go get github.com/pennsieve/github-client
```

## Usage

### Creating a client

```go
import "github.com/pennsieve/github-client/pkg/github"

client := github.NewGitHubApiClient(logger, appId, appSecret, "https://api.github.com", pennsieveAppId)
```

### Authentication

GitHub App (JWT-based):

```go
client.WithAppPrivateKey(pemBytes)
client.WithInstallationId(installationId)
```

Personal access token / OAuth:

```go
client.WithAccessToken("ghp_...")
```

### Fetching repository data

```go
repo, err := client.GetRepo("https://github.com/owner/repo")
release, err := client.GetRelease("https://github.com/owner/repo", "v1.0.0")
contributors, err := client.GetContributors("https://github.com/owner/repo", "v1.0.0")
readme, err := client.GetReadme("https://github.com/owner/repo")
```

### Syncing content to S3

```go
import (
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/pennsieve/github-client/pkg/github/sync"
)

cfg, _ := config.LoadDefaultConfig(ctx)
dest := sync.NewS3Destination(s3.NewFromConfig(cfg), "my-bucket")

results := sync.SyncContent(ctx, logger, fetcher, sync.Config{
    RepoUrl:   "https://github.com/owner/repo",
    Tag:       "v1.0.0",
    Namespace: "owner/repo/v1.0.0",
    Files:     []string{"README.md", "pennsieve.json"},
}, dest)
```

## Package Structure

| Package | Description |
|---|---|
| `pkg/github` | Core GitHub API client and data models |
| `pkg/github/sync` | Content synchronization engine with S3 destination support |
| `internal/cache` | TTL-based in-memory cache (used internally for token caching) |

## Dependencies

| Dependency | Purpose |
|---|---|
| `github.com/golang-jwt/jwt/v5` | JWT signing for GitHub App authentication |
| `github.com/aws/aws-sdk-go-v2/service/s3` | S3 storage for content sync |
| `github.com/stretchr/testify` | Testing |
