package validator

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestCoversEmbeddedRules(t *testing.T) {
	want := manifest()
	seen := 0
	err := fs.WalkDir(bundled, "rules", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := strings.TrimPrefix(path, "rules/")
		data, err := fs.ReadFile(bundled, path)
		if err != nil {
			return err
		}
		if sum, ok := want[name]; !ok {
			t.Errorf("%s is embedded but not in rules.sha256.json", name)
		} else if hash(data) != sum {
			t.Errorf("%s does not match rules.sha256.json", name)
		}
		seen++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != len(want) || Rules().Files != seen {
		t.Errorf("embedded %d files, manifest %d", seen, len(want))
	}
}

func TestMaterializeRepairsChangedFiles(t *testing.T) {
	dir := t.TempDir()
	root, err := materialize(dir)
	if err != nil {
		t.Fatal(err)
	}
	sheet := filepath.Join(root, "xrechnung", "scenarios.xml")
	if err := os.WriteFile(sheet, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	again, err := materialize(dir)
	if err != nil || again != root {
		t.Fatalf("got %q, %v", again, err)
	}
	if err := verify(root, manifest()); err != nil {
		t.Fatal(err)
	}
}
