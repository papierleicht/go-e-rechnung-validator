package validator

import (
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
)

//go:embed all:rules
var bundled embed.FS

//go:embed rules.sha256.json
var manifestJSON []byte

// RuleBundle describes the official rule material embedded in this module.
type RuleBundle struct {
	XRechnung          string // XRechnung standard version
	KoSITConfiguration string // validator-configuration-xrechnung release
	FacturX            string // Factur-X version
	ZUGFeRD            string // matching ZUGFeRD version
	SHA256             string // hash over all rule files (rules.sha256.json)
	Files              int
}

// Rules returns the versions and hash of the embedded rule bundle.
func Rules() RuleBundle {
	sum := sha256.Sum256(manifestJSON)
	return RuleBundle{
		XRechnung:          "3.0.2",
		KoSITConfiguration: "2026-08-31",
		FacturX:            "1.09.2",
		ZUGFeRD:            "2.5.2",
		SHA256:             hex.EncodeToString(sum[:]),
		Files:              len(manifest()),
	}
}

func manifest() map[string]string {
	var m map[string]string
	if err := json.Unmarshal(manifestJSON, &m); err != nil {
		panic("go-e-rechnung-validator: broken rules.sha256.json: " + err.Error())
	}
	return m
}

// materialize writes the embedded rules below dir/<bundle hash> and returns
// that directory. An existing copy is reused when every file matches the
// manifest; otherwise a fresh copy replaces it.
func materialize(dir string) (string, error) {
	want := manifest()
	root := filepath.Join(dir, Rules().SHA256[:16])
	if verify(root, want) == nil {
		return root, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	tmp := fmt.Sprintf("%s.tmp-%s", root, rand.Text()[:8])
	if err := write(tmp, want); err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	_ = os.RemoveAll(root)
	if err := os.Rename(tmp, root); err != nil {
		_ = os.RemoveAll(tmp)
		// Another process may have won the race with an identical copy.
		if verify(root, want) == nil {
			return root, nil
		}
		return "", err
	}
	return root, nil
}

func write(dest string, want map[string]string) error {
	names := make([]string, 0, len(want))
	for name := range want {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		data, err := fs.ReadFile(bundled, path.Join("rules", name))
		if err != nil {
			return err
		}
		if hash(data) != want[name] {
			return fmt.Errorf("embedded rule file %s does not match its SHA-256", name)
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func verify(root string, want map[string]string) error {
	for name, sum := range want {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if hash(data) != sum {
			return errors.New("rule file " + name + " changed")
		}
	}
	return nil
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
