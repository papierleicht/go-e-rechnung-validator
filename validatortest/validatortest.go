// Package validatortest provides test assertions for code that writes
// e-invoices: generate an XRechnung or ZUGFeRD / Factur-X document in a test
// and assert that the official rules accept it.
//
//	func TestInvoiceXML(t *testing.T) {
//		xml := myapp.RenderInvoice(sample)
//		validatortest.AssertValid(t, xml)
//	}
//
// All assertions share one Validator per test binary, so the rule sets
// compile once.
package validatortest

import (
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	validator "github.com/papierleicht/go-e-rechnung-validator"
)

var (
	once   sync.Once
	shared *validator.Validator
	err    error
)

// Validator returns the Validator shared by this package's assertions. Its
// rule files live in a directory that is removed when the process ends
// normally; reuse across test runs is not needed because writing them takes
// milliseconds.
func Validator(t testing.TB) *validator.Validator {
	t.Helper()
	once.Do(func() {
		var dir string
		if dir, err = os.MkdirTemp("", "validatortest-"); err == nil {
			shared, err = validator.New(validator.Options{CacheDir: dir})
		}
	})
	if err != nil {
		t.Fatalf("validatortest: %v", err)
	}
	return shared
}

// AssertValid fails the test unless the XML is accepted (valid, or valid
// with warnings). Every finding is reported.
func AssertValid(t testing.TB, xml []byte) validator.Result {
	t.Helper()
	r := Validator(t).Validate(xml)
	if !r.Accepted() {
		t.Errorf("invoice rejected (%s, format %q):\n%s", r.State, r.Format, list(r.Issues))
	}
	return r
}

// AssertValidPDF is AssertValid for a ZUGFeRD / Factur-X or XRechnung PDF.
// It checks the embedded invoice XML, not PDF/A-3 conformance.
func AssertValidPDF(t testing.TB, pdf []byte) validator.Result {
	t.Helper()
	r := Validator(t).ValidatePDF(pdf)
	if !r.Accepted() {
		t.Errorf("invoice PDF rejected (%s, format %q):\n%s", r.State, r.Format, list(r.Issues))
	}
	return r
}

// AssertNoWarnings fails the test unless the XML is valid without any
// warning or information message.
func AssertNoWarnings(t testing.TB, xml []byte) validator.Result {
	t.Helper()
	r := Validator(t).Validate(xml)
	if r.State != validator.Valid {
		t.Errorf("invoice not clean (%s, format %q):\n%s", r.State, r.Format, list(r.Issues))
	}
	return r
}

// AssertInvalid fails the test if the XML is accepted or if any of the given
// rule IDs is not among the blocking findings. Use it to check that broken
// input is caught by the intended rule.
func AssertInvalid(t testing.TB, xml []byte, ruleIDs ...string) validator.Result {
	t.Helper()
	r := Validator(t).Validate(xml)
	if r.Accepted() {
		t.Errorf("invoice accepted (%s), want rejection by %v", r.State, ruleIDs)
		return r
	}
	var blocking []string
	for _, i := range r.Errors() {
		blocking = append(blocking, i.RuleID)
	}
	for _, id := range ruleIDs {
		if !slices.Contains(blocking, id) {
			t.Errorf("rule %s did not reject the invoice; blocking findings:\n%s", id, list(r.Errors()))
		}
	}
	return r
}

func list(issues []validator.Issue) string {
	var b strings.Builder
	for _, i := range issues {
		b.WriteString("  " + i.String() + "\n")
	}
	return b.String()
}
