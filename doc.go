// Package validator checks German and European e-invoices (XRechnung 3.0 in
// CII and UBL, ZUGFeRD / Factur-X BASIC, EN 16931 and EXTENDED) in pure Go.
//
// It runs the official XSD schemas and Schematron rules — the KoSIT
// configuration for XRechnung and the FeRD package for Factur-X — without
// Java and without network access. The rules are embedded in the module and
// pinned by SHA-256. Severities follow the KoSIT scenario configuration, so a
// document gets the same verdict, rule IDs and severities as from the KoSIT
// validator; the repository's parity job checks this against the latest
// official releases.
//
//	v, err := validator.New(validator.Options{})
//	if err != nil { ... }
//	result := v.Validate(xmlBytes)
//	if !result.Accepted() {
//		for _, issue := range result.Issues { fmt.Println(issue) }
//	}
//
// ZUGFeRD / Factur-X PDFs carry the invoice as an embedded XML file;
// [Validator.ValidatePDF] extracts it and validates the XML. PDF/A-3
// conformance of the PDF itself is not checked.
//
// The package validator/validatortest offers assertions for Go tests of
// invoice generators.
package validator
