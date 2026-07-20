// Copyright 2016-2026, Pulumi Corporation.
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

package crosstests

import (
	"context"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-terraform-bridge/v3/internal/logging"
	pb "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfgen"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/tfcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

// FunctionResult holds the value a provider-defined function produced on each side of a
// [Function] cross-test.
type FunctionResult struct {
	// TF is the function's value as Terraform computed it, decoded from JSON.
	TF any

	// Pulumi is the bridged invoke's return, converted to plain values. Non-object
	// returns arrive under the bridge's "result" wrapper property.
	Pulumi map[string]any
}

// Function calls a provider-defined function through Terraform (an output evaluating
// hclCall) and through the bridged Pulumi provider (an Invoke of tok with args),
// returning both results so the caller can assert parity.
//
// hclCall and (tok, args) must express the same call: Terraform takes positional
// arguments while the bridged invoke takes the schema's multiArgumentInputs property
// names.
func Function(
	t *testing.T, prov *pb.Provider, hclCall string, tok string, args resource.PropertyMap,
) FunctionResult {
	SkipUnlessLinux(t)
	ctx := context.Background()

	// Terraform resolves provider::<name>::<fn> through the provider's local name,
	// which must be declared in required_providers.
	program := fmt.Sprintf(`
terraform {
  required_providers {
    %[1]s = {
      source = "hashicorp/%[1]s"
    }
  }
}

output "result" {
  value = %s
}
`, prov.TypeName, hclCall)
	driver := tfcheck.NewTfDriver(t, t.TempDir(), prov.TypeName, tfcheck.NewTFDriverOpts{
		V6Provider: prov,
	})
	driver.Write(t, program)
	plan, err := driver.Plan(t)
	require.NoError(t, err)
	require.NoError(t, driver.ApplyPlan(t, plan))
	tfResult := driver.GetOutputJSON(t, "result")

	p := prov.ToProviderInfo()
	schema, err := tfgen.GenerateSchema(ctx, tfgen.GenerateSchemaOptions{ProviderInfo: p})
	require.NoError(t, err)
	p.MetadataInfo = &info.Metadata{Path: "non-empty"}
	server, err := tfbridge.NewProviderServer(ctx, logging.NewTestingSink(t), p, tfbridge.ProviderMetadata{
		PackageSchema: schema.ProviderMetadata.PackageSchema,
	})
	require.NoError(t, err)

	marshaled, err := plugin.MarshalProperties(args, plugin.MarshalOptions{})
	require.NoError(t, err)
	resp, err := server.Invoke(ctx, &pulumirpc.InvokeRequest{Tok: tok, Args: marshaled})
	require.NoError(t, err)
	require.Empty(t, resp.Failures)
	ret, err := plugin.UnmarshalProperties(resp.Return, plugin.MarshalOptions{})
	require.NoError(t, err)

	return FunctionResult{TF: tfResult, Pulumi: ret.Mappable()}
}
