package connectors

import "testing"

func TestNormalizeLabel(t *testing.T) {
	t.Parallel()

	got, del, err := NormalizeLabel("  Next.js UI  ")
	if err != nil || del || got != "Next.js UI" {
		t.Fatalf("got %q del=%v err=%v", got, del, err)
	}

	got, del, err = NormalizeLabel("   ")
	if err != nil || !del || got != "" {
		t.Fatalf("empty should delete, got %q del=%v err=%v", got, del, err)
	}

	if _, _, err := NormalizeLabel(string(make([]byte, 65))); err == nil {
		t.Fatal("expected too-long error")
	}
}

func TestApplyLabels(t *testing.T) {
	t.Parallel()

	rows := []PortRow{
		{Proto: "tcp", Address: "0.0.0.0", Port: 80, Name: "nginx"},
		{Proto: "tcp", Address: "127.0.0.1", Port: 3000, Name: "web"},
	}
	labels := []PortLabel{
		{ServerID: "s1", Proto: "tcp", Address: "0.0.0.0", Port: 80, Label: "public http"},
	}
	out := ApplyLabels("s1", rows, labels)
	if out[0].Label != "public http" {
		t.Fatalf("port 80 label = %q", out[0].Label)
	}
	if out[1].Label != "" {
		t.Fatalf("unlabelled port got %q", out[1].Label)
	}
	if rows[0].Label != "" {
		t.Fatal("ApplyLabels must not mutate the input rows")
	}
}
