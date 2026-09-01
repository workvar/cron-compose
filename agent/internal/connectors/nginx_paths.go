package connectors

import (
	"os"
	"path/filepath"
)

// nginx does not live in one place. A distro package puts it under /etc/nginx; Homebrew
// puts it under its own prefix, which differs between Apple Silicon (/opt/homebrew) and
// Intel (/usr/local); a hand-built nginx often lands in /usr/local/nginx/conf.
//
// Hardcoding /etc/nginx meant discovery on a Mac reported an nginx with no config paths
// at all, and because `owns` confines every read and write to those paths, every config
// operation was then refused as "outside this connector's config paths". The prefix is
// resolved once per call from what exists on disk rather than from GOOS, so a Linux box
// with a Homebrew nginx is handled by the same code.
var nginxPrefixes = []string{
	"/etc/nginx",
	"/opt/homebrew/etc/nginx",
	"/usr/local/etc/nginx",
	"/usr/local/nginx/conf",
}

// nginxPrefix returns the first prefix that has an nginx.conf in it. An empty string
// means nginx is installed but its configuration is somewhere we do not recognise,
// which callers must report rather than paper over with a guess.
func nginxPrefix() string {
	for _, p := range nginxPrefixes {
		if _, err := os.Stat(filepath.Join(p, "nginx.conf")); err == nil {
			return p
		}
	}
	return ""
}

// nginxMainConfig is the file nginx reads first, for whichever prefix this host uses.
// A candidate for this path can be checked standalone with `nginx -t -c`; anything
// else is an include and only makes sense in the context of the tree, which is what
// ValidateLive is for.
func nginxMainConfig(prefix string) string {
	if prefix == "" {
		return ""
	}
	return filepath.Join(prefix, "nginx.conf")
}

// nginxConfigPaths lists the config locations that exist under a prefix. `servers` is
// Homebrew's equivalent of Debian's `sites-enabled`; both are included because the
// same agent binary may meet either.
func nginxConfigPaths(prefix string) []string {
	if prefix == "" {
		return nil
	}
	return existingPaths(
		filepath.Join(prefix, "nginx.conf"),
		filepath.Join(prefix, "conf.d"),
		filepath.Join(prefix, "sites-enabled"),
		filepath.Join(prefix, "sites-available"),
		filepath.Join(prefix, "servers"),
	)
}
