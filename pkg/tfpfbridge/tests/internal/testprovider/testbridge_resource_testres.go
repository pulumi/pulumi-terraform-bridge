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

package testprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
)

type testres struct{}

var _ resource.Resource = &testres{}

func newTestres() resource.Resource {
	return &testres{}
}

func (*testres) schema() tfsdk.Schema {
	return tfsdk.Schema{
		Description: `
testbridge_testres resource is built to facilitate testing the Pulumi bridge.

It emulates cloud state by storing the state in a binary file identified, with location configured by the statedir
attribute.

The CRUD model is as simple as possible. Update and Create replace the cloud state with the planned state, Delete
removes the cloud state, and Read copies it.
`,
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.UseStateForUnknown(),
				},
			},
			"statedir": {
				Type:        types.StringType,
				Required:    true,
				Description: "Dir to store pseudo-cloud state in.",
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.UseStateForUnknown(),
					resource.RequiresReplace(),
				},
			},
			"requiredInputString": {
				Type:     types.StringType,
				Required: true,
			},
			"optionalInputString": {
				Type:     types.StringType,
				Optional: true,
			},
			"requiredInputStringCopy": {
				Type:        types.StringType,
				Computed:    true,
				Description: "Computed as a copy of requiredInputString",
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.UseStateForUnknown(),
				},
			},
			"optionalInputStringCopy": {
				Type:        types.StringType,
				Computed:    true,
				Description: "Computed as a copy of optionalInputString",
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.UseStateForUnknown(),
					PropagatesNullFrom{"optionalInputString"},
				},
			},
			"optionalInputNumber": {
				Type:     types.NumberType,
				Optional: true,
			},
			"optionalInputNumberCopy": {
				Type:        types.NumberType,
				Computed:    true,
				Description: "Computed as a copy of optionalInputNumber",
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.UseStateForUnknown(),
					PropagatesNullFrom{"optionalInputNumber"},
				},
			},
			"optionalInputBool": {
				Type:     types.BoolType,
				Optional: true,
			},
			"optionalInputBoolCopy": {
				Type:        types.BoolType,
				Computed:    true,
				Description: "Computed as a copy of optionalInputBool",
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.UseStateForUnknown(),
					PropagatesNullFrom{"optionalInputBool"},
				},
			},
			"optionalInputStringList": {
				Type:     types.ListType{ElemType: types.StringType},
				Optional: true,
			},
			"optionalInputStringListCopy": {
				Type:        types.ListType{ElemType: types.StringType},
				Computed:    true,
				Description: "Computed as a copy of optionalInputStringList",
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.UseStateForUnknown(),
					PropagatesNullFrom{"optionalInputStringList"},
				},
			},
			"optionalInputStringMap": {
				Type:     types.MapType{ElemType: types.StringType},
				Optional: true,
			},
			"optionalInputStringMapCopy": {
				Type:        types.MapType{ElemType: types.StringType},
				Computed:    true,
				Description: "Computed as a copy of optionalInputStringMap",
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.UseStateForUnknown(),
					PropagatesNullFrom{"optionalInputStringMap"},
				},
			},
			"singleNestedAttr": {
				MarkdownDescription: "singleNestedAttr: tests SingleNestedAttribute support",
				Optional:            true,
				Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
					"description": {
						Optional: true,
						Type:     types.StringType,
					},
					"quantity": {
						Optional: true,
						Type:     types.Float64Type,
					},
				}),
			},
			"singleNestedAttrJSONCopy": {
				Type:        types.StringType,
				Computed:    true,
				Description: "Computed as a JSON-ified copy of singleNestedAttr input",
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.UseStateForUnknown(),
					PropagatesNullFrom{"singleNestedAttr"},
				},
			},
			// Example borrowed from https://github.com/fly-apps/terraform-provider-fly/blob/28438713f2bdf08dbd0aa2fae9d74baaca9845f1/internal/provider/machine_resource.go#L176
			"services": {
				MarkdownDescription: "services: tests ListNestedAttributes support",
				Optional:            true,
				Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
					"ports": {
						MarkdownDescription: "External ports and handlers",
						Required:            true,
						Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
							"port": {
								MarkdownDescription: "External port",
								Required:            true,
								Type:                types.Int64Type,
							},
							"handlers": {
								MarkdownDescription: "How the edge should process requests",
								Optional:            true,
								Type:                types.ListType{ElemType: types.StringType},
							},
						}),
					},
					"protocol": {
						MarkdownDescription: "network protocol",
						Required:            true,
						Type:                types.StringType,
					},
					// TODO internal_port gets mangled to internalPort by Pulumi renaming and does
					// not work end-to-end yet.
					"intport": {
						MarkdownDescription: "Port application listens on internally",
						Required:            true,
						Type:                types.Int64Type,
					},
				}),
			},
			"servicesJSONCopy": {
				Type:        types.StringType,
				Computed:    true,
				Description: "Computed as a JSON-ified copy of services input",
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.UseStateForUnknown(),
					PropagatesNullFrom{"services"},
				},
			},
		},
	}
}

func (e *testres) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_testres"
}

func (e *testres) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return e.schema(), nil
}

func (e *testres) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var statedir string
	diags0 := req.Plan.GetAttribute(ctx, path.Root("statedir"), &statedir)
	resp.Diagnostics.Append(diags0...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceId, err := e.freshID(statedir)
	if err != nil {
		resp.Diagnostics.AddError("testres.freshID", err.Error())
		return
	}

	cloudStateFile := e.cloudStateFile(statedir, resourceId)
	if _, gotState, err := e.readCloudState(ctx, cloudStateFile); gotState && err == nil {
		resp.Diagnostics.AddError("testbridge_testres.Create found unexpected pseudo-cloud state",
			cloudStateFile)
	}

	// Copy plan to state.
	resp.State.Raw = req.Plan.Raw.Copy()

	// Set id computed by the provider.
	diags2 := resp.State.SetAttribute(ctx, path.Root("id"), resourceId)
	resp.Diagnostics.Append(diags2...)
	if resp.Diagnostics.HasError() {
		return
	}

	e.refreshComputedFields(ctx, &resp.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := e.writeCloudState(ctx, cloudStateFile, resp.State); err != nil {
		resp.Diagnostics.AddError("testbridge_testres.Create cannot write pseudo-cloud state",
			err.Error())
	}
}

func (e *testres) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var statedir string
	diags0 := req.State.GetAttribute(ctx, path.Root("statedir"), &statedir)
	resp.Diagnostics.Append(diags0...)
	if resp.Diagnostics.HasError() {
		return
	}

	var id string
	diags1 := req.State.GetAttribute(ctx, path.Root("id"), &id)
	resp.Diagnostics.Append(diags1...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudStateFile := e.cloudStateFile(statedir, id)

	savedState, gotState, err := e.readCloudState(ctx, cloudStateFile)
	if err != nil {
		resp.Diagnostics.AddError("testbridge_testres.Update cannot read pseudo-cloud state",
			err.Error())
		return
	}
	if gotState {
		resp.State = savedState
	}
	// TODO set resp.Private
}

func (e *testres) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var statedir string
	diags0 := req.Plan.GetAttribute(ctx, path.Root("statedir"), &statedir)
	resp.Diagnostics.Append(diags0...)
	if resp.Diagnostics.HasError() {
		return
	}

	var id string
	diags1 := req.State.GetAttribute(ctx, path.Root("id"), &id)
	resp.Diagnostics.Append(diags1...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudStateFile := e.cloudStateFile(statedir, id)

	oldState, gotState, err := e.readCloudState(ctx, cloudStateFile)
	if err != nil {
		resp.Diagnostics.AddError("testbridge_testres.Update cannot read pseudo-cloud state",
			err.Error())
		return
	}
	if !gotState {
		resp.Diagnostics.AddError("testbridge_testres.Update error",
			"found no prior pseudo-cloud state")
		return
	}
	if !oldState.Raw.Equal(req.State.Raw) {
		resp.Diagnostics.AddError(
			"testbridge_testres.Update error",
			"called with a different State than it remembers")
		return
	}

	// Copy plan to state without changes.
	resp.State = tfsdk.State{
		Raw:    req.Plan.Raw,
		Schema: e.schema(),
	}

	e.refreshComputedFields(ctx, &resp.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := e.writeCloudState(ctx, cloudStateFile, resp.State); err != nil {
		resp.Diagnostics.AddError("testbridge_testres.Update cannot write pseudo-cloud state",
			err.Error())
	}
}

type copyDataOptions struct {
	outputProp string
	transform  func(interface{}) interface{}
}

func copyData[T any](ctx context.Context, diag *diag.Diagnostics, state *tfsdk.State, inputProp string, slot *T, opts copyDataOptions) bool {
	outputProp := inputProp + "Copy"
	if opts.outputProp != "" {
		outputProp = opts.outputProp
	}

	diag2 := state.GetAttribute(ctx, path.Root(inputProp), &slot)
	diag.Append(diag2...)
	if diag.HasError() {
		return false
	}

	var replacement interface{}
	if slot != nil {
		replacement = *slot
		if opts.transform != nil {
			replacement = opts.transform(replacement)
		}
	} else {
		// This seems needlessly complicated, but nil will not do, need a typed nil.
		attrib, diag3 := state.Schema.AttributeAtPath(ctx, path.Root(outputProp))
		diag.Append(diag3...)
		if diag3.HasError() {
			return false
		}
		typedNil := tftypes.NewValue(attrib.GetType().TerraformType(ctx), nil)
		attrv, err := attrib.GetType().ValueFromTerraform(ctx, typedNil)
		if err != nil {
			panic(err)
		}
		replacement = attrv
	}

	diag3 := state.SetAttribute(ctx, path.Root(outputProp), replacement)
	diag.Append(diag3...)
	if diag.HasError() {
		return false
	}
	return true
}

func (e *testres) refreshComputedFields(ctx context.Context, state *tfsdk.State, diag *diag.Diagnostics) {
	var requiredInputString string
	diag0 := state.GetAttribute(ctx, path.Root("requiredInputString"), &requiredInputString)
	diag.Append(diag0...)
	if diag.HasError() {
		return
	}
	diag1 := state.SetAttribute(ctx, path.Root("requiredInputStringCopy"), requiredInputString)
	diag.Append(diag1...)
	if diag.HasError() {
		return
	}

	var s *string
	if ok := copyData(ctx, diag, state, "optionalInputString", &s, copyDataOptions{}); !ok {
		return
	}

	var n *float64
	if ok := copyData(ctx, diag, state, "optionalInputNumber", &n, copyDataOptions{}); !ok {
		return
	}

	var b *bool
	if ok := copyData(ctx, diag, state, "optionalInputBool", &b, copyDataOptions{}); !ok {
		return
	}

	var sl *[]string
	if ok := copyData(ctx, diag, state, "optionalInputStringList", &sl, copyDataOptions{}); !ok {
		return
	}

	var sm *map[string]string
	if ok := copyData(ctx, diag, state, "optionalInputStringMap", &sm, copyDataOptions{}); !ok {
		return
	}

	jsonify := func(x interface{}) interface{} {
		b, err := json.Marshal(x)
		if err != nil {
			panic(err)
		}
		return string(b)
	}

	var services *[]ServiceModel
	if ok := copyData(ctx, diag, state, "services", &services, copyDataOptions{
		outputProp: "servicesJSONCopy",
		transform:  jsonify,
	}); !ok {
		return
	}

	var snaModel *SingleNestedAttrModel
	if ok := copyData(ctx, diag, state, "singleNestedAttr", &snaModel, copyDataOptions{
		outputProp: "singleNestedAttrJSONCopy",
		transform:  jsonify,
	}); !ok {
		return
	}

	if !state.Raw.IsFullyKnown() {
		panic(fmt.Sprintf(
			"Error in testres: resource computation should resolve all unknowns, but got %v",
			state.Raw))
	}
}

func (e *testres) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var statedir string
	diags0 := req.State.GetAttribute(ctx, path.Root("statedir"), &statedir)
	resp.Diagnostics.Append(diags0...)
	if resp.Diagnostics.HasError() {
		return
	}

	var id string
	diags1 := req.State.GetAttribute(ctx, path.Root("id"), &id)
	resp.Diagnostics.Append(diags1...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudStateFile := e.cloudStateFile(statedir, id)

	oldState, gotState, err := e.readCloudState(ctx, cloudStateFile)
	if err != nil {
		resp.Diagnostics.AddError("testbridge_testres.Delete cannot read pseudo-cloud state",
			err.Error())
		return
	}
	if !gotState {
		resp.Diagnostics.AddError(
			"testbridge_testres.Delete found no prior pseudo-cloud state", "")
		return
	}
	if !oldState.Raw.Equal(req.State.Raw) {
		resp.Diagnostics.AddError(
			"testbridge_testres.Delete called with a different State than it remembers", "")
		return
	}

	resp.State = e.nilState(ctx)

	if err := e.deleteCloudState(cloudStateFile); err != nil {
		resp.Diagnostics.AddError(
			"testbridge_testres.Delete failed to delete the pseudo-cloud state",
			err.Error())
		return
	}
}

func (e *testres) freshID(statedir string) (string, error) {
	mu := fsutil.NewFileMutex(filepath.Join(statedir, "testres.lock"))
	if err := mu.Lock(); err != nil {
		return "", err
	}
	defer func() {
		if err := mu.Unlock(); err != nil {
			panic(err)
		}
	}()

	cF := filepath.Join(statedir, "testres.counter")

	i := 0
	f, err := os.ReadFile(cF)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err == nil {
		var x []byte = f
		i, err = strconv.Atoi(string(x))
		if err != nil {
			return "", err
		}
	}

	if err := os.WriteFile(cF, []byte(fmt.Sprintf("%d", i+1)), 0700); err != nil {
		return "", err
	}

	return fmt.Sprintf("%d", i), nil
}

func (e *testres) cloudStateFile(statedir, resourceId string) string {
	return filepath.Join(statedir, fmt.Sprintf("%s.bin", resourceId))
}

func (e *testres) deleteCloudState(file string) error {
	return os.Remove(file)
}

func (e *testres) readCloudState(ctx context.Context, file string) (tfsdk.State, bool, error) {
	bytes, err := os.ReadFile(file)

	if err != nil && os.IsNotExist(err) {
		return tfsdk.State{}, false, nil
	}

	if err != nil {
		return tfsdk.State{}, false, err
	}

	state, err := e.bytesToState(ctx, bytes)
	return state, err == nil, err
}

func (e *testres) writeCloudState(ctx context.Context, file string, state tfsdk.State) error {
	stateBytes, err := e.stateToBytes(ctx, state)
	if err != nil {
		return err
	}
	return os.WriteFile(file, stateBytes, 0600)
}

func (*testres) stateToBytes(ctx context.Context, state tfsdk.State) ([]byte, error) {
	typ := state.Schema.Type().TerraformType(ctx)
	dv, err := tfprotov6.NewDynamicValue(typ, state.Raw)
	return dv.MsgPack, err
}

func (e *testres) bytesToState(ctx context.Context, raw []byte) (tfsdk.State, error) {
	schema := e.schema()
	dv := tfprotov6.DynamicValue{MsgPack: raw}
	typ := schema.Type().TerraformType(ctx)
	v, err := dv.Unmarshal(typ)
	return tfsdk.State{Raw: v, Schema: schema}, err
}

func (e *testres) nilState(ctx context.Context) tfsdk.State {
	typ := e.terraformType(ctx)
	return tfsdk.State{
		Raw:    tftypes.NewValue(typ, nil),
		Schema: e.schema(),
	}
}

func (e *testres) terraformType(ctx context.Context) tftypes.Type {
	return e.schema().Type().TerraformType(ctx)
}
