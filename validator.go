package validator

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/knroy/go-xml/xdm"
	"github.com/knroy/go-xml/xsd"
	"github.com/knroy/go-xml/xslt"
)

// Options configure a Validator. The zero value is ready to use.
type Options struct {
	// CacheDir receives the bundled rule files, which the XSD and XSLT
	// engines load from disk. Default: os.UserCacheDir()/go-e-rechnung-validator,
	// or the temporary directory when there is no user cache directory.
	// Files are verified against their SHA-256 on every New.
	CacheDir string
	// Timeout bounds the Schematron run of one document. Default: 20 seconds.
	Timeout time.Duration
}

// Validator validates invoices. It is safe for concurrent use; validations
// run one at a time because the compiled rule sets are shared. Create one
// Validator and reuse it: rule sets compile on first use per format (about a
// second each) and stay in memory.
type Validator struct {
	root    string
	timeout time.Duration
	setsMu  sync.Mutex
	runMu   sync.Mutex
	sets    map[string]*ruleSet
}

type ruleSet struct {
	once   sync.Once
	schema *xsd.Schema
	sheets []ruleSheet
	levels map[string]string
	err    error
}

type ruleSheet struct {
	sheet    *xslt.Stylesheet
	resolver *xslt.FileResolver
}

// New prepares a Validator. Rule sets compile lazily on first use.
func New(opts Options) (*Validator, error) {
	dir := opts.CacheDir
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			base = os.TempDir()
		}
		dir = filepath.Join(base, "go-e-rechnung-validator")
	}
	root, err := materialize(dir)
	if err != nil {
		return nil, fmt.Errorf("prepare rule files: %w", err)
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Validator{root: root, timeout: timeout, sets: map[string]*ruleSet{}}, nil
}

// Validate checks an invoice XML document against the schema and rules of
// its profile.
func (v *Validator) Validate(data []byte) Result {
	return v.ValidateContext(context.Background(), data)
}

// ValidateContext is Validate with a context that can cancel the Schematron run.
func (v *Validator) ValidateContext(ctx context.Context, data []byte) Result {
	if bytes.Contains(bytes.ToUpper(data), []byte("<!DOCTYPE")) {
		return failed(Invalid, StageSyntax, "DOCTYPE", errors.New("DOCTYPE declarations are not allowed in invoice XML"))
	}
	tree, err := xdm.Parse(bytes.NewReader(data), xdm.ParseOptions{TrackPositions: true})
	if err != nil {
		return failed(Invalid, StageSyntax, "XML", err)
	}
	doc, err := detect(data)
	if err != nil {
		return failed(Unsupported, StageProfile, "UNSUPPORTED", err)
	}
	r := v.rules(doc)
	if r.err != nil {
		return failed(Error, StageEngine, "VALIDATOR-INIT", r.err)
	}

	v.runMu.Lock()
	defer v.runMu.Unlock()
	result := Result{State: Valid, Format: doc.format.ID, Issues: []Issue{}}
	if err = r.schema.Validate(tree.Root, xsd.ValidateOptions{}); err != nil {
		result.State = Invalid
		result.Issues = append(result.Issues, Issue{Stage: StageXSD, RuleID: "XSD", Severity: "error", Message: err.Error()})
		return result
	}
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()
	for _, s := range r.sheets {
		out, err := s.sheet.Transform(ctx, tree.Root, xslt.TransformOptions{Documents: s.resolver, Texts: s.resolver})
		if err != nil {
			return engineFailure(doc, "XSLT", err)
		}
		var buf bytes.Buffer
		if err = out.Serialize(&buf); err != nil {
			return engineFailure(doc, "SVRL-SERIALIZE", err)
		}
		var report svrlReport
		if err = xml.Unmarshal(buf.Bytes(), &report); err != nil {
			return engineFailure(doc, "SVRL-PARSE", err)
		}
		for _, i := range append(report.Failed, report.Reports...) {
			severity := i.Flag
			if level := r.levels[i.ID]; level != "" {
				severity = level
			}
			if severity == "" {
				severity = "error"
			}
			issue := Issue{Stage: StageRules, RuleID: i.ID, Severity: severity, Message: strings.TrimSpace(i.Text), Location: i.Location}
			result.Issues = append(result.Issues, issue)
			if issue.Blocking() {
				result.State = Invalid
			} else if result.State == Valid {
				result.State = Warning
			}
		}
	}
	return result
}

func engineFailure(doc document, code string, err error) Result {
	if errors.Is(err, context.DeadlineExceeded) {
		code = "TIMEOUT"
	}
	r := failed(Error, StageEngine, code, err)
	r.Format = doc.format.ID
	return r
}

type svrlReport struct {
	Failed  []svrlIssue `xml:"failed-assert"`
	Reports []svrlIssue `xml:"successful-report"`
}

type svrlIssue struct {
	ID       string `xml:"id,attr"`
	Flag     string `xml:"flag,attr"`
	Location string `xml:"location,attr"`
	Text     string `xml:"text"`
}

func (v *Validator) rules(doc document) *ruleSet {
	key := doc.format.ID
	if doc.creditNote {
		key += "-credit"
	}
	v.setsMu.Lock()
	r := v.sets[key]
	if r == nil {
		r = &ruleSet{}
		v.sets[key] = r
	}
	v.setsMu.Unlock()
	r.once.Do(func() { r.err = r.compile(v.root, doc) })
	return r
}

func (r *ruleSet) compile(root string, doc document) error {
	f := doc.format
	var schema string
	var sheets []string
	if !f.XRechnung {
		level := strings.ReplaceAll(f.Conformance, " ", "")
		root = filepath.Join(root, f.ID)
		schema = "Factur-X_1.09.2_" + level + ".xsd"
		sheets = []string{"_XSLT_" + level + "/FACTUR-X_" + level + ".xslt"}
	} else {
		scenario := "EN16931 XRechnung (CII)"
		if f.Syntax == "ubl" {
			scenario = "EN16931 XRechnung (UBL Invoice)"
			if doc.creditNote {
				scenario = "EN16931 XRechnung (UBL CreditNote)"
			}
		}
		var err error
		if r.levels, err = scenarioLevels(filepath.Join(root, "xrechnung", "scenarios.xml"), scenario); err != nil {
			return err
		}
		root = filepath.Join(root, "xrechnung", "resources")
		if f.Syntax == "cii" {
			schema = "cii/16b/xsd/CrossIndustryInvoice_100pD16B.xsd"
			sheets = []string{"cii/16b/xsl/EN16931-CII-validation.xsl", "xrechnung/3.0.2/xsl/XRechnung-CII-validation.xsl"}
		} else {
			schema = "ubl/2.1/xsd/maindoc/UBL-Invoice-2.1.xsd"
			if doc.creditNote {
				schema = "ubl/2.1/xsd/maindoc/UBL-CreditNote-2.1.xsd"
			}
			sheets = []string{"ubl/2.1/xsl/EN16931-UBL-validation.xsl", "xrechnung/3.0.2/xsl/XRechnung-UBL-validation.xsl"}
		}
	}
	opts := xsd.Options{Resolver: &xsd.FileResolver{Root: root}}
	// Only the trusted, bundled UBL schemas need their internal DTD. Invoice
	// input still rejects DOCTYPE and external entities are never enabled.
	opts.ParseOptions.AllowDOCTYPE = f.Syntax == "ubl"
	var err error
	if r.schema, err = xsd.LoadFile(filepath.Join(root, schema), opts); err != nil {
		return err
	}
	for _, name := range sheets {
		path := filepath.Join(root, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// go-xml rejects this XSLT 2.0 pattern. Only complete XML document
		// nodes are transformed, where root() and / select the same node.
		// The bundled files stay unchanged; assertions are not rewritten.
		if f.XRechnung {
			data = bytes.ReplaceAll(data, []byte(`match="root()"`), []byte(`match="/"`))
		}
		resolver, err := xslt.NewFileResolver(filepath.Dir(path))
		if err != nil {
			return err
		}
		tree, err := xdm.ParseString(string(data), xdm.ParseOptions{BaseURI: fileURI(path)})
		if err != nil {
			return err
		}
		sheet, err := xslt.Compile(tree.Root, xslt.CompileOptions{Resolver: resolver, BaseURI: fileURI(path)})
		if err != nil {
			return err
		}
		r.sheets = append(r.sheets, ruleSheet{sheet, resolver})
	}
	return nil
}

func fileURI(path string) string {
	abs, _ := filepath.Abs(path)
	if filepath.VolumeName(abs) != "" {
		abs = "/" + abs
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}).String()
}

// scenarioLevels reads the customLevel severity overrides of one standard
// KoSIT scenario (extension and CVD scenarios are separate and not selected).
func scenarioLevels(path, name string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Scenarios []struct {
			Name   string `xml:"name"`
			Levels []struct {
				Level string `xml:"level,attr"`
				ID    string `xml:",chardata"`
			} `xml:"createReport>customLevel"`
		} `xml:"scenario"`
	}
	if err = xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	for _, sc := range doc.Scenarios {
		if sc.Name != name {
			continue
		}
		levels := map[string]string{}
		for _, l := range sc.Levels {
			for _, id := range strings.Fields(l.ID) {
				levels[id] = l.Level
			}
		}
		return levels, nil
	}
	return nil, fmt.Errorf("KoSIT scenario %q not found in %s", name, path)
}
