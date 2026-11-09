package deploy

import (
	"bytes"
	"io"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

// pumpReadSize is large enough that a burst of build output is read in one call
// instead of 4KB at a time.
const pumpReadSize = 32 << 10

// pumpOutput streams installer stdout/stderr into deploy log events. Output is
// coalesced by a logBatcher so a chatty build becomes a few events per second
// instead of one per line. When tag is set, complete lines are prefixed with
// `[in <dir>]` for multi-app panes; a batch only ever holds whole tagged lines.
func (m *Manager) pumpOutput(runID, token, dir string, r io.Reader, tag bool) {
	batch := newLogBatcher(logFlushDelay, logFlushBytes, func(chunk []byte) {
		m.emit(runID, token, &agentv1.DeployEvent{
			RunId: runID, Kind: "log", Phase: phaseInstall, Data: chunk,
		})
	})
	defer batch.Close()

	prefix := ""
	if tag && dir != "" {
		prefix = "[in " + dir + "] "
	}
	var pending []byte
	buf := make([]byte, pumpReadSize)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if prefix == "" {
				batch.Write(buf[:n])
			} else {
				pending = append(pending, buf[:n]...)
				pending = writeTaggedLines(batch, prefix, pending)
			}
		}
		if err != nil {
			if prefix != "" && len(pending) > 0 {
				if pending[len(pending)-1] != '\n' {
					pending = append(pending, '\n')
				}
				writeTaggedLines(batch, prefix, pending)
			}
			return
		}
	}
}

// writeTaggedLines writes every complete line in pending with prefix and returns
// the unfinished tail.
func writeTaggedLines(batch *logBatcher, prefix string, pending []byte) []byte {
	var out []byte
	for {
		i := bytes.IndexByte(pending, '\n')
		if i < 0 {
			break
		}
		out = append(out, prefix...)
		out = append(out, pending[:i+1]...)
		pending = pending[i+1:]
	}
	if len(out) > 0 {
		batch.Write(out)
	}
	// Copy the tail so the read buffer can be reused safely.
	return append([]byte(nil), pending...)
}
