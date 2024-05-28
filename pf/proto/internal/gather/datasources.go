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

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/pfutils"
)

func DataSources(p getSchema) pfutils.Resources { return dataSources{p} }

type dataSources struct{ schema getSchema }

func (d dataSources) All() []pfutils.TypeName {
	res := make([]pfutils.TypeName, 0, len(d.schema().DataSourceSchemas))
	for k := range d.schema().DataSourceSchemas {
		res = append(res, pfutils.TypeName(k))
	}
	return res
}

func (d dataSources) Has(key pfutils.TypeName) bool {
	_, ok := d.schema().DataSourceSchemas[string(key)]
	return ok
}

func (d dataSources) Schema(key pfutils.TypeName) pfutils.Schema {
	v, ok := d.schema().DataSourceSchemas[string(key)]
	contract.Assertf(ok, "datasources missing %s", key)
	return schema{v}
}

func (d dataSources) Diagnostics(pfutils.TypeName) diag.Diagnostics { return nil }

func (d dataSources) AllDiagnostics() diag.Diagnostics {
	// TODO This includes *all* diagnostics, there is no clear way to limit it to
	// datasource diagnostics. Since [resources.AllDiagnostics] is implemented the
	// same way, this may lead to duplicates.
	diags := d.schema().Diagnostics
	result := make([]diag.Diagnostic, len(diags))
	for i, v := range diags {
		result[i] = diagnostic{v}
	}
	return result
}
