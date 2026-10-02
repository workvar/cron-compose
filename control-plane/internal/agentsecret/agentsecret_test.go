package agentsecret

import "testing"

func TestGenerateMatchesItsOwnHash(t *testing.T) {
	plain, hash, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) < 40 || plain == hash {
		t.Fatalf("secret looks weak or unhashed: %q", plain)
	}
	if !Matches(plain, hash) {
		t.Fatal("generated secret must match its hash")
	}
	other, _, _ := Generate()
	if other == plain {
		t.Fatal("secrets must differ")
	}
}

// Production change that would fail this test: treating an empty stored hash as a
// match, which would let any server without a secret be logged in with an empty one.
func TestMatchesRejectsEmptyAndWrong(t *testing.T) {
	plain, hash, _ := Generate()
	for name, c := range map[string][2]string{
		"empty plain":  {"", hash},
		"empty stored": {plain, ""},
		"both empty":   {"", ""},
		"wrong":        {plain + "x", hash},
	} {
		if Matches(c[0], c[1]) {
			t.Errorf("%s must not match", name)
		}
	}
}
