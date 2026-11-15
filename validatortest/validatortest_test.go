package validatortest_test

import (
	"os"
	"strings"
	"testing"

	"github.com/papierleicht/go-e-rechnung-validator/validatortest"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../testdata/corpus/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAssertions(t *testing.T) {
	validatortest.AssertValid(t, read(t, "xrechnung-ubl-invoice-standard.xml"))
	validatortest.AssertNoWarnings(t, read(t, "facturx-en16931-invoice-standard.xml"))
	validatortest.AssertValidPDF(t, read(t, "facturx-extended-invoice-standard.pdf"))
	r := validatortest.AssertInvalid(t, read(t, "xrechnung-cii-wrong-total.xml"))
	validatortest.AssertInvalid(t, read(t, "xrechnung-cii-wrong-total.xml"), r.Errors()[0].RuleID)
}

// recorder captures failures instead of failing the real test.
type recorder struct {
	testing.TB
	msgs []string
}

func (r *recorder) Helper()                           {}
func (r *recorder) Errorf(format string, args ...any) { r.msgs = append(r.msgs, format) }

func TestAssertionsFail(t *testing.T) {
	rec := &recorder{TB: t}
	validatortest.AssertValid(rec, read(t, "xrechnung-cii-wrong-total.xml"))
	validatortest.AssertInvalid(rec, read(t, "xrechnung-ubl-invoice-standard.xml"))
	validatortest.AssertInvalid(rec, read(t, "xrechnung-cii-wrong-total.xml"), "BR-NOT-A-RULE")
	if len(rec.msgs) != 3 {
		t.Fatalf("want 3 failures, got %d: %v", len(rec.msgs), rec.msgs)
	}
	if !strings.Contains(rec.msgs[0], "rejected") {
		t.Errorf("message %q", rec.msgs[0])
	}
}
