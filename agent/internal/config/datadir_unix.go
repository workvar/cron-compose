//go:build !darwin

package config

// defaultDataDir is the FHS location for variable state owned by a system service.
// Every Linux install path (deb, apk, install-agent.sh) creates this directory owned
// by the croncompose service user.
const defaultDataDir = "/var/lib/croncompose"
