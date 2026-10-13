package deploy

import "testing"

func TestSplitInstallBuild(t *testing.T) {
	inst, build := splitInstallBuild("npm ci && npm run build", "node")
	if inst != "npm ci" || build != "npm run build" {
		t.Fatalf("got install=%q build=%q", inst, build)
	}
	inst, build = splitInstallBuild("go mod download && go build -o app .", "go")
	if inst != "go mod download" || build != "go build -o app ." {
		t.Fatalf("go: install=%q build=%q", inst, build)
	}
	inst, build = splitInstallBuild("go build -o app .", "go")
	if inst != "" || build != "go build -o app ." {
		t.Fatalf("build-only: install=%q build=%q", inst, build)
	}
}
