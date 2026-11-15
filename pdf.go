package validator

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"strings"

	pdf "github.com/speedata/pdfdisassembler"
)

const (
	maxAttachments     = 128
	maxAttachmentBytes = 20 << 20 // decoded size of all attachments together
)

var (
	// ErrNoInvoiceXML: the PDF has no embedded invoice XML. It may still be
	// a readable invoice, but it is not a structured e-invoice.
	ErrNoInvoiceXML = errors.New("the PDF contains no embedded invoice XML")
	// ErrMultipleInvoiceXML: the PDF embeds more than one invoice XML.
	ErrMultipleInvoiceXML = errors.New("the PDF contains more than one invoice XML")
)

// ExtractXML returns the invoice XML embedded in a ZUGFeRD / Factur-X or
// XRechnung PDF. It looks at the EmbeddedFiles name tree and the document's
// associated files (/AF). An attachment counts as invoice when its root
// element is a CII CrossIndustryInvoice or UBL Invoice/CreditNote; when none
// is, a single attachment named *.xml is returned so that a malformed
// invoice still gets validated.
func ExtractXML(data []byte) ([]byte, error) {
	r, err := pdf.Open(bytes.NewReader(data), pdf.WithMaxStreamSize(maxAttachmentBytes))
	if err != nil {
		return nil, fmt.Errorf("PDF damaged, encrypted or unreadable: %w", err)
	}
	defer r.Close()
	if count, err := r.PageCount(); err != nil || count == 0 {
		return nil, errors.New("the PDF has no readable pages")
	}
	cat, err := r.Catalog()
	if err != nil {
		return nil, err
	}
	attachments := r.EmbeddedFiles()
	// Associated files may be registered in /AF without an EmbeddedFiles name tree.
	if associated, ok := cat.Array("AF"); ok {
		for _, ref := range associated {
			spec, err := r.ResolveDict(ref)
			if err != nil {
				return nil, fmt.Errorf("unreadable PDF attachment: %w", err)
			}
			attachments = append(attachments, pdf.EmbeddedFile{Spec: spec})
		}
	}
	if len(attachments) > maxAttachments {
		return nil, fmt.Errorf("the PDF has more than %d attachments", maxAttachments)
	}
	seen := map[*pdf.Dict]bool{}
	var invoices, otherXML [][]byte
	decoded := 0
	for _, a := range attachments {
		if seen[a.Spec] {
			continue
		}
		seen[a.Spec] = true
		name := a.Name
		if n, ok := a.Spec.String("UF"); ok {
			name = string(n)
		} else if n, ok := a.Spec.String("F"); ok {
			name = string(n)
		}
		ef, ok := a.Spec.Dict("EF")
		if !ok {
			return nil, fmt.Errorf("PDF attachment %q has no data", name)
		}
		stream, ok := ef.Get("UF")
		if !ok {
			stream, ok = ef.Get("F")
		}
		if !ok {
			return nil, fmt.Errorf("PDF attachment %q has no data", name)
		}
		content, err := r.DecodeStream(stream)
		if err != nil {
			return nil, fmt.Errorf("PDF attachment %q unreadable: %w", name, err)
		}
		if decoded += len(content); decoded > maxAttachmentBytes {
			return nil, fmt.Errorf("decoded PDF attachments exceed %d MB", maxAttachmentBytes>>20)
		}
		if isInvoiceXML(content) {
			invoices = append(invoices, content)
		} else if strings.HasSuffix(strings.ToLower(name), ".xml") {
			otherXML = append(otherXML, content)
		}
	}
	// Still validate a malformed XML attachment, but never let unrelated
	// metadata displace a recognizable invoice.
	if len(invoices) == 0 {
		invoices = otherXML
	}
	switch len(invoices) {
	case 0:
		return nil, ErrNoInvoiceXML
	case 1:
		return invoices[0], nil
	}
	return nil, ErrMultipleInvoiceXML
}

func isInvoiceXML(data []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		if start, ok := tok.(xml.StartElement); ok {
			n := start.Name
			return n.Space == nsCII && n.Local == "CrossIndustryInvoice" ||
				n.Space == nsUBLInvoice && n.Local == "Invoice" ||
				n.Space == nsUBLCreditNote && n.Local == "CreditNote"
		}
	}
}

// ValidatePDF extracts the invoice XML from a PDF and validates it.
// PDF/A-3 conformance of the PDF itself is not checked.
func (v *Validator) ValidatePDF(data []byte) Result {
	return v.ValidatePDFContext(context.Background(), data)
}

// ValidatePDFContext is ValidatePDF with a context.
func (v *Validator) ValidatePDFContext(ctx context.Context, data []byte) Result {
	invoice, err := ExtractXML(data)
	switch {
	case errors.Is(err, ErrNoInvoiceXML):
		return failed(Unsupported, StagePDF, "PDF-NO-XML", err)
	case errors.Is(err, ErrMultipleInvoiceXML):
		return failed(Unsupported, StagePDF, "PDF-MULTIPLE-XML", err)
	case err != nil:
		return failed(Error, StagePDF, "PDF", err)
	}
	return v.ValidateContext(ctx, invoice)
}

// IsPDF reports whether data starts like a PDF file.
func IsPDF(data []byte) bool {
	return bytes.HasPrefix(bytes.TrimLeft(data[:min(len(data), 1024)], "\x00\t\r\n "), []byte("%PDF-"))
}

// ValidateFile reads a file and validates it as PDF or XML by its content.
func (v *Validator) ValidateFile(path string) Result {
	data, err := os.ReadFile(path)
	if err != nil {
		return failed(Error, StageSyntax, "READ", err)
	}
	if IsPDF(data) {
		return v.ValidatePDF(data)
	}
	return v.Validate(data)
}
