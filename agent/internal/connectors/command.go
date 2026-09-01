package connectors

import "context"

// Command is one instruction from the control plane, in package-neutral form. The
// runtime translates the protobuf ConnectorCommand into this so the connectors
// package stays free of any wire dependency (same reason discoveredToProto lives in
// runtime rather than here).
type Command struct {
	Op           string // discover|status|list|read|validate|apply|lifecycle|rollback|ports
	Kind         string // nginx|systemd|docker|pm2|...
	Instance     string // provider instance discriminator; empty for singletons
	Ref          string // path | unit | container id | pm2 id
	Action       string // lifecycle: start|stop|restart|reload|enable|disable
	Content      []byte // apply/rollback: the bytes to write
	BaseChecksum string // apply: optimistic concurrency against the last read
	DryRun       bool
}

// Step is one stage of a multi-stage operation (backup, validate, write, reload,
// health check). Steps are reported even when the operation as a whole failed, since
// knowing which stage broke is the whole point.
type Step struct {
	Name     string
	OK       bool
	Output   string
	ExitCode int32
}

// Result statuses. These are the strings that reach the database and the UI.
const (
	StatusSucceeded    = "succeeded"
	StatusFailed       = "failed"
	StatusInvalid      = "invalid"      // the tool's own validator rejected the config
	StatusUnauthorized = "unauthorized" // the agent lacks the privilege to do this
	StatusUnsupported  = "unsupported"  // this provider does not implement the op
)

// Result is what the agent sends back for a Command.
type Result struct {
	Status   string
	Message  string
	Content  []byte // read: the file bytes
	Checksum string
	Payload  []byte // ports: JSON array of ListenPort
	Steps    []Step
}

// Failed reports whether the result is anything other than success.
func (r Result) Failed() bool { return r.Status != StatusSucceeded }

func ok(msg string, steps ...Step) Result {
	return Result{Status: StatusSucceeded, Message: msg, Steps: steps}
}

func fail(status, msg string, steps ...Step) Result {
	return Result{Status: status, Message: msg, Steps: steps}
}

// configBytesResult is the successful ReadConfig payload: bytes plus the checksum
// the editor will send back as base_checksum on apply.
func configBytesResult(path string, b []byte) Result {
	return Result{
		Status:   StatusSucceeded,
		Message:  "read " + path,
		Content:  b,
		Checksum: checksum(b),
	}
}

// Actor is implemented by providers that can drive object lifecycle (start a unit,
// restart a container, reload a service). Optional: providers that do not implement
// it report `unsupported` for lifecycle ops.
type Actor interface {
	Lifecycle(ctx context.Context, inst Instance, ref, action string) Result
}

// ConfigManager is implemented by providers that own text configuration files and
// can validate them before they take effect. Optional, like Actor.
type ConfigManager interface {
	// ReadConfig returns the current bytes of one config file.
	ReadConfig(ctx context.Context, inst Instance, path string) Result
	// Validate checks candidate bytes WITHOUT touching the live tree. Some tools can
	// only check a complete config, not a fragment; those return succeeded with a
	// message saying validation is deferred, and ValidateLive does the real work.
	Validate(ctx context.Context, inst Instance, path string, content []byte) Result
	// ValidateLive checks the configuration as it currently sits on disk. The apply
	// pipeline calls it after the write and before the activation, so a fragment that
	// only breaks in context is caught and rolled back without ever going live.
	// path is the file that was just written; connectors that can only check a whole
	// tree (nginx) ignore it.
	ValidateLive(ctx context.Context, inst Instance, path string) Result
	// Activate makes an already-written config live (reload/restart). Called by the
	// shared apply pipeline in safety.go, never directly by the executor.
	// path identifies which file changed, so systemd can reload that unit after
	// daemon-reload rather than restarting the whole machine's services.
	Activate(ctx context.Context, inst Instance, path string) Result
}

// PortLister is implemented by providers that can report which TCP ports their
// objects are listening on. Optional: others report `unsupported` for the ports op.
type PortLister interface {
	Ports(ctx context.Context, inst Instance) Result
}

// allowedActions is the closed set of lifecycle verbs. Anything else is refused
// before it can reach a shell, so a compromised control plane cannot smuggle in an
// arbitrary subcommand through the action field.
var allowedActions = map[string]bool{
	"start":   true,
	"stop":    true,
	"restart": true,
	"reload":  true,
	"enable":  true,
	"disable": true,
}

// ValidAction reports whether an action is in the allowlist.
func ValidAction(a string) bool { return allowedActions[a] }
