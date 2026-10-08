package hosttools

import (
	"bytes"
	"testing"
)

func TestStreamWriterForwardsChunks(t *testing.T) {
	var got [][]byte
	var lastPct int32
	w := newStreamWriter(func(chunk []byte, percent int32) {
		got = append(got, append([]byte(nil), chunk...))
		lastPct = percent
	})
	if _, err := w.Write([]byte("hello ")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("world\n")); err != nil {
		t.Fatal(err)
	}
	if got := w.String(); got != "hello world\n" {
		t.Fatalf("buf=%q", got)
	}
	if len(got) != 2 || !bytes.Equal(got[0], []byte("hello ")) || !bytes.Equal(got[1], []byte("world\n")) {
		t.Fatalf("chunks=%q", got)
	}
	if lastPct < 1 || lastPct > 90 {
		t.Fatalf("percent=%d", lastPct)
	}
}
