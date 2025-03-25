// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package tfbridge

import (
	"context"

	ctyjson "github.com/hashicorp/go-cty/cty/json"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

type rawInstanceState struct {
	rawStateValue *[]byte
}

func (r *rawInstanceState) ID() string {
	panic("not implemented")
}

func (r *rawInstanceState) RawStateValue() *[]byte {
	return r.rawStateValue
}

func (r *rawInstanceState) Meta() map[string]interface{} {
	panic("not implemented")
}

func (r *rawInstanceState) Object(sch shim.SchemaMap) (map[string]interface{}, error) {
	panic("not implemented")
}

func (r *rawInstanceState) Type() string {
	panic("not implemented")
}

var _ = shim.Resource(runtimeStateResource{})

// runtimeStateResource is a shim.Resource that is used to parse the raw state of a resource.
// It is only intended for use with MakeTerraformState. Unrelated methods are not implemented.
type runtimeStateResource struct {
	schemaVersion int
	schema        shim.SchemaMap
}

func (r runtimeStateResource) Schema() shim.SchemaMap {
	return r.schema
}

func (r runtimeStateResource) SchemaVersion() int {
	return r.schemaVersion
}

func (r runtimeStateResource) Importer() shim.ImportFunc {
	panic("not implemented")
}

func (r runtimeStateResource) DeprecationMessage() string {
	panic("not implemented")
}

func (r runtimeStateResource) Timeouts() *shim.ResourceTimeout {
	panic("not implemented")
}

func (r runtimeStateResource) InstanceState(
	id string, object, meta map[string]interface{},
) (shim.InstanceState, error) {
	if _, gotID := object["id"]; !gotID && id != "" {
		copy := map[string]interface{}{}
		for k, v := range object {
			copy[k] = v
		}
		copy["id"] = id
		object = copy
	}

	original := schema.HCL2ValueFromConfigValue(object)
	jsonState, err := ctyjson.Marshal(original, original.Type())
	if err != nil {
		return nil, err
	}
	return &rawInstanceState{
		rawStateValue: &jsonState,
	}, nil
}

func (r runtimeStateResource) DecodeTimeouts(config shim.ResourceConfig) (*shim.ResourceTimeout, error) {
	panic("not implemented")
}

func parseRawResourceState(
	ctx context.Context,
	pulumiResourceInfo *tfbridge.ResourceInfo,
	// TODO: generate the shim.SchemaMap from the tfprotov6.Schema?
	schemaMap shim.SchemaMap,
	tfResourceName string,
	resID resource.ID,
	schemaVersion int,
	props resource.PropertyMap,
) (*[]byte, error) {
	shimRes := runtimeStateResource{
		schemaVersion: schemaVersion,
		schema:        schemaMap,
	}

	instanceState, err := tfbridge.MakeTerraformStateWithOpts(
		ctx,
		tfbridge.Resource{
			Schema: pulumiResourceInfo,
			TF:     shimRes,
			TFName: tfResourceName,
		},
		resID.String(),
		props,
		tfbridge.MakeTerraformStateOptions{
			DefaultZeroSchemaVersion: true,
		},
	)
	if err != nil {
		return nil, err
	}

	rawStateValue := instanceState.(*rawInstanceState).rawStateValue

	return rawStateValue, nil
}
