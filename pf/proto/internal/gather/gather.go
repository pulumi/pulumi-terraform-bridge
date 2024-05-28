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

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
)

type getSchema = func() *tfprotov6.GetProviderSchemaResponse

func Resources(p getSchema) pfutils.Resources { return resources{p} }

func DataSources(p getSchema) pfutils.DataSources { return datasources{p} }

type (
	resources   struct{ schema getSchema }
	dataSources struct{ schema getSchema }
)

func (r resources) All() []pfutils.TypeName {
	s := r.schema().ResourceSchemas
	arr := make([]pfutils.TypeName, 0, len(s))
	for k := range s {
		arr = append(arr, pfutils.TypeName(k))
	}
	return arr
}

func (r resources) Has(key pfutils.TypeName) bool {
	_, ok := r.schema().ResourceSchemas[string(key)]
	return ok
}

func (r resources) Schema(key pfutils.TypeName) pfutils.Schema {
	s, ok := r.schema().ResourceSchemas[string(key)]
	contract.Assertf(ok, "called Schema on a resource that does not exist")

	return schema{s}
}

func (r resources) Diagnostics(pfutils.TypeName) diag.Diagnostics {
	// It's not clear how to split diagnostics by resource
	return nil
}

func (r resources) AllDiagnostics() diag.Diagnostics {
	diags := r.schema().Diagnostics
	result := make([]diag.Diagnostic, len(diags))
	for i, v := range diags {
		result[i] = diagnostic{v}
	}
	return result
}

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
