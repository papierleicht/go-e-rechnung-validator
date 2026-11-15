// Command e-rechnung-validate checks XRechnung and ZUGFeRD / Factur-X files
// (XML or PDF) against the official rules, without Java.
//
//	go install github.com/papierleicht/go-e-rechnung-validator/cmd/e-rechnung-validate@latest
//	e-rechnung-validate invoice.xml invoice.pdf
//	e-rechnung-validate -json invoices/*.xml
//
// Exit status: 0 all files accepted, 1 at least one file rejected or
// unsupported, 2 usage error or a validation that could not run.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	validator "github.com/papierleicht/go-e-rechnung-validator"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

type fileResult struct {
	File string `json:"file"`
	validator.Result
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("e-rechnung-validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	asJSON := flags.Bool("json", false, "print a JSON array with one result per file")
	quiet := flags.Bool("q", false, "print only rejected files and their blocking findings")
	version := flags.Bool("version", false, "print the module and rule bundle versions")
	cacheDir := flags.String("cache", "", "directory for the rule files (default: user cache directory)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: e-rechnung-validate [-json] [-q] [-cache dir] file.xml|file.pdf ...")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *version {
		printVersion(stdout)
		return 0
	}
	if flags.NArg() == 0 {
		flags.Usage()
		return 2
	}
	v, err := validator.New(validator.Options{CacheDir: *cacheDir})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	status := 0
	results := make([]fileResult, 0, flags.NArg())
	for _, path := range flags.Args() {
		r := v.ValidateFile(path)
		results = append(results, fileResult{path, r})
		switch {
		case r.State == validator.Error:
			status = 2
		case !r.Accepted() && status == 0:
			status = 1
		}
		if !*asJSON {
			printText(stdout, path, r, *quiet)
		}
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	return status
}

func printText(w io.Writer, path string, r validator.Result, quiet bool) {
	if quiet && r.Accepted() {
		return
	}
	mark := "✓"
	if !r.Accepted() {
		mark = "✗"
	}
	format := ""
	if r.Format != "" {
		format = " (" + r.Format + ")"
	}
	fmt.Fprintf(w, "%s %s: %s%s\n", mark, path, r.State, format)
	issues := r.Issues
	if quiet {
		issues = r.Errors()
	}
	for _, i := range issues {
		fmt.Fprintln(w, "  "+i.String())
	}
}

func printVersion(w io.Writer) {
	module := "(devel)"
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range append([]*debug.Module{&info.Main}, info.Deps...) {
			if dep.Path == "github.com/papierleicht/go-e-rechnung-validator" && dep.Version != "" {
				module = dep.Version
			}
		}
	}
	b := validator.Rules()
	fmt.Fprintf(w, "go-e-rechnung-validator %s\nXRechnung %s (KoSIT configuration %s)\nFactur-X %s / ZUGFeRD %s\nrule bundle sha256 %s (%d files)\n",
		module, b.XRechnung, b.KoSITConfiguration, b.FacturX, b.ZUGFeRD, b.SHA256, b.Files)
}
