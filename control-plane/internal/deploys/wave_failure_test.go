package deploys

import "testing"

func TestNormalizeOnWaveFailure(t *testing.T) {
	cases := map[string]string{
		"":         OnWaveFailureStop,
		"stop":     OnWaveFailureStop,
		"STOP":     OnWaveFailureStop,
		"continue": OnWaveFailureContinue,
		"Continue": OnWaveFailureContinue,
		"nope":     OnWaveFailureStop,
	}
	for in, want := range cases {
		if got := NormalizeOnWaveFailure(in); got != want {
			t.Errorf("NormalizeOnWaveFailure(%q)=%q want %q", in, got, want)
		}
	}
}
