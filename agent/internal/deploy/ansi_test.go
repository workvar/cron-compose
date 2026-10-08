package deploy

import "testing"

func TestScrubPTYLogSpinner(t *testing.T) {
	in := []byte("\x1b[1G\x1b[0K-\x1b[1G\x1b[0K\\\x1b[1G\x1b[0K|")
	if got := scrubPTYLog(in); got != nil {
		t.Fatalf("spinner frames should drop, got %q", got)
	}
}

func TestScrubPTYLogKeepsText(t *testing.T) {
	in := []byte("\x1b[32madded 42 packages\x1b[0m in 3s\n")
	got := string(scrubPTYLog(in))
	want := "added 42 packages in 3s\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestScrubPTYLogCR(t *testing.T) {
	in := []byte("progress 10%\rprogress 20%\n")
	got := string(scrubPTYLog(in))
	want := "progress 10%progress 20%\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
