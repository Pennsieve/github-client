package github

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

const TestPennsieveGitHubAppId = 1000000001

func TestGithubAPI(t *testing.T) {

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			w.WriteHeader(http.StatusOK)

			p := GithubProfile{
				AccessToken:    "1234",
				InstallationId: "1",
				Login:          "testUserName",
				Url:            "https://github.com/testuser",
				AvatarUrl:      "https://avatars2.githubusercontent.com/u/1234",
			}
			pJson, _ := json.Marshal(p)

			w.Write(pJson)
			return
		}
		if r.URL.Path == "/app/installations/*" {
			w.WriteHeader(http.StatusOK)

			w.Write([]byte("success"))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer mockServer.Close()

	testClient := NewGitHubApiClient(
		slog.Default(),
		"testClientId",
		"testClientSecret",
		mockServer.URL,
		TestPennsieveGitHubAppId)

	for scenario, fn := range map[string]func(
		tt *testing.T, c GitHubApi,
	){
		"get user profile from GitHub": testGetGithubProfile,
	} {
		t.Run(scenario, func(t *testing.T) {
			fn(t, testClient)
		})
	}
}

func TestGetFileContent(t *testing.T) {
	fileContent := "package main\n\nfunc main() {}\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(fileContent))

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/contents/main.go":
			resp := GitHubContentResponse{
				Name:     "main.go",
				Path:     "main.go",
				Content:  encoded,
				Encoding: "base64",
			}
			json.NewEncoder(w).Encode(resp)
		case "/repos/owner/repo/contents/missing.go":
			w.WriteHeader(http.StatusNotFound)
		case "/repos/owner/repo/contents/bad-encoding.go":
			resp := GitHubContentResponse{
				Name:     "bad-encoding.go",
				Content:  "not-valid-base64!!!",
				Encoding: "base64",
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer mockServer.Close()

	client := NewGitHubApiClient(
		slog.Default(),
		"testClientId",
		"testClientSecret",
		mockServer.URL,
		TestPennsieveGitHubAppId,
	).WithAccessToken("test-token")

	t.Run("returns decoded file content", func(t *testing.T) {
		content, err := client.GetFileContent("https://github.com/owner/repo", "main.go", "v1.0.0")
		assert.NoError(t, err)
		assert.Equal(t, []byte(fileContent), content)
	})

	t.Run("returns nil for not found", func(t *testing.T) {
		content, err := client.GetFileContent("https://github.com/owner/repo", "missing.go", "v1.0.0")
		assert.NoError(t, err)
		assert.Nil(t, content)
	})

	t.Run("returns error for invalid base64", func(t *testing.T) {
		content, err := client.GetFileContent("https://github.com/owner/repo", "bad-encoding.go", "v1.0.0")
		assert.Error(t, err)
		assert.Nil(t, content)
		assert.Contains(t, err.Error(), "failed to decode base64 content")
	})
}

func testGetGithubProfile(t *testing.T, c GitHubApi) {

	testUserProfile, err := c.GetUserProfile()

	assert.NoError(t, err)

	p := GithubProfile{
		AccessToken:    "1234",
		InstallationId: "1",
		Login:          "testUserName",
		Url:            "https://github.com/testuser",
		AvatarUrl:      "https://avatars2.githubusercontent.com/u/1234",
	}
	assert.Equal(t, testUserProfile, &p)
}
