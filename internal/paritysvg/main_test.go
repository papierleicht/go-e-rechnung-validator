package main

import (
	"encoding/xml"
	"strings"
	"testing"
)

func sample(matching int) results {
	return results{
		Checked: "2026-11-15",
		Bundle:  map[string]string{"XRechnung": "3.0.2", "Factur-X": "1.09.2"},
		References: []reference{
			{ID: "kosit", Name: "KoSIT-Validator", Version: "1.6.3", Details: "Konfiguration & Testsuite",
				Checks: []check{{Label: "Testsuite", Unit: "documents", Total: 78, Matching: matching}, {Label: "Regeln", Unit: "files", Total: 5, Matching: 5}}},
			{ID: "mustang", Name: "Mustang", Version: "2.26.0", Checks: []check{{Label: "PDF", Unit: "documents", Total: 6, Matching: 6}}},
		},
	}
}

func wellFormed(t *testing.T, svg string) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(svg))
	for {
		if _, err := dec.Token(); err != nil {
			if err.Error() == "EOF" {
				return
			}
			t.Fatalf("SVG not well-formed: %v", err)
		}
	}
}

func TestGraphics(t *testing.T) {
	ok := sample(78)
	for _, svg := range []string{paritySVG(ok), bannerSVG(ok)} {
		wellFormed(t, svg)
		if strings.Contains(svg, "abweichend") || strings.Contains(svg, "Abweichung") {
			t.Error("parity drawn as deviation")
		}
	}
	if !strings.Contains(bannerSVG(ok), "78/78") || !strings.Contains(paritySVG(ok), "Konfiguration &amp; Testsuite") {
		t.Error("counts or escaping missing")
	}
	bad := sample(77)
	if s := paritySVG(bad); !strings.Contains(s, "77/78") || !strings.Contains(s, "abweichend") {
		t.Error("deviation not shown in parity.svg")
	}
	if !strings.Contains(bannerSVG(bad), "Abweichung") {
		t.Error("deviation not shown in banner.svg")
	}
	if b := badge(bad.References[0]); !strings.Contains(b, `"77/78 identisch"`) || strings.Contains(b, "397e6b") {
		t.Errorf("badge %s", b)
	}
}
