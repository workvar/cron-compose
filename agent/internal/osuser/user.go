package osuser

// User is one OS account this agent knows about, for offering a real list of "run
// as" targets instead of a free-text guess.
type User struct {
	Username string
	UID      uint32
	Home     string
	Shell    string
	// Available reports whether this agent could switch to this user right now
	// (Resolve(Username) would succeed): either the agent already is this user, or
	// the agent runs as root. False means the account exists but picking it will
	// fail until the agent on that server is (re)installed to run as root.
	Available bool
}
