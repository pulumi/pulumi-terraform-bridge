// Copyright 2016-2022, Pulumi Corporation.
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

package pfutils

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// Full resource type, including the provider type prefix and an underscore. For example,
// examplecloud_thing.
type TypeName string

// Represents all provider's resources pre-indexed by TypeName.
type Resources interface {
	All() []TypeName
	Has(TypeName) bool
	Schema(TypeName) tfsdk.Schema
	Diagnostics(TypeName) diag.Diagnostics
	AllDiagnostics() diag.Diagnostics
	Resource(TypeName) resource.Resource
}

// Collects all resources from prov and indexes them by TypeName.
func GatherResources(ctx context.Context, prov provider.Provider) (Resources, error) {
	provMetadata := provider.MetadataResponse{}

	if provWithMeta, ok := prov.(provider.ProviderWithMetadata); ok {
		provWithMeta.Metadata(ctx, provider.MetadataRequest{}, &provMetadata)
	}

	rs := resources{
		schemaByTypeName:      map[string]tfsdk.Schema{},
		resourceByTypeName:    map[string]func() resource.Resource{},
		diagnosticsByTypeName: map[string]diag.Diagnostics{},
	}

	for _, makeResource := range prov.Resources(ctx) {
		res := makeResource()

		resMeta := resource.MetadataResponse{}

		res.Metadata(ctx, resource.MetadataRequest{
			ProviderTypeName: provMetadata.TypeName,
		}, &resMeta)

		resSchema, diag := res.GetSchema(ctx)

		if diag.HasError() {
			errs := diag.Errors()
			err := fmt.Errorf(
				"Resource %s GetSchema() found %d errors. First error: %s",
				resMeta.TypeName,
				diag.ErrorsCount(),
				errs[0].Summary(),
			)
			return nil, err
		}

		diags := rs.diagnosticsByTypeName[resMeta.TypeName]
		diags.Append(diag...)
		rs.diagnosticsByTypeName[resMeta.TypeName] = diags

		rs.schemaByTypeName[resMeta.TypeName] = resSchema
		rs.resourceByTypeName[resMeta.TypeName] = makeResource
	}

	return &rs, nil
}

type resources struct {
	schemaByTypeName      map[string]tfsdk.Schema
	resourceByTypeName    map[string]func() resource.Resource
	diagnosticsByTypeName map[string]diag.Diagnostics
}

var _ Resources = (*resources)(nil)

func (r *resources) Has(name TypeName) bool {
	_, ok := r.schemaByTypeName[string(name)]
	return ok
}

func (r *resources) All() []TypeName {
	var result []TypeName
	for k := range r.schemaByTypeName {
		result = append(result, TypeName(k))
	}
	sort.SliceStable(result, func(i, j int) bool {
		return string(result[i]) < string(result[j])
	})
	return result
}

func (r *resources) Schema(name TypeName) tfsdk.Schema {
	return r.schemaByTypeName[string(name)]
}

func (r *resources) Diagnostics(name TypeName) diag.Diagnostics {
	return r.diagnosticsByTypeName[string(name)]
}

func (r *resources) Resource(name TypeName) resource.Resource {
	makeRes := r.resourceByTypeName[string(name)]
	return makeRes()
}

func (r *resources) AllDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	for _, name := range r.All() {
		diags.Append(r.Diagnostics(name)...)
	}
	return diags
}
