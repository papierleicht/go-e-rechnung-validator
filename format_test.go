package validator

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	data, err := os.ReadFile("testdata/corpus/xrechnung-ubl-credit-note-standard.xml")
	if err != nil {
		t.Fatal(err)
	}
	d, err := detect(data)
	if err != nil || d.format.ID != "xrechnung-ubl" || !d.creditNote {
		t.Fatalf("got %+v, %v", d, err)
	}
	for _, f := range Formats() {
		if f.Syntax != "cii" {
			continue
		}
		doc := `<rsm:CrossIndustryInvoice xmlns:rsm="` + nsCII + `" xmlns:ram="` + nsRAM + `">
  <rsm:ExchangedDocumentContext>
    <ram:BusinessProcessSpecifiedDocumentContextParameter><ram:ID>urn:fdc:peppol.eu:2017:poacc:billing:01:1.0</ram:ID></ram:BusinessProcessSpecifiedDocumentContextParameter>
    <ram:GuidelineSpecifiedDocumentContextParameter><ram:ID> ` + f.Specification + ` </ram:ID></ram:GuidelineSpecifiedDocumentContextParameter>
  </rsm:ExchangedDocumentContext>
</rsm:CrossIndustryInvoice>`
		got, err := Detect([]byte(doc))
		if err != nil || got != f {
			t.Errorf("%s: got %+v, %v", f.ID, got, err)
		}
	}
}

func TestDetectUnsupported(t *testing.T) {
	for name, doc := range map[string]string{
		"root":    `<Invoice/>`,
		"minimum": `<rsm:CrossIndustryInvoice xmlns:rsm="` + nsCII + `" xmlns:ram="` + nsRAM + `"><rsm:ExchangedDocumentContext><ram:GuidelineSpecifiedDocumentContextParameter><ram:ID>urn:factur-x.eu:1p0:minimum</ram:ID></ram:GuidelineSpecifiedDocumentContextParameter></rsm:ExchangedDocumentContext></rsm:CrossIndustryInvoice>`,
		"missing": `<Invoice xmlns="` + nsUBLInvoice + `"/>`,
	} {
		if _, err := Detect([]byte(doc)); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestRejectsBeforeRules(t *testing.T) {
	v := testValidator(t)
	cases := map[string]struct {
		doc   string
		state State
		rule  string
	}{
		"doctype":     {`<!DOCTYPE x [<!ENTITY e SYSTEM "file:///etc/passwd">]><x>&e;</x>`, Invalid, "DOCTYPE"},
		"malformed":   {`<Invoice><open></Invoice>`, Invalid, "XML"},
		"unsupported": {`<Invoice xmlns="` + nsUBLInvoice + `"/>`, Unsupported, "UNSUPPORTED"},
	}
	for name, c := range cases {
		r := v.Validate([]byte(c.doc))
		if r.State != c.state || len(r.Issues) != 1 || r.Issues[0].RuleID != c.rule {
			t.Errorf("%s: got %+v", name, r)
		}
	}
}

func TestSchemaAndRuleFindings(t *testing.T) {
	v := testValidator(t)
	data, err := os.ReadFile("testdata/corpus/xrechnung-cii-invoice-standard.xml")
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(data), "<rsm:ExchangedDocument>", "<rsm:ExchangedDocument><ram:Unknown/>", 1)
	if r := v.Validate([]byte(broken)); r.State != Invalid || r.Issues[0].Stage != StageXSD {
		t.Errorf("schema violation: got %+v", r)
	}
	r := v.ValidateFile("testdata/corpus/xrechnung-cii-wrong-total.xml")
	if r.State != Invalid || len(r.Errors()) == 0 || r.Errors()[0].Stage != StageRules || r.Errors()[0].Location == "" {
		t.Errorf("rule violation: got %+v", r)
	}
	if s := r.Errors()[0].String(); !strings.HasPrefix(s, "error BR-") && !strings.HasPrefix(s, "fatal BR-") {
		t.Errorf("issue string %q", s)
	}
}
