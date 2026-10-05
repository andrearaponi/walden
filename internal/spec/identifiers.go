package spec

import "regexp"

// Identifier syntax shared by validation and consolidation, so both read the
// same requirement, criterion, NFR and constraint identifiers.
const (
	RequirementIDExpr = `R\d+`
	CriterionIDExpr   = `R\d+\.AC\d+`
	NFRIDExpr         = `NFR\d+`
	ConstraintIDExpr  = `C\d+`
)

// Patterns that find a backticked identifier anywhere in a document body.
// They report mentions as well as definitions; callers that need definitions
// only must anchor on definition lines.
var (
	RequirementHeaderPattern = regexp.MustCompile(`(?m)^### (` + RequirementIDExpr + `)\b`)
	CriterionIDPattern       = regexp.MustCompile("`(" + CriterionIDExpr + ")`")
	NFRIDPattern             = regexp.MustCompile("`(" + NFRIDExpr + ")`")
	ConstraintIDPattern      = regexp.MustCompile("`(" + ConstraintIDExpr + ")`")
	AnyIDPattern             = regexp.MustCompile("`((?:" + RequirementIDExpr + `(?:\.AC\d+)?)|(?:` + NFRIDExpr + ")|(?:" + ConstraintIDExpr + "))`")
)
