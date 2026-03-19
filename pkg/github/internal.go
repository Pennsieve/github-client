package github

import "time"

type Avatar struct {
	Type string `json:"type"`
	Data []byte `json:"data"`
}

type InstallationAccessToken struct {
	Token      string    `json:"token"`
	Expiration time.Time `json:"expiration"`
}

func (t *InstallationAccessToken) Expired() bool {
	return t.Expiration.IsZero() || t.Expiration.Before(time.Now())
}
