package github

import (
	"archive/zip"
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pennsieve/github-client/internal/cache"
)

const handlerName = "handler"
const functionName = "function"

var userInstallationIdListCache = cache.New(cache.DefaultTTL)

type GitHubApi interface {
	DeleteInstallation(installationId string) error
	GetAvatar(url string) (*Avatar, error)
	GetChangeLog(repoUrl string, releaseTag string) (*GitHubChangeLog, error)
	GetContent(url string, filePath string, tag string) (*GitHubContentResponse, error)
	GetFileContent(url string, filePath string, tag string) ([]byte, error)
	GetContributors(url string, tag string) ([]GitHubContributor, error)
	GetLicense(url string) (*GitHubLicenseResponse, error)
	GetReadme(url string, tag string) (*GitHubReadme, error)
	GetRelease(url string, tag string) (*GitHubRelease, error)
	GetReleaseAsset(url string, tag string) (*GitHubReleaseAsset, error)
	GetRepo(url string) (*GitHubRepo, error)
	GetRepoOwner(url string) (*GitHubUser, error)
	GetRepoContributors(url string, tag string) ([]GitHubRepoContributor, error)
	GetTag(url string, tag string) (*GitHubTag, error)
	GetTagDetails(tag *GitHubTag) (*GitHubTagDetails, error)
	GetUser(username string) (*GitHubUser, error)
	GetUserProfile() (*GithubProfile, error)
	ListAppInstallationsAccessibleToTheUserAccessToken() (*GitHubInstallationList, error)
	GetAccessibleInstallations(username string) ([]int, error)
	WithAccessToken(accessToken string) GitHubApi
	WithAppPrivateKey(key string) GitHubApi
	WithInstallationId(id int) GitHubApi
	WithPennsieveAppId(id int) GitHubApi
}

type GitHubApiClient struct {
	httpClient              *http.Client
	apiUrl                  string
	clientAppId             string
	clientAppSecret         string
	appPrivateKey           string
	privateKey              *rsa.PrivateKey
	accessToken             string
	installationId          int
	logger                  *slog.Logger
	pennsieveAppId          int
	installationAccessToken *InstallationAccessToken
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

const GitHubApiUrl = "https://api.github.com"

type GitHubUrlParts int

const (
	Protocol GitHubUrlParts = iota
	_
	Domain
	Owner
	Repository
)

func NewGitHubApiClient(
	logger *slog.Logger,
	clientAppId string,
	clientAppSecret string,
	url string,
	pennsieveAppId int,
) GitHubApi {

	var netClient = &http.Client{
		Timeout: time.Second * 10,
	}

	return &GitHubApiClient{
		httpClient:      netClient,
		apiUrl:          url,
		clientAppId:     clientAppId,
		clientAppSecret: clientAppSecret,
		logger:          adaptedLogger(logger),
		pennsieveAppId:  pennsieveAppId,
	}
}

func (s *GitHubApiClient) WithAccessToken(accessToken string) GitHubApi {
	s.accessToken = accessToken
	return s
}

func (s *GitHubApiClient) WithAppPrivateKey(key string) GitHubApi {
	s.appPrivateKey = key
	return s
}

func (s *GitHubApiClient) WithInstallationId(id int) GitHubApi {
	s.installationId = id
	return s
}

func (s *GitHubApiClient) WithPennsieveAppId(id int) GitHubApi {
	s.pennsieveAppId = id
	return s
}

func adaptedLogger(logger *slog.Logger) *slog.Logger {
	return logger.With(
		slog.String(handlerName, "GitHubApiClient"),
	)
}

func base64Decode(str string) (string, bool) {
	data, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func DecodePrivateKey(privateKeyStr string) (*rsa.PrivateKey, error) {
	var err error

	block, _ := pem.Decode([]byte(privateKeyStr))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the key")
	}

	var privateKeyBytes []byte
	if x509.IsEncryptedPEMBlock(block) {
		return nil, errors.New("encrypted private keys are not handled")
	} else {
		privateKeyBytes = block.Bytes
	}

	var parsedKey interface{}
	parsedKey, err = x509.ParsePKCS1PrivateKey(privateKeyBytes)
	if err != nil {
		parsedKey, err = x509.ParsePKCS8PrivateKey(privateKeyBytes)
		if err != nil {
			return nil, err
		}
	}

	privateKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("parsed key is not an RSA private key")
	}

	return privateKey, nil
}

func (s *GitHubApiClient) getPrivateKey() (*rsa.PrivateKey, error) {
	if s.privateKey == nil {
		if s.appPrivateKey != "" {
			privateKey, err := DecodePrivateKey(s.appPrivateKey)
			if err != nil {
				return nil, err
			}
			s.privateKey = privateKey
		} else {
			return nil, errors.New("no app private key provided")
		}
	}

	return s.privateKey, nil
}

func (s *GitHubApiClient) getAccessToken() (string, error) {
	logger := s.logger.With(
		functionName, "getAccessToken",
	)

	if s.accessToken != "" {
		logger.Debug(fmt.Sprintf("s.accessToken: %s", s.accessToken))
		return s.accessToken, nil
	}

	installationAccessToken, err := s.getInstallationAccessToken()
	if err != nil {
		return "", err
	}

	logger.Debug(fmt.Sprintf("installationAccessToken: %+v", installationAccessToken))
	return installationAccessToken.Token, nil
}

func (s *GitHubApiClient) hasAuth() bool {
	return s.accessToken != "" || s.appPrivateKey != ""
}

func (s *GitHubApiClient) GetUserProfile() (*GithubProfile, error) {
	logger := s.logger.With(
		functionName, "GetUserProfile",
	)

	const ghUSerUrl = "/user"

	req, _ := http.NewRequest("GET", s.apiUrl+ghUSerUrl, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+s.accessToken)
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	defer resp.Body.Close()

	if err != nil {
		message := "Error: Unable to user from GitHub: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	var profile GithubProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		message := "Error: Unable to parse access token response: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	return &profile, nil
}

func (s *GitHubApiClient) DeleteInstallation(installationId string) error {
	logger := s.logger.With(
		slog.String(handlerName, "GitHubApiClient"),
		slog.String(functionName, "DeleteInstallation"),
	)

	ghDeleteInstallationUrl := fmt.Sprintf("/app/installations/%s", installationId)

	logger.Info(fmt.Sprintf("Deleting GitHub Installation: %s", installationId))

	token := jwt.NewWithClaims(jwt.SigningMethodRS256,
		jwt.MapClaims{
			"iss": s.clientAppId,
			"iat": time.Now().Unix(),
			"exp": time.Now().Add(time.Second * 60).Unix(),
		})

	privateKey, err := s.getPrivateKey()
	if err != nil {
		message := "Error: Unable to get private key: " + err.Error()
		logger.Error(message)
		return err
	}

	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		message := "Error: Unable to create signed token: " + fmt.Sprint(err)
		logger.Error(message)
		return err
	}

	req, _ := http.NewRequest("DELETE", s.apiUrl+ghDeleteInstallationUrl, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+signedToken)
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	defer resp.Body.Close()

	if err != nil {
		message := "Error: Unable to delete App Installation from GitHub: " + fmt.Sprint(err)
		logger.Error(message)
		return err
	}

	return nil
}

func (s *GitHubApiClient) GetRepo(url string) (*GitHubRepo, error) {
	logger := s.logger.With(
		slog.String(functionName, "GetRepo"),
	)

	var urlParts = strings.Split(url, "/")

	requestUrl := fmt.Sprintf("%s/repos/%s/%s",
		s.apiUrl,
		urlParts[Owner],
		urlParts[Repository])

	accessToken, err := s.getAccessToken()
	if err != nil {
		message := "Error: Unable to get access token: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	req, _ := http.NewRequest("GET", requestUrl, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	defer resp.Body.Close()

	if err != nil {
		message := "Error: Unable to get GitHub repo: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	var repo GitHubRepo
	if err = json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		message := "Error: Unable to parse user permission response: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	return &repo, nil
}

func (s *GitHubApiClient) GetRepoOwner(url string) (*GitHubUser, error) {
	logger := s.logger.With(
		slog.String(functionName, "GetRepoOwner"),
	)

	accessToken, err := s.getAccessToken()
	if err != nil {
		message := "Error: Unable to get access token: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	defer resp.Body.Close()

	if err != nil {
		message := "Error: Unable to get GitHub repo owner: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	var ghUser GitHubUser
	if err = json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		message := "Error: Unable to parse repo owner (user) response: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	return &ghUser, nil
}

func (s *GitHubApiClient) GetRelease(url string, tag string) (*GitHubRelease, error) {
	logger := s.logger.With(
		functionName, "GetRelease",
	)

	accessToken, err := s.getAccessToken()
	if err != nil {
		message := "Error: Unable to get access token: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	var urlParts = strings.Split(url, "/")

	requestUrl := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s",
		s.apiUrl,
		urlParts[Owner],
		urlParts[Repository],
		tag)

	req, _ := http.NewRequest("GET", requestUrl, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	defer resp.Body.Close()

	if err != nil {
		message := "Error: Unable to get GitHub release: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	var release GitHubRelease
	if err = json.NewDecoder(resp.Body).Decode(&release); err != nil {
		message := "Error: Unable to parse GitHub release response: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	return &release, nil
}

func (s *GitHubApiClient) GetTag(url string, tag string) (*GitHubTag, error) {
	logger := s.logger.With(
		functionName, "GetTag",
	)

	var urlParts = strings.Split(url, "/")

	requestUrl := fmt.Sprintf("%s/repos/%s/%s/git/ref/tags/%s",
		s.apiUrl,
		urlParts[Owner],
		urlParts[Repository],
		tag)

	accessToken, err := s.getAccessToken()
	if err != nil {
		message := "Error: Unable to get access token: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	req, _ := http.NewRequest("GET", requestUrl, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	defer resp.Body.Close()

	if err != nil {
		message := "Error: Unable to get GitHub tag: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	var tagInfo GitHubTag
	if err = json.NewDecoder(resp.Body).Decode(&tagInfo); err != nil {
		message := "Error: Unable to parse GitHub tag response: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	return &tagInfo, nil
}

func (s *GitHubApiClient) GetTagDetails(tag *GitHubTag) (*GitHubTagDetails, error) {
	logger := s.logger.With(
		functionName, "GetTagDetails",
	)

	if tag.Object.Url == "" {
		message := "error: GitHub tag does not have object apiUrl"
		logger.Error(message)
		return nil, errors.New(message)
	}

	accessToken, err := s.getAccessToken()
	if err != nil {
		message := "Error: Unable to get access token: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	req, _ := http.NewRequest("GET", tag.Object.Url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	defer resp.Body.Close()
	if err != nil {
		message := fmt.Sprintf("error: Unable to get GitHub tag details: %+v", err)
		logger.Error(message)
		return nil, errors.New(message)
	}

	var tagDetails GitHubTagDetails
	if err = json.NewDecoder(resp.Body).Decode(&tagDetails); err != nil {
		message := "error: Unable to parse GitHub tag details response: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, errors.New(message)
	}

	return &tagDetails, nil
}

func (s *GitHubApiClient) GetReadme(url string, tag string) (*GitHubReadme, error) {
	logger := s.logger.With(
		functionName, "GetReadme",
	)

	var urlParts = strings.Split(url, "/")

	requestUrl := fmt.Sprintf("%s/repos/%s/%s/readme?ref=%s",
		s.apiUrl,
		urlParts[Owner],
		urlParts[Repository],
		tag)

	accessToken, err := s.getAccessToken()
	if err != nil {
		message := "Error: Unable to get access token: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	req, _ := http.NewRequest("GET", requestUrl, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	defer resp.Body.Close()

	if err != nil {
		message := "Error: Unable to get GitHub Readme: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	var readme GitHubReadme
	if err = json.NewDecoder(resp.Body).Decode(&readme); err != nil {
		message := "Error: Unable to parse GitHub Readme response: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	return &readme, nil
}

func (s *GitHubApiClient) GetLicense(url string) (*GitHubLicenseResponse, error) {
	logger := s.logger.With(
		slog.String(functionName, "GetLicense"),
	)

	accessToken, err := s.getAccessToken()
	if err != nil {
		message := "Error: Unable to get access token: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	var urlParts = strings.Split(url, "/")

	requestUrl := fmt.Sprintf("%s/repos/%s/%s/license",
		s.apiUrl,
		urlParts[Owner],
		urlParts[Repository])

	req, _ := http.NewRequest("GET", requestUrl, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)

	if err != nil {
		message := "Error: Unable to get GitHub License: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	defer resp.Body.Close()
	var license GitHubLicenseResponse
	if err = json.NewDecoder(resp.Body).Decode(&license); err != nil {
		message := "Error: Unable to parse GitHub License response: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	return &license, nil
}

func (s *GitHubApiClient) GetContent(url string, filePath string, tag string) (*GitHubContentResponse, error) {
	logger := s.logger.With(
		functionName, "GetContent",
	)

	var urlParts = strings.Split(url, "/")

	requestUrl := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
		s.apiUrl,
		urlParts[Owner],
		urlParts[Repository],
		filePath,
		tag)

	req, _ := http.NewRequest("GET", requestUrl, nil)
	req.Header.Set("Accept", "application/vnd.github+json")

	if s.hasAuth() {
		accessToken, err := s.getAccessToken()
		if err != nil {
			message := "Error: Unable to get access token: " + fmt.Sprint(err)
			logger.Error(message)
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	defer resp.Body.Close()
	var content GitHubContentResponse
	if err = json.NewDecoder(resp.Body).Decode(&content); err != nil {
		message := "Error: Unable to parse GitHub Content response: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	return &content, nil
}

func (s *GitHubApiClient) GetFileContent(url string, filePath string, tag string) ([]byte, error) {
	resp, err := s.GetContent(url, filePath, tag)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	decoded, ok := base64Decode(resp.Content)
	if !ok {
		return nil, fmt.Errorf("failed to decode base64 content for %s", filePath)
	}
	return []byte(decoded), nil
}

func (s *GitHubApiClient) GetUser(username string) (*GitHubUser, error) {
	logger := s.logger.With(
		slog.String(functionName, "GetUser"),
	)

	accessToken, err := s.getAccessToken()
	if err != nil {
		message := "Error: Unable to get access token: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	requestUrl := fmt.Sprintf("%s/users/%s",
		s.apiUrl,
		username)

	req, _ := http.NewRequest("GET", requestUrl, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		message := "Error: Unable to get GitHub User: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}
	defer resp.Body.Close()

	var user GitHubUser
	if err = json.NewDecoder(resp.Body).Decode(&user); err != nil {
		message := "Error: Unable to parse GitHub User response: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	return &user, nil
}

func (s *GitHubApiClient) GetContributors(url string, tag string) ([]GitHubContributor, error) {
	logger := s.logger.With(
		slog.String(functionName, "GetContributors"),
	)

	accessToken, err := s.getAccessToken()
	if err != nil {
		message := "Error: Unable to get access token: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	var urlParts = strings.Split(url, "/")

	requestUrl := fmt.Sprintf("%s/repos/%s/%s/contributors?ref=%s",
		s.apiUrl,
		urlParts[Owner],
		urlParts[Repository],
		tag)

	req, _ := http.NewRequest("GET", requestUrl, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	defer resp.Body.Close()
	var contributors []GitHubContributor
	if err = json.NewDecoder(resp.Body).Decode(&contributors); err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	return contributors, nil
}

func (s *GitHubApiClient) GetRepoContributors(url string, tag string) ([]GitHubRepoContributor, error) {
	logger := s.logger.With(
		slog.String(functionName, "GetRepoContributors"),
	)

	contributors, err := s.GetContributors(url, tag)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	var repoContributors []GitHubRepoContributor
	for _, contributor := range contributors {
		user, err := s.GetUser(contributor.Login)
		if err != nil {
			logger.Error(err.Error())
			return nil, err
		}
		repoContributors = append(repoContributors, GitHubRepoContributor{
			Login:         contributor.Login,
			Id:            contributor.Id,
			NodeId:        contributor.NodeId,
			Url:           user.Url,
			HtmlUrl:       user.HtmlUrl,
			Type:          user.Type,
			Contributions: contributor.Contributions,
			Name:          user.Name,
			Email:         user.Email,
		})
	}
	return repoContributors, nil
}

func (s *GitHubApiClient) GetReleaseAsset(url string, tag string) (*GitHubReleaseAsset, error) {
	logger := s.logger.With(
		slog.String(functionName, "GetReleaseAsset"),
	)

	var urlParts = strings.Split(url, "/")
	requestUrl := fmt.Sprintf("%s/repos/%s/%s/zipball/%s",
		s.apiUrl,
		urlParts[Owner],
		urlParts[Repository],
		tag,
	)

	accessToken, err := s.getAccessToken()
	if err != nil {
		message := "Error: Unable to get access token: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	req, _ := http.NewRequest("GET", requestUrl, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	assetName := "release.zip"
	contentDisposition := resp.Header.Get("Content-Disposition")
	if contentDisposition != "" {
		assetNameParts := strings.Split(contentDisposition, "=")
		if len(assetNameParts) == 2 {
			assetName = assetNameParts[1]
		}
	}

	var buffer bytes.Buffer
	buf := make([]byte, 32*1024)
	var downloaded int64
	count := 0
	for {
		count += 1
		n, err := resp.Body.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			logger.Error(fmt.Sprintf("GetReleaseAsset() error reading GitHub Release asset buffer: %s", err))
			return nil, err
		}
		if n > 0 {
			buffer.Write(buf[:n])
			downloaded += int64(n)
		}
	}
	bufferSize := int64(buffer.Len())
	logger.Info(fmt.Sprintf("GetReleaseAsset() downloaded %d bytes (buffer size: %d)", downloaded, bufferSize))

	var assetFileList []GitHubReleaseAssetFile
	bufferReader := bytes.NewReader(buffer.Bytes())
	zipReader, err := zip.NewReader(bufferReader, bufferSize)
	if err != nil {
		logger.Warn(fmt.Sprintf("GetReleaseAsset() unable to create zip reader: %s", err))
	} else {
		var fileListingPrefix string
		var filePath string
		var fileName string
		var fileType string
		for _, file := range zipReader.File {
			if fileListingPrefix == "" {
				fileListingPrefix = file.Name
				continue
			}
			filePath = strings.Replace(file.Name, fileListingPrefix, "", -1)
			if file.Name[len(file.Name)-1:] == "/" {
				fileType = "folder"
			} else {
				fileType = "file"
			}
			if len(filePath) > 0 {
				fileName = filepath.Base(filePath)
				assetFileList = append(assetFileList, GitHubReleaseAssetFile{
					File: filePath,
					Name: fileName,
					Type: fileType,
					Size: int64(file.UncompressedSize64),
				})
			}
		}
		logger.Info(fmt.Sprintf("GetReleaseAsset() assetFileList: %d files/folders", len(assetFileList)))
	}

	return &GitHubReleaseAsset{
		RepoUrl:      url,
		ReleaseTag:   tag,
		AssetName:    assetName,
		AssetData:    buffer.Bytes(),
		AssetSize:    bufferSize,
		AssetListing: assetFileList,
	}, nil
}

// ListAppInstallationsAccessibleToTheUserAccessToken
// see: https://docs.github.com/en/rest/apps/installations?apiVersion=2022-11-28#list-app-installations-accessible-to-the-user-access-token
func (s *GitHubApiClient) ListAppInstallationsAccessibleToTheUserAccessToken() (*GitHubInstallationList, error) {
	logger := s.logger.With(
		slog.String(functionName, "ListAppInstallationsAccessibleToTheUserAccessToken"),
	)

	requestUrl := fmt.Sprintf("%s/user/installations", s.apiUrl)

	req, _ := http.NewRequest("GET", requestUrl, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+s.accessToken)
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		message := "ListAppInstallationsAccessibleToTheUserAccessToken() Http Client error: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}
	defer resp.Body.Close()

	installationResponseList := GitHubInstallationList{}
	if err = json.NewDecoder(resp.Body).Decode(&installationResponseList); err != nil {
		message := "ListAppInstallationsAccessibleToTheUserAccessToken() JSON decoding error: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	installationList := GitHubInstallationList{}
	for _, installation := range installationResponseList.Installations {
		if installation.AppId == s.pennsieveAppId {
			installationList.Installations = append(installationList.Installations, installation)
		}
	}
	installationList.TotalCount = len(installationList.Installations)

	return &installationList, nil
}

func (s *GitHubApiClient) GetAccessibleInstallations(username string) ([]int, error) {
	installationIdList := make([]int, 0)

	val, ok := userInstallationIdListCache.Get(username)
	if ok {
		installationIdList = val.([]int)
		return installationIdList, nil
	}

	installationList, err := s.ListAppInstallationsAccessibleToTheUserAccessToken()
	if err != nil {
		return nil, err
	}

	if installationList.TotalCount > 0 || len(installationList.Installations) > 0 {
		for _, installation := range installationList.Installations {
			installationIdList = append(installationIdList, installation.Id)
		}
		userInstallationIdListCache.Add(username, installationIdList)
	}

	return installationIdList, nil
}

type GitHubInstallationAccessToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *GitHubApiClient) getInstallationAccessToken() (*InstallationAccessToken, error) {
	logger := s.logger.With(
		slog.String(functionName, "getInstallationAccessToken"),
		slog.Int("installation_id", s.installationId),
	)

	if s.installationAccessToken != nil && !s.installationAccessToken.Expired() {
		logger.Debug(fmt.Sprintf("using non-expired installation access token: %+v", s.installationAccessToken))
		return s.installationAccessToken, nil
	}
	logger.Debug(fmt.Sprintf("generating new installation access token"))

	privateKey, err := s.getPrivateKey()
	if err != nil {
		message := fmt.Sprintf("error getting private key: %s", err)
		logger.Error(message)
		return nil, err
	}

	issuedAt := time.Now()
	expiresAt := issuedAt.Add(10 * time.Minute)
	claims := jwt.MapClaims{
		"iat": issuedAt.Unix(),
		"exp": expiresAt.Unix(),
		"iss": s.clientAppId,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		message := fmt.Sprintf("error signing JWT: %s", err)
		logger.Error(message)
		return nil, err
	}

	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", s.apiUrl, s.installationId)
	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+signedToken)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		message := "Http Client error: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		message := fmt.Sprintf("Http Client unexpected response - expecting %d but received %d", http.StatusCreated, resp.StatusCode)
		logger.Error(message)
		return nil, errors.New(message)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		message := fmt.Sprintf("Error parsing response body: %s", err)
		logger.Error(message)
		return nil, err
	}

	s.installationAccessToken = &InstallationAccessToken{
		Token:      result.Token,
		Expiration: expiresAt,
	}

	return s.installationAccessToken, nil
}

func (s *GitHubApiClient) GetAvatar(url string) (*Avatar, error) {
	logger := s.logger.With(
		slog.String(functionName, "GetAvatar"),
	)

	accessToken, err := s.getAccessToken()
	if err != nil {
		message := "Error: Unable to get access token: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	logger.Debug(fmt.Sprintf("req: %+v", req))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		message := "Http Client error: " + fmt.Sprint(err)
		s.logger.Error(message)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		message := "Reading response body error: " + fmt.Sprint(err)
		logger.Error(message)
		return nil, err
	}
	bodyLength := int64(len(body))

	contentLengthString := resp.Header.Get("Content-Length")
	if contentLengthString == "" {
		message := "Content-Length header is empty or missing"
		logger.Error(message)
		return nil, errors.New(message)
	}

	contentLength, err := strconv.ParseInt(contentLengthString, 10, 64)
	if err != nil {
		message := fmt.Sprintf("Content-Length header is invalid: %s", contentLengthString)
		logger.Error(message)
		return nil, err
	}

	if contentLength != bodyLength {
		message := fmt.Sprintf("Content-Length header and body length disagree (contentLength : %d, bodyLength: %d)", contentLength, bodyLength)
		logger.Error(message)
		return nil, errors.New(message)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		message := "Content-Type header is empty"
		logger.Error(message)
		return nil, errors.New(message)
	}

	return &Avatar{
		Type: contentType,
		Data: body,
	}, nil
}

type GitHubChangeLog struct {
	Text    string `json:"text"`
	Content []byte `json:"content"`
}

func (s *GitHubApiClient) GetChangeLog(repoUrl string, releaseTag string) (*GitHubChangeLog, error) {
	logger := s.logger.With(
		slog.String(functionName, "GetChangeLog"),
	)

	var changelogText string
	var content []byte
	var ok bool

	var changeLogFileName string = "CHANGELOG.md"
	repoContent, err := s.GetContent(repoUrl, changeLogFileName, releaseTag)
	if err != nil {
		message := fmt.Sprintf("failed to get repo content (%s): %s", changeLogFileName, fmt.Sprint(err))
		logger.Error(message)
		return nil, err
	}

	if repoContent != nil {
		changelogText, ok = base64Decode(repoContent.Content)
		if !ok {
			message := "Base64Decode error: " + fmt.Sprint(err)
			logger.Error(message)
			return nil, err
		}
	} else {
		release, err := s.GetRelease(repoUrl, releaseTag)
		if err != nil {
			message := "failed to get release: " + fmt.Sprint(err)
			logger.Error(message)
			return nil, err
		}
		changelogText = fmt.Sprintf("# Release: %s\n\n%s\n", releaseTag, release.Body)
	}

	content = []byte(changelogText)

	return &GitHubChangeLog{
		Text:    changelogText,
		Content: content,
	}, nil
}
