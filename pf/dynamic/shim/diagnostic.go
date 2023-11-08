package shim

import (
	"github.com/opentofu/opentofu/internal/tfdiags"
)

type Diagnostic = tfdiags.Diagnostic

type Severity = tfdiags.Severity

const (
	SeverityError   Severity = 'E'
	SeverityWarning Severity = 'W'
)

type DiagDescription = tfdiags.Description

type DiagSource = tfdiags.Source

var GetAttribute = tfdiags.GetAttribute

//func GetAttribute(d Diagnostic) cty.Path {
