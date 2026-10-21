package storage

import (
	"os"
	"path/filepath"
	"testing"

	"gridsim/internal/model"
)

func TestConfigStoreClearPersistsEmptyArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instances.json")
	store := NewConfigStore(path)
	if err := store.Add(model.InstanceConfig{ID: "instance-1", Name: "实例"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	if got := store.List(); len(got) != 0 {
		t.Fatalf("configs=%d, want 0", len(got))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "[]" {
		t.Fatalf("file=%q, want []", data)
	}
}
