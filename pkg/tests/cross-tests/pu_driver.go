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

// Driver code for running tests against an in-process bridged provider under Pulumi CLI.
package crosstests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	pulumidiag "github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

type pulumiDriver struct {
	name                string
	version             string
	shimProvider        shim.Provider
	pulumiResourceToken string
	tfResourceName      string
	objectType          *tftypes.Object
}


func startPulumiProvider(ctx context.Context, name, version string, providerInfo tfbridge.ProviderInfo) (*rpcutil.ServeHandle, error) {
	sink := pulumidiag.DefaultSink(io.Discard, io.Discard, pulumidiag.FormatOptions{
		Color: colors.Never,
	})

	schema, err := tfgen.GenerateSchema(providerInfo, sink)
	if err != nil {
		return nil, fmt.Errorf("tfgen.GenerateSchema failed: %w", err)
	}

	schemaBytes, err := json.MarshalIndent(schema, "", " ")
	if err != nil {
		return nil, fmt.Errorf("json.MarshalIndent(schema, ..) failed: %w", err)
	}

	prov := tfbridge.NewProvider(ctx, nil, name, version, providerInfo.P, providerInfo, schemaBytes)

	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, prov)
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("rpcutil.ServeWithOptions failed: %w", err)
	}

	return &handle, nil
}

func (pd *pulumiDriver) providerInfo() tfbridge.ProviderInfo {
	return tfbridge.ProviderInfo{
		Name: pd.name,
		P:    pd.shimProvider,

		Resources: map[string]*tfbridge.ResourceInfo{
			pd.tfResourceName: {
				Tok: tokens.Type(pd.pulumiResourceToken),
			},
		},
	}
}

func (pd *pulumiDriver) startPulumiProvider(ctx context.Context) (*rpcutil.ServeHandle, error) {
	info := pd.providerInfo()
	return startPulumiProvider(ctx, pd.name, pd.version, info)
}

func (pd *pulumiDriver) writeYAML(t T, workdir string, tfConfig any) {
	res := pd.shimProvider.ResourcesMap().Get(pd.tfResourceName)
	schema := res.Schema()

	data, err := generateYaml(schema, pd.pulumiResourceToken, pd.objectType, tfConfig)
	require.NoErrorf(t, err, "generateYaml")

	b, err := yaml.Marshal(data)
	require.NoErrorf(t, err, "marshaling Pulumi.yaml")
	t.Logf("\n\n%s", b)
	p := filepath.Join(workdir, "Pulumi.yaml")
	err = os.WriteFile(p, b, 0o600)
	require.NoErrorf(t, err, "writing Pulumi.yaml")
}
