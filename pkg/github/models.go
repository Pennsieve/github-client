package github

import (
	"encoding/json"
	"fmt"
	"time"
)

const GitHub = "GitHub"

type GithubProfile struct {
	AccessToken    string `json:"access_token"`
	InstallationId string `json:"installation_id"`
	Login          string `json:"login"`
	Url            string `json:"url"`
	HTMLUrl        string `json:"html_url"`
	AvatarUrl      string `json:"avatar_url"`
}

// Scan converts the data returned from the DB into the struct.
func (u *GithubProfile) Scan(v interface{}) error {
	switch vv := v.(type) {
	case []byte:
		return json.Unmarshal(vv, u)
	case string:
		return json.Unmarshal([]byte(vv), u)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
}

type GitHubRepo struct {
	Id          int             `json:"id"`
	NodeId      string          `json:"node_id"`
	Name        string          `json:"name"`
	FullName    string          `json:"full_name"`
	Description string          `json:"description,omitempty"`
	HtmlUrl     string          `json:"html_url,omitempty"`
	License     GitHubLicense   `json:"license,omitempty"`
	Private     bool            `json:"private,omitempty"`
	Owner       GitHubRepoOwner `json:"owner,omitempty"`
}

type GitHubRepoOwner struct {
	Login             string `json:"login"`
	Id                int    `json:"id"`
	NodeId            string `json:"node_id"`
	AvatarUrl         string `json:"avatar_url"`
	GravatarId        string `json:"gravatar_id"`
	Url               string `json:"url"`
	HtmlUrl           string `json:"html_url"`
	FollowersUrl      string `json:"followers_url,omitempty"`
	FollowingUrl      string `json:"following_url,omitempty"`
	GistsUrl          string `json:"gists_url,omitempty"`
	StarredUrl        string `json:"starred_url,omitempty"`
	SubscriptionsUrl  string `json:"subscriptions_url,omitempty"`
	OrganizationsUrl  string `json:"organizations_url,omitempty"`
	ReposUrl          string `json:"repos_url,omitempty"`
	EventsUrl         string `json:"events_url,omitempty"`
	ReceivedEventsUrl string `json:"received_events_url,omitempty"`
	Type              string `json:"type"`
	UserViewType      string `json:"user_view_type,omitempty"`
	SiteAdmin         bool   `json:"site_admin"`
}

type GitHubLicense struct {
	Key     string `json:"key,omitempty"`
	Name    string `json:"name,omitempty"`
	SpdxId  string `json:"spdx_id,omitempty"`
	Url     string `json:"url,omitempty"`
	HtmlUrl string `json:"html_url,omitempty"`
	NodeId  string `json:"node_id,omitempty"`
}

type GitHubContributor struct {
	Login         string `json:"login"`
	Id            int    `json:"id"`
	NodeId        string `json:"node_id"`
	Url           string `json:"url"`
	HtmlUrl       string `json:"html_url"`
	Type          string `json:"type"`
	Contributions int    `json:"contributions"`
}

type GitHubUser struct {
	Login     string `json:"login"`
	Id        int    `json:"id"`
	NodeId    string `json:"node_id"`
	AvatarUrl string `json:"avatar_url"`
	Url       string `json:"url"`
	HtmlUrl   string `json:"html_url"`
	Type      string `json:"type"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
	Bio       string `json:"bio,omitempty"`
}

type GitHubRepoContributor struct {
	Login         string `json:"login"`
	Id            int    `json:"id"`
	NodeId        string `json:"node_id"`
	Url           string `json:"url"`
	HtmlUrl       string `json:"html_url"`
	Type          string `json:"type"`
	Contributions int    `json:"contributions"`
	Name          string `json:"name"`
	Email         string `json:"email,omitempty"`
}

func (u *GitHubUser) String() string {
	json, err := json.Marshal(u)
	if err != nil {
		return err.Error()
	}
	return string(json)
}

type GitHubRelease struct {
	Url             string     `json:"url"`
	AssetsUrl       string     `json:"assets_url"`
	UploadUrl       string     `json:"upload_url"`
	HtmlUrl         string     `json:"html_url"`
	Id              int        `json:"id"`
	NodeId          string     `json:"node_id"`
	TagName         string     `json:"tag_name"`
	TargetCommitish string     `json:"target_commitish"`
	Name            string     `json:"name"`
	Draft           bool       `json:"draft"`
	Prerelease      bool       `json:"prerelease"`
	CreatedAt       time.Time  `json:"created_at"`
	PublishedAt     time.Time  `json:"published_at"`
	Author          GitHubUser `json:"author"`
	ZipballUrl      string     `json:"zipball_url"`
	Body            string     `json:"body"`
}

type GitHubTagObject struct {
	Sha  string `json:"sha"`
	Type string `json:"type"`
	Url  string `json:"url"`
}

type GitHubTag struct {
	Ref    string          `json:"ref"`
	NodeId string          `json:"node_id"`
	Url    string          `json:"url"`
	Object GitHubTagObject `json:"object"`
}

type GitHubTagDetails struct {
	NodeId string `json:"node_id"`
	Sha    string `json:"sha"`
	Url    string `json:"url"`
	Tagger struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Date  string `json:"date"`
	} `json:"tagger"`
	Object       GitHubTagObject `json:"object"`
	Tag          string          `json:"tag"`
	Message      string          `json:"message,omitempty"`
	Verification struct {
		Verified  bool   `json:"verified,omitempty"`
		Reason    string `json:"reason,omitempty"`
		Signature string `json:"signature,omitempty"`
		Payload   string `json:"payload,omitempty"`
	}
}

type GitHubReadme struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Sha         string `json:"sha"`
	Size        int64  `json:"size"`
	Url         string `json:"url"`
	HtmlUrl     string `json:"html_url"`
	GitUrl      string `json:"git_url"`
	DownloadUrl string `json:"download_url"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Encoding    string `json:"encoding"`
}

type GitHubLicenseResponse struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Sha         string `json:"sha"`
	Size        int64  `json:"size"`
	Url         string `json:"url"`
	HtmlUrl     string `json:"html_url"`
	GitUrl      string `json:"git_url"`
	DownloadUrl string `json:"download_url"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Encoding    string `json:"encoding"`
	Links       struct {
		Self string `json:"self,omitempty"`
		Git  string `json:"git,omitempty"`
		Html string `json:"html,omitempty"`
	} `json:"_links,omitempty"`
	License GitHubLicense `json:"license,omitempty"`
}

type GitHubContentResponse struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Sha         string `json:"sha"`
	Size        int64  `json:"size"`
	Url         string `json:"url"`
	HtmlUrl     string `json:"html_url"`
	GitUrl      string `json:"git_url"`
	DownloadUrl string `json:"download_url"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Encoding    string `json:"encoding"`
	Links       struct {
		Self string `json:"self,omitempty"`
		Git  string `json:"git,omitempty"`
		Html string `json:"html,omitempty"`
	}
}

type GitHubReleaseAssetFile struct {
	File string `json:"file"`
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

type GitHubReleaseAsset struct {
	RepoUrl      string                   `json:"repo_url"`
	ReleaseTag   string                   `json:"release_tag"`
	AssetName    string                   `json:"asset_name"`
	AssetData    []byte                   `json:"asset_data"`
	AssetSize    int64                    `json:"asset_size"`
	AssetListing []GitHubReleaseAssetFile `json:"asset_listing"`
}

type ReleaseAssetListingOutput struct {
	AssetListing []GitHubReleaseAssetFile `json:"files"`
}

func (a *GitHubReleaseAsset) String() string {
	return fmt.Sprintf("GitHubReleaseAsset(RepoUrl: %s, ReleaseTag: %s, AssetName: %s, AssetSize: %d, AssetListing(count): %d)", a.RepoUrl, a.ReleaseTag, a.AssetName, a.AssetSize, len(a.AssetListing))
}

type GitHubAccount struct {
	Login             string `json:"login"`
	Id                int    `json:"id"`
	NodeId            string `json:"node_id,omitempty"`
	AvatarUrl         string `json:"avatar_url,omitempty"`
	GravatarId        string `json:"gravatar_id,omitempty"`
	Url               string `json:"url,omitempty"`
	HtmlUrl           string `json:"html_url,omitempty"`
	FollowersUrl      string `json:"followers_url,omitempty"`
	FollowingUrl      string `json:"following_url,omitempty"`
	GistsUrl          string `json:"gists_url,omitempty"`
	StarredUrl        string `json:"starred_url,omitempty"`
	SubscriptionsUrl  string `json:"subscriptions_url,omitempty"`
	OrganizationsUrl  string `json:"organizations_url,omitempty"`
	ReposUrl          string `json:"repos_url,omitempty"`
	EventsUrl         string `json:"events_url,omitempty"`
	ReceivedEventsUrl string `json:"received_events_url,omitempty"`
	Type              string `json:"type,omitempty"`
	UserViewType      string `json:"user_view_type,omitempty"`
	SiteAdmin         bool   `json:"site_admin,omitempty"`
}

type GitHubPermissions struct {
	Administration string `json:"administration"`
	Contents       string `json:"contents"`
	Metadata       string `json:"metadata"`
}

type GitHubInstallation struct {
	Id                     int               `json:"id"`
	ClientId               string            `json:"client_id"`
	Account                GitHubAccount     `json:"account"`
	RepositorySelection    string            `json:"repository_selection"`
	AccessTokensUrl        string            `json:"access_tokens_url"`
	RepositoriesUrl        string            `json:"repositories_url"`
	HtmlUrl                string            `json:"html_url"`
	AppId                  int               `json:"app_id"`
	AppSlug                string            `json:"app_slug"`
	TargetId               int               `json:"target_id"`
	TargetType             string            `json:"target_type"`
	Permissions            GitHubPermissions `json:"permissions"`
	Events                 []string          `json:"events"`
	CreatedAt              time.Time         `json:"created_at"`
	UpdatedAt              time.Time         `json:"updated_at"`
	SingleFileName         string            `json:"single_file_name"`
	HasMultipleSingleFiles bool              `json:"has_multiple_single_files"`
	SingleFilePaths        []string          `json:"single_file_paths"`
	SuspendedBy            GitHubAccount     `json:"suspended_by"`
	SuspendedAt            time.Time         `json:"suspended_at"`
}

type GitHubInstallationList struct {
	TotalCount    int                  `json:"total_count"`
	Installations []GitHubInstallation `json:"installations"`
}
