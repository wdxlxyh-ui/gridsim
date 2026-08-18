package trend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func testTemplate() Template {
	return Template{
		ID:   "station-a",
		Name: "变电站 A",
		Panels: []Panel{{
			Kind: "collection",
			TraceConfigs: []TraceConfig{{InstID: "abc123", Inst: "实例 A", IOA: 16385, PointType: "AI"}},
		}},
	}
}

func TestStoreRoundTripAndUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trends", "templates.json")
	store := NewStoreAt(path)
	if err := store.Load(); err != nil {
		t.Fatalf("Load empty store: %v", err)
	}
	saved, err := store.Save(testTemplate())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.CreatedAt == 0 || saved.UpdatedAt == 0 {
		t.Fatalf("timestamps were not assigned: %+v", saved)
	}

	loaded := NewStoreAt(path)
	if err := loaded.Load(); err != nil {
		t.Fatalf("Load saved store: %v", err)
	}
	items := loaded.List()
	if len(items) != 1 || items[0].Name != "变电站 A" || len(items[0].Panels[0].TraceConfigs) != 1 {
		t.Fatalf("unexpected loaded templates: %+v", items)
	}

	updated := items[0]
	updated.Name = "变电站 A（更新）"
	if _, err := loaded.Save(updated); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := loaded.List(); len(got) != 1 || got[0].Name != "变电站 A（更新）" {
		t.Fatalf("unexpected update result: %+v", got)
	}
}

func TestStoreDeleteAndInvalidDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "templates.json")
	store := NewStoreAt(path)
	if _, err := store.Save(testTemplate()); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("station-a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(store.List()) != 0 {
		t.Fatalf("template was not deleted")
	}
	if err := store.Delete("missing"); !os.IsNotExist(err) {
		t.Fatalf("Delete missing error = %v, want os.ErrNotExist", err)
	}

	if err := os.WriteFile(path, []byte(`{"version":1,"templates":[{"id":"bad"},{"id":"bad"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	invalid := NewStoreAt(path)
	if err := invalid.Load(); err == nil {
		t.Fatal("Load accepted duplicate IDs")
	}
}

func TestStoreWritesVersionedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "templates.json")
	store := NewStoreAt(path)
	if _, err := store.Save(testTemplate()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document fileData
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if document.Version != fileVersion || len(document.Templates) != 1 {
		t.Fatalf("unexpected document: %+v", document)
	}
}
