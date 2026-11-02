package router

import (
	"context"
	"testing"

	"github.com/Balanov/shim/internal/provider"
)

func TestDeterministicRussian(t *testing.T) {
	command, ok := Deterministic("покажи модели")
	if !ok || command.Text != "/models" || command.Mutating {
		t.Fatalf("unexpected route: %+v %v", command, ok)
	}
	command, ok = Deterministic("используй модель local")
	if !ok || command.Text != "/model use local" || !command.Mutating {
		t.Fatalf("unexpected use route: %+v", command)
	}
	if _, ok := Deterministic("напиши тесты для проекта"); ok {
		t.Fatal("ordinary task was treated as configuration")
	}
}

func TestSplitAndValidate(t *testing.T) {
	args, err := Split(`/mcp add "my server" npx -y "package name"`)
	if err != nil || len(args) != 6 || args[2] != "my server" || args[5] != "package name" {
		t.Fatalf("unexpected split: %#v %v", args, err)
	}
	if mutating, err := Validate("/models"); err != nil || mutating {
		t.Fatalf("models should be read-only: %v %v", mutating, err)
	}
	if mutating, err := Validate("/discover --add"); err != nil || !mutating {
		t.Fatalf("discover --add should mutate: %v %v", mutating, err)
	}
	if _, err := Validate("/shell rm -rf /"); err == nil {
		t.Fatal("disallowed shell command accepted")
	}
}

type fakeClient struct{ response string }

func (f fakeClient) Complete(context.Context, provider.Request) (provider.Response, error) {
	return provider.Response{Text: f.response}, nil
}

func TestCompileAllowlist(t *testing.T) {
	command, err := Compile(context.Background(), fakeClient{`{"command":"/skill add owner/repo","mutating":false}`}, "local", "add a skill")
	if err != nil || command.Text != "/skill add owner/repo" || !command.Mutating {
		t.Fatalf("unexpected compiled command: %+v %v", command, err)
	}
	if _, err := Compile(context.Background(), fakeClient{`{"command":"/shell whoami"}`}, "local", "run shell"); err == nil {
		t.Fatal("unsafe compiled command accepted")
	}
}
