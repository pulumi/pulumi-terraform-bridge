// Copyright 2025-2025, Pulumi Corporation.
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

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	eschema "github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
)

type testeph struct{}

var _ ephemeral.EphemeralResource = &testeph{}

func newTesteph() ephemeral.EphemeralResource {
	return &testeph{}
}

func (*testeph) schema() eschema.Schema {
	return eschema.Schema{
		Description: `
testbridge_testeph resource is built to facilitate testing ephemeral resources in the Pulumi bridge.

It emulates cloud state by storing the state in a binary file identified, with location configured by the statedir
attribute.
`,
		Attributes: map[string]eschema.Attribute{
			"id": eschema.StringAttribute{
				Computed: true,
			},
			"statedir": eschema.StringAttribute{
				Required:    true,
				Description: "Dir to store pseudo-cloud state in.",
			},
		},
	}
}

func (e *testeph) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_testeph"
}

func (e *testeph) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = e.schema()
}

func (e *testeph) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var statedir string
	diags0 := req.Config.GetAttribute(ctx, path.Root("statedir"), &statedir)
	resp.Diagnostics.Append(diags0...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceID, err := e.freshID(statedir)
	if err != nil {
		resp.Diagnostics.AddError("testres.freshID", err.Error())
		return
	}

	cloudStateFile := e.cloudStateFile(statedir, resourceID)
	if _, gotState, err := e.readCloudState(ctx, cloudStateFile); gotState && err == nil {
		resp.Diagnostics.AddError("testbridge_testres.Create found unexpected pseudo-cloud state",
			cloudStateFile)
	}

	// Copy plan to state.
	resp.Result.Raw = req.Config.Raw.Copy()

	// Set id computed by the provider.
	diags2 := resp.Result.SetAttribute(ctx, path.Root("id"), resourceID)
	resp.Diagnostics.Append(diags2...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := e.writeCloudState(ctx, cloudStateFile, resp.Result); err != nil {
		resp.Diagnostics.AddError("testbridge_testres.Create cannot write pseudo-cloud state",
			err.Error())
	}

	setJSON := func(key string, value string) {
		data, err := json.Marshal(value)
		if err != nil {
			resp.Diagnostics.AddError("testbridge_testres.Open cannot marshal private state",
				err.Error())
			return
		}
		diags := resp.Private.SetKey(ctx, key, data)
		resp.Diagnostics.Append(diags...)
	}

	setJSON("statedir", statedir)
	setJSON("id", resourceID)
}

func (e *testeph) Close(ctx context.Context, req ephemeral.CloseRequest, resp *ephemeral.CloseResponse) {
	getJSON := func(key string) string {
		var value string
		data, diags := req.Private.GetKey(ctx, key)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return ""
		}
		if err := json.Unmarshal(data, &value); err != nil {
			resp.Diagnostics.AddError("testbridge_testres.Close cannot unmarshal private state",
				err.Error())
			return ""
		}
		return value
	}

	statedir := getJSON("statedir")
	if statedir == "" {
		return
	}
	resourceID := getJSON("id")
	if resourceID == "" {
		return
	}

	cloudStateFile := e.cloudStateFile(statedir, resourceID)
	if err := e.deleteCloudState(cloudStateFile); err != nil {
		resp.Diagnostics.AddError("testbridge_testres.Close cannot delete pseudo-cloud state",
			err.Error())
	}
}

func (e *testeph) freshID(statedir string) (string, error) {
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
		i, err = strconv.Atoi(string(f))
		if err != nil {
			return "", err
		}
	}

	if err := os.WriteFile(cF, []byte(fmt.Sprintf("%d", i+1)), 0o600); err != nil {
		return "", err
	}

	return fmt.Sprintf("%d", i), nil
}

func (e *testeph) cloudStateFile(statedir, resourceID string) string {
	return filepath.Join(statedir, fmt.Sprintf("%s.bin", resourceID))
}

func (e *testeph) deleteCloudState(file string) error {
	return os.Remove(file)
}

func (e *testeph) readCloudState(ctx context.Context, file string) (tfsdk.State, bool, error) {
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

func (e *testeph) writeCloudState(ctx context.Context, file string, state tfsdk.EphemeralResultData) error {
	stateBytes, err := e.stateToBytes(ctx, state)
	if err != nil {
		return err
	}
	return os.WriteFile(file, stateBytes, 0o600)
}

func (*testeph) stateToBytes(ctx context.Context, state tfsdk.EphemeralResultData) ([]byte, error) {
	typ := state.Schema.Type().TerraformType(ctx)
	dv, err := tfprotov6.NewDynamicValue(typ, state.Raw)
	return dv.MsgPack, err
}

func (e *testeph) bytesToState(ctx context.Context, raw []byte) (tfsdk.State, error) {
	schema := e.schema()
	dv := tfprotov6.DynamicValue{MsgPack: raw}
	typ := schema.Type().TerraformType(ctx)
	v, err := dv.Unmarshal(typ)
	return tfsdk.State{Raw: v, Schema: schema}, err
}
