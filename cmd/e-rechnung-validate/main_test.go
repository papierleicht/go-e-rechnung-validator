package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const corpus = "../../testdata/corpus/"

func TestRun(t *testing.T) {
	cache := t.TempDir()
	cases := []struct {
		args   []string
		status int
		out    string
	}{
		{[]string{corpus + "xrechnung-ubl-invoice-standard.xml", corpus + "facturx-basic-invoice-standard.pdf"}, 0, "✓"},
		{[]string{"-q", corpus + "xrechnung-ubl-invoice-standard.xml", corpus + "xrechnung-ubl-wrong-total.xml"}, 1, "✗"},
		{[]string{corpus + "does-not-exist.xml"}, 2, "READ"},
		{[]string{}, 2, ""},
		{[]string{"-version"}, 0, "KoSIT configuration"},
	}
	for _, c := range cases {
		var out, errOut bytes.Buffer
		args := append([]string{"-cache", cache}, c.args...)
		if status := run(args, &out, &errOut); status != c.status {
			t.Errorf("%v: status %d, want %d; %s%s", c.args, status, c.status, out.String(), errOut.String())
		}
		if !strings.Contains(out.String(), c.out) {
			t.Errorf("%v: output %q lacks %q", c.args, out.String(), c.out)
		}
	}
}

func TestRunQuietHidesAccepted(t *testing.T) {
	var out bytes.Buffer
	run([]string{"-cache", t.TempDir(), "-q", corpus + "xrechnung-ubl-invoice-standard.xml"}, &out, &out)
	if out.Len() != 0 {
		t.Errorf("quiet output %q", out.String())
	}
}

func TestRunJSON(t *testing.T) {
	var out, errOut bytes.Buffer
	status := run([]string{"-cache", t.TempDir(), "-json", corpus + "xrechnung-cii-wrong-total.xml"}, &out, &errOut)
	var results []struct {
		File   string `json:"file"`
		State  string `json:"state"`
		Format string `json:"format"`
		Issues []struct {
			RuleID string `json:"ruleId"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatal(err, out.String())
	}
	if status != 1 || len(results) != 1 || results[0].State != "invalid" || results[0].Format != "xrechnung-cii" || len(results[0].Issues) == 0 {
		t.Errorf("status %d, results %+v", status, results)
	}
}
