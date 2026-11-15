package validator

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
)

// Format is an invoice profile in one XML syntax with a bundled rule set.
type Format struct {
	ID            string `json:"id"`
	Syntax        string `json:"syntax"`        // "cii" or "ubl"
	Specification string `json:"specification"` // BT-24 specification identifier
	Conformance   string `json:"conformance,omitempty"`
	XRechnung     bool   `json:"xrechnung"`
}

const (
	xrechnung30 = "urn:cen.eu:en16931:2017#compliant#urn:xeinkauf.de:kosit:xrechnung_3.0"

	nsCII            = "urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
	nsUBLInvoice     = "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
	nsUBLCreditNote  = "urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2"
	nsRAM            = "urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"
	nsUBLBasic       = "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
	maxDetectedDepth = 4
)

var formats = []Format{
	{ID: "xrechnung-cii", Syntax: "cii", Specification: xrechnung30, XRechnung: true},
	{ID: "xrechnung-ubl", Syntax: "ubl", Specification: xrechnung30, XRechnung: true},
	{ID: "facturx-basic", Syntax: "cii", Specification: "urn:cen.eu:en16931:2017#compliant#urn:factur-x.eu:1p0:basic", Conformance: "BASIC"},
	{ID: "facturx-en16931", Syntax: "cii", Specification: "urn:cen.eu:en16931:2017", Conformance: "EN 16931"},
	{ID: "facturx-extended", Syntax: "cii", Specification: "urn:cen.eu:en16931:2017#conformant#urn:factur-x.eu:1p0:extended", Conformance: "EXTENDED"},
}

// Formats lists the supported profiles.
func Formats() []Format { return append([]Format(nil), formats...) }

// ErrUnsupported is returned by Detect for documents without a bundled rule set.
var ErrUnsupported = errors.New("unsupported invoice profile")

// document is what Detect reads from the start of an invoice.
type document struct {
	format     Format
	creditNote bool // UBL CreditNote root element
}

// Detect returns the format of an invoice from its root element and
// specification identifier (BT-24), without validating it.
func Detect(data []byte) (Format, error) {
	d, err := detect(data)
	return d.format, err
}

func detect(data []byte) (document, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var path []string
	var syntax string
	var credit bool
	var spec *string
	for {
		tok, err := dec.Token()
		if err != nil {
			if spec == nil {
				return document{}, fmt.Errorf("%w: no specification identifier (BT-24) found", ErrUnsupported)
			}
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if len(path) == 0 {
				switch {
				case t.Name.Space == nsCII && t.Name.Local == "CrossIndustryInvoice":
					syntax = "cii"
				case t.Name.Space == nsUBLInvoice && t.Name.Local == "Invoice":
					syntax = "ubl"
				case t.Name.Space == nsUBLCreditNote && t.Name.Local == "CreditNote":
					syntax, credit = "ubl", true
				default:
					return document{}, fmt.Errorf("%w: root element {%s}%s is neither CII nor UBL", ErrUnsupported, t.Name.Space, t.Name.Local)
				}
			}
			path = append(path, t.Name.Space+" "+t.Name.Local)
			if len(path) > maxDetectedDepth {
				if err := dec.Skip(); err != nil {
					return document{}, err
				}
				path = path[:len(path)-1]
			}
		case xml.EndElement:
			path = path[:len(path)-1]
		case xml.CharData:
			if spec != nil || !specPath(syntax, path) {
				continue
			}
			s := strings.TrimSpace(string(t))
			spec = &s
		}
		if spec != nil {
			break
		}
	}
	for _, f := range formats {
		if f.Syntax == syntax && f.Specification == *spec {
			return document{format: f, creditNote: credit}, nil
		}
	}
	return document{}, fmt.Errorf("%w: no rule set for specification %q in %s syntax", ErrUnsupported, *spec, strings.ToUpper(syntax))
}

func specPath(syntax string, path []string) bool {
	switch syntax {
	case "cii":
		return len(path) == 4 &&
			path[1] == nsCII+" ExchangedDocumentContext" &&
			path[2] == nsRAM+" GuidelineSpecifiedDocumentContextParameter" &&
			path[3] == nsRAM+" ID"
	case "ubl":
		return len(path) == 2 && path[1] == nsUBLBasic+" CustomizationID"
	}
	return false
}
