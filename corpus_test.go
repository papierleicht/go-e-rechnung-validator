package validator

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/corpus/expected.json")

var (
	sharedOnce sync.Once
	shared     *Validator
	sharedErr  error
)

func testValidator(t *testing.T) *Validator {
	t.Helper()
	sharedOnce.Do(func() {
		dir, err := os.MkdirTemp("", "go-e-rechnung-validator-test-")
		if err != nil {
			sharedErr = err
			return
		}
		shared, sharedErr = New(Options{CacheDir: dir})
	})
	if sharedErr != nil {
		t.Fatal(sharedErr)
	}
	return shared
}

type expectation struct {
	State  State    `json:"state"`
	Format string   `json:"format"`
	Rules  []string `json:"rules"`
}

func summarize(r Result) expectation {
	ids := r.RuleIDs()
	slices.Sort(ids)
	if ids == nil {
		ids = []string{}
	}
	return expectation{r.State, r.Format, ids}
}

// TestCorpus pins the verdict and rule IDs for every document of the corpus.
// The parity job checks the same documents against KoSIT and Mustang; this
// test keeps them stable offline and without Java.
func TestCorpus(t *testing.T) {
	v := testValidator(t)
	files, err := filepath.Glob("testdata/corpus/*.*")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]expectation{}
	for _, f := range files {
		if strings.HasSuffix(f, ".json") {
			continue
		}
		got[filepath.Base(f)] = summarize(v.ValidateFile(f))
	}
	golden := "testdata/corpus/expected.json"
	if *update {
		data, _ := json.MarshalIndent(got, "", "  ")
		if err := os.WriteFile(golden, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]expectation
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if len(want) != len(got) {
		t.Errorf("corpus has %d documents, expected.json %d (run go test -run TestCorpus -update)", len(got), len(want))
	}
	for name, w := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("%s: missing in corpus", name)
			continue
		}
		if g.State != w.State || g.Format != w.Format || !slices.Equal(g.Rules, w.Rules) {
			t.Errorf("%s: got %s %s %v, want %s %s %v", name, g.State, g.Format, g.Rules, w.State, w.Format, w.Rules)
		}
		if want := strings.SplitN(name, "-", 3); w.Format != want[0]+"-"+want[1] {
			t.Errorf("%s: format %q does not match the file name", name, w.Format)
		}
	}
}

func TestConcurrentUse(t *testing.T) {
	v := testValidator(t)
	files, _ := filepath.Glob("testdata/corpus/*-standard.xml")
	var wg sync.WaitGroup
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for range 3 {
			wg.Go(func() {
				if r := v.Validate(data); !r.Accepted() {
					t.Errorf("%s: %s %v", f, r.State, r.Issues)
				}
			})
		}
	}
	wg.Wait()
}
