package validator

import (
	"fmt"
	"strings"
)

// State is the overall verdict of a validation.
type State string

const (
	// Valid: schema and all rules pass.
	Valid State = "valid"
	// Warning: no errors, but at least one warning or information message.
	// The document is accepted.
	Warning State = "warning"
	// Invalid: the document violates the schema or an error-level rule.
	Invalid State = "invalid"
	// Unsupported: the profile or syntax has no bundled rule set, or a PDF
	// contains no unambiguous invoice XML.
	Unsupported State = "unsupported"
	// Error: the validation itself failed (unreadable input, timeout,
	// engine failure). Nothing is known about the document.
	Error State = "error"
)

// Stages name the step that produced an issue.
const (
	StageSyntax  = "syntax"  // XML not well-formed or forbidden DOCTYPE
	StagePDF     = "pdf"     // invoice XML could not be taken from the PDF
	StageProfile = "profile" // no rule set for this profile and syntax
	StageXSD     = "xsd"     // XML schema
	StageRules   = "rules"   // Schematron business rules
	StageEngine  = "engine"  // rule compilation, transformation or timeout
)

// Issue is one finding. RuleID is the official rule identifier for the
// "rules" stage (for example BR-DE-15 or PEPPOL-EN16931-R001) and a short code
// for the other stages.
type Issue struct {
	Stage    string `json:"stage"`
	RuleID   string `json:"ruleId"`
	Severity string `json:"severity"` // fatal, error, warning or information
	Message  string `json:"message"`
	Location string `json:"location,omitempty"` // XPath of the offending node
}

func (i Issue) String() string {
	s := fmt.Sprintf("%s %s [%s] %s", i.Severity, i.RuleID, i.Stage, strings.Join(strings.Fields(i.Message), " "))
	if i.Location != "" {
		s += " (" + i.Location + ")"
	}
	return s
}

// Blocking reports whether the issue rejects the document.
func (i Issue) Blocking() bool { return i.Severity == "fatal" || i.Severity == "error" }

// Result is the outcome of one validation.
type Result struct {
	State  State   `json:"state"`
	Format string  `json:"format,omitempty"` // Format.ID of the detected profile
	Issues []Issue `json:"issues"`
}

// Accepted reports whether the document passes: valid, or valid with warnings.
func (r Result) Accepted() bool { return r.State == Valid || r.State == Warning }

// RuleIDs returns the rule IDs of all issues in report order.
func (r Result) RuleIDs() []string {
	ids := make([]string, len(r.Issues))
	for n, i := range r.Issues {
		ids[n] = i.RuleID
	}
	return ids
}

// Errors returns the blocking issues.
func (r Result) Errors() []Issue {
	var out []Issue
	for _, i := range r.Issues {
		if i.Blocking() {
			out = append(out, i)
		}
	}
	return out
}

func failed(state State, stage, code string, err error) Result {
	return Result{State: state, Issues: []Issue{{Stage: stage, RuleID: code, Severity: "error", Message: fmt.Sprint(err)}}}
}
