// Package outbox is the agent's persistent queue for AgentMessages.
//
// Every message we want to send upstream is first written to a bbolt-backed queue.
// A drain goroutine pulls the queue in order, sends each message, and deletes the key
// on success. Disconnect or restart leaves messages on disk; next connect drains them
// in order. This is what makes the "jobs keep firing while the control plane is down"
// promise actually hold for status + log events.
package outbox

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"

	agentv1 "github.com/croncompose/croncompose/proto/agent/v1"
)

var bucket = []byte("outbox")

// Outbox persists pending AgentMessages.
type Outbox struct {
	db   *bbolt.DB
	mu   sync.Mutex // guards the monotonic key counter
	next uint64
}

// Open creates or opens the outbox database under dataDir/outbox.db.
func Open(dataDir string) (*Outbox, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	db, err := bbolt.Open(filepath.Join(dataDir, "outbox.db"), 0o600, &bbolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, err
	}
	o := &Outbox{db: db}
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		return err
	}); err != nil {
		db.Close()
		return nil, err
	}
	// Seed monotonic counter from the highest existing key.
	if err := db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket(bucket).Cursor()
		if k, _ := c.Last(); k != nil {
			o.next = binary.BigEndian.Uint64(k) + 1
		}
		return nil
	}); err != nil {
		db.Close()
		return nil, err
	}
	return o, nil
}

// Close shuts down the underlying database.
func (o *Outbox) Close() error { return o.db.Close() }

// Enqueue serializes msg and appends it to the queue. Safe for concurrent callers.
//
// Writes go through bbolt's Batch so concurrent enqueuers (parallel app installs,
// phase lines, heartbeats) share one commit and one fsync instead of paying for
// one each. A single caller still returns only after its message is durable, so
// one goroutine's messages keep their order.
func (o *Outbox) Enqueue(msg *agentv1.AgentMessage) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	o.mu.Lock()
	key := o.next
	o.next++
	o.mu.Unlock()
	k := make([]byte, 8)
	binary.BigEndian.PutUint64(k, key)
	return o.db.Batch(func(tx *bbolt.Tx) error {
		return tx.Bucket(bucket).Put(k, data)
	})
}

// drainBatch is how many messages one read transaction pulls. Sent messages are
// deleted together, so a backlog of log events costs one fsync per batch rather
// than one per message.
const drainBatch = 256

type pendingMsg struct {
	key []byte
	msg *agentv1.AgentMessage
}

// Drain iterates the queue in order. fn returns nil on success (the key is then
// deleted) or an error (drain stops, that key and everything after it stays).
// Returns nil when the queue is empty.
func (o *Outbox) Drain(fn func(msg *agentv1.AgentMessage) error) error {
	for {
		batch, err := o.peek(drainBatch)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		sent := 0
		var sendErr error
		for _, p := range batch {
			if sendErr = fn(p.msg); sendErr != nil {
				break
			}
			sent++
		}
		if err := o.remove(batch[:sent]); err != nil {
			return err
		}
		if sendErr != nil {
			return sendErr
		}
	}
}

// peek reads up to n messages from the head of the queue.
func (o *Outbox) peek(n int) ([]pendingMsg, error) {
	var out []pendingMsg
	err := o.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket(bucket).Cursor()
		for k, v := c.First(); k != nil && len(out) < n; k, v = c.Next() {
			msg := &agentv1.AgentMessage{}
			if err := proto.Unmarshal(v, msg); err != nil {
				return err
			}
			out = append(out, pendingMsg{key: append([]byte(nil), k...), msg: msg})
		}
		return nil
	})
	return out, err
}

// remove deletes sent messages in one transaction.
func (o *Outbox) remove(batch []pendingMsg) error {
	if len(batch) == 0 {
		return nil
	}
	return o.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucket)
		for _, p := range batch {
			if err := b.Delete(p.key); err != nil {
				return err
			}
		}
		return nil
	})
}

// Len returns the number of pending messages. Useful for metrics.
func (o *Outbox) Len() int {
	n := 0
	_ = o.db.View(func(tx *bbolt.Tx) error {
		n = tx.Bucket(bucket).Stats().KeyN
		return nil
	})
	return n
}

// errStop is kept for callers that compare against an empty-queue sentinel.
var errStop = errors.New("outbox: empty")
