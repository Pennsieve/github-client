package github

import (
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
