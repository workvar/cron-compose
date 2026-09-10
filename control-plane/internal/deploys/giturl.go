package deploys

import (
	"net/url"
	"strings"
)

// AuthenticatedCloneURL injects an OAuth token into a https git URL.
func AuthenticatedCloneURL(raw, provider, token string) string {
	if token == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return raw
	}
	user := "x-access-token"
	if provider == "gitlab" {
		user = "oauth2"
	}
	u.User = url.UserPassword(user, token)
	return u.String()
}

// PublicCloneURL strips userinfo so a URL is safe to log or show.
func PublicCloneURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.User = nil
	return u.String()
}

func httpsCloneURL(provider, fullName string) string {
	fullName = strings.Trim(fullName, "/")
	switch provider {
	case "gitlab":
		return "https://gitlab.com/" + fullName + ".git"
	default:
		return "https://github.com/" + fullName + ".git"
	}
}
