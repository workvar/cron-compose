package connectors

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

func (p *pm2Provider) dumpOf(inst Instance) string {
	if len(inst.ConfigPaths) > 0 && inst.ConfigPaths[0] != "" {
		return inst.ConfigPaths[0]
	}
	return pm2DumpPath()
}

func pm2Owns(path, dump string) bool {
	if dump == "" {
		return false
	}
	return filepath.Clean(path) == filepath.Clean(dump)
}

func (p *pm2Provider) owns(inst Instance, path string) bool {
	return pm2Owns(path, p.dumpOf(inst))
}

func (p *pm2Provider) ReadConfig(ctx context.Context, inst Instance, path string) Result {
	if !p.owns(inst, path) {
		return fail(StatusUnauthorized, "path is outside this connector's dump file: "+path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fail(StatusFailed, "read failed: "+err.Error())
	}
	return configBytesResult(path, b)
}

func (p *pm2Provider) Validate(ctx context.Context, inst Instance, path string, content []byte) Result {
	if !p.owns(inst, path) {
		return fail(StatusUnauthorized, "path is outside this connector's dump file: "+path)
	}
	if !json.Valid(content) {
		return fail(StatusInvalid, "dump.pm2 is not valid JSON")
	}
	return ok("JSON accepted")
}

func (p *pm2Provider) ValidateLive(ctx context.Context, inst Instance, path string) Result {
	if !p.owns(inst, path) {
		return fail(StatusUnauthorized, "path is outside this connector's dump file: "+path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fail(StatusInvalid, "read failed: "+err.Error())
	}
	if !json.Valid(b) {
		return fail(StatusInvalid, "dump.pm2 on disk is not valid JSON")
	}
	return ok("dump.pm2 is valid JSON")
}

// Activate restores the process list from dump.pm2. That is the file `pm2 save`
// writes, so resurrecting after a write is what actually makes the new dump live.
func (p *pm2Provider) Activate(ctx context.Context, inst Instance, path string) Result {
	out, err := run(ctx, "pm2", "resurrect")
	s := step("pm2 resurrect", err == nil, out)
	if err != nil {
		return fail(StatusFailed, "pm2 resurrect failed: "+trimOutput(out), s)
	}
	return ok("process list restored from dump.pm2", s)
}
