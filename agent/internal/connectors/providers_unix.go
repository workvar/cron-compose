//go:build !windows

package connectors

// platformProviders adds OS-specific connectors. Unix has none beyond the defaults.
func platformProviders() []Provider { return nil }
