// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gather

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

type getSchema = func() *tfprotov6.GetProviderSchemaResponse

type diagnostic struct{ d *tfprotov6.Diagnostic }

func (d diagnostic) Severity() diag.Severity {
	switch d.d.Severity {
	case tfprotov6.DiagnosticSeverityError:
		return diag.SeverityError
	case tfprotov6.DiagnosticSeverityWarning:
		return diag.SeverityWarning
	default:
		return diag.SeverityInvalid
	}
}

func (d diagnostic) Summary() string { return d.d.Summary }

func (d diagnostic) Detail() string { return d.d.Detail }

func (d diagnostic) Equal(other diag.Diagnostic) bool {
	// Since [diag.Diagnostic] is a pure data carrier, we test equality by comparing
	// each "field".
	return d.Severity() == other.Severity() &&
		d.Summary() == other.Summary() &&
		d.Detail() == other.Detail()
}

func deprecated(isDeprecated bool) string {
	if isDeprecated {
		return "Deprecated"
	}
	return ""
}
