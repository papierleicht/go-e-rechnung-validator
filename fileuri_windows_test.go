//go:build windows

package validator

import (
	"net/url"
	"strings"
	"testing"
)

func TestFileURIWindowsPath(t *testing.T) {
	parsed, err := url.Parse(fileURI(`C:\tmp\rules\facturx\sheet.xslt`))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "file" || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/C:/tmp/rules/facturx/") {
		t.Fatalf("unexpected file URI %s", parsed)
	}
}
