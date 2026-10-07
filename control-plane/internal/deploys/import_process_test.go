package deploys

import "testing"

func TestEnvMapToVarsMarksSensitive(t *testing.T) {
	vars := envMapToVars(map[string]string{"FOO": "bar", "": "skip"})
	if len(vars) != 1 {
		t.Fatalf("len=%d", len(vars))
	}
	if vars[0].Key != "FOO" || vars[0].Value != "bar" || !vars[0].Sensitive {
		t.Fatalf("%+v", vars[0])
	}
}
