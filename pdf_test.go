package validator

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestExtractXML(t *testing.T) {
	data, err := os.ReadFile("testdata/corpus/facturx-en16931-invoice-standard.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if !IsPDF(data) {
		t.Fatal("IsPDF = false")
	}
	invoice, err := ExtractXML(data)
	if err != nil {
		t.Fatal(err)
	}
	if f, err := Detect(invoice); err != nil || f.ID != "facturx-en16931" {
		t.Fatalf("embedded XML: %+v, %v", f, err)
	}
}

// minimalPDF builds a one-page PDF without attachments.
func minimalPDF() []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>",
	}
	var b strings.Builder
	b.WriteString("%PDF-1.7\n")
	offsets := make([]int, len(objects))
	for n, o := range objects {
		offsets[n] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", n+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return []byte(b.String())
}

func TestValidatePDFWithoutInvoice(t *testing.T) {
	v := testValidator(t)
	pdf := minimalPDF()
	if _, err := ExtractXML(pdf); !errors.Is(err, ErrNoInvoiceXML) {
		t.Fatalf("ExtractXML: %v", err)
	}
	if r := v.ValidatePDF(pdf); r.State != Unsupported || r.Issues[0].RuleID != "PDF-NO-XML" {
		t.Errorf("got %+v", r)
	}
	if r := v.ValidatePDF([]byte("%PDF-1.7 garbage")); r.State != Error || r.Issues[0].Stage != StagePDF {
		t.Errorf("damaged PDF: got %+v", r)
	}
}
