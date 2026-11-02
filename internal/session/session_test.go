package session

import (
	"testing"

	"github.com/Balanov/shim/internal/provider"
)

func TestSessionRoundTrip(t *testing.T) {
	store := New(t.TempDir())
	if err := store.Append(provider.Message{Role: "user", Content: "hello"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(provider.Message{Role: "assistant", Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	id := store.ID
	list, err := store.List()
	if err != nil || len(list) != 1 || list[0].Messages != 2 {
		t.Fatalf("unexpected list: %+v %v", list, err)
	}
	resumed := New(store.Dir)
	messages, err := resumed.Resume(id)
	if err != nil || len(messages) != 2 || messages[1].Content != "hi" {
		t.Fatalf("unexpected resume: %+v %v", messages, err)
	}
	if _, err := resumed.Resume("../bad"); err == nil {
		t.Fatal("invalid session id accepted")
	}
}
