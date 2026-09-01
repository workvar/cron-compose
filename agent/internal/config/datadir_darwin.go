package config

// defaultDataDir is where the agent keeps its store, identity, and certificates on
// macOS.
//
// /var/lib is not a directory macOS ships or expects anything to live in; the
// platform equivalent for machine-wide, non-user state written by a local daemon is
// under /usr/local/var. It is chosen over /Library/Application Support because the
// path has no spaces, which keeps launchd plists, sudoers entries, and shell
// one-liners free of quoting traps.
const defaultDataDir = "/usr/local/var/croncompose"
