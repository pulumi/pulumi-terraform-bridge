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
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/propertyvalue"
	pulumidiag "github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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

	sink := pulumidiag.DefaultSink(io.Discard, io.Discard, pulumidiag.FormatOptions{
		Color: colors.Never,
	})

	schema, err := tfgen.GenerateSchema(info, sink)
	if err != nil {
		return nil, fmt.Errorf("tfgen.GenerateSchema failed: %w", err)
	}

	schemaBytes, err := json.MarshalIndent(schema, "", " ")
	if err != nil {
		return nil, fmt.Errorf("json.MarshalIndent(schema, ..) failed: %w", err)
	}

	prov := tfbridge.NewProvider(ctx, nil, pd.name, pd.version, info.P, info, schemaBytes)

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

func (pd *pulumiDriver) writeYAML(t T, workdir string, tfConfig any) {
	res := pd.shimProvider.ResourcesMap().Get(pd.tfResourceName)
	schema := res.Schema()
	pConfig, err := pd.convertConfigToPulumi(schema, nil, pd.objectType, tfConfig)
	require.NoErrorf(t, err, "convertConfigToPulumi failed")

	// TODO[pulumi/pulumi-terraform-bridge#1864]: schema secrets may be set by convertConfigToPulumi.
	pConfig = propertyvalue.RemoveSecrets(resource.NewObjectProperty(pConfig)).ObjectValue()

	// This is a bit of a leap of faith that serializing PropertyMap to YAML in this way will yield valid Pulumi
	// YAML. This probably needs refinement.
	yamlProperties := pConfig.Mappable()

	data := map[string]any{
		"name":    "project",
		"runtime": "yaml",
		"resources": map[string]any{
			"example": map[string]any{
				"type":       pd.pulumiResourceToken,
				"properties": yamlProperties,
			},
		},
		"backend": map[string]any{
			"url": "file://./data",
		},
	}
	b, err := yaml.Marshal(data)
	require.NoErrorf(t, err, "marshaling Pulumi.yaml")
	t.Logf("\n\n%s", b)
	p := filepath.Join(workdir, "Pulumi.yaml")
	err = os.WriteFile(p, b, 0600)
	require.NoErrorf(t, err, "writing Pulumi.yaml")
}

func (pd *pulumiDriver) convertConfigToPulumi(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
	objectType *tftypes.Object,
	tfConfig any,
) (resource.PropertyMap, error) {
	var v *tftypes.Value

	switch tfConfig := tfConfig.(type) {
	case tftypes.Value:
		v = &tfConfig
		if objectType == nil {
			ty := v.Type().(tftypes.Object)
			objectType = &ty
		}
	case *tftypes.Value:
		v = tfConfig
		if objectType == nil {
			ty := v.Type().(tftypes.Object)
			objectType = &ty
		}
	default:
		if objectType == nil {
			t := convert.InferObjectType(schemaMap, nil)
			objectType = &t
		}
		bytes, err := json.Marshal(tfConfig)
		if err != nil {
			return nil, err
		}
		// Knowingly using a deprecated function so we can connect back up to tftypes.Value; if this disappears
		// it should not be prohibitively difficult to rewrite or vendor.
		//
		//nolint:staticcheck
		value, err := tftypes.ValueFromJSON(bytes, *objectType)
		if err != nil {
			return nil, err
		}
		v = &value
	}

	decoder, err := convert.NewObjectDecoder(convert.ObjectSchema{
		SchemaMap:   schemaMap,
		SchemaInfos: schemaInfos,
		Object:      objectType,
	})
	if err != nil {
		return nil, err
	}

	// There is not yet a way to opt out of marking schema secrets, so the resulting map might have secrets marked.
	pm, err := convert.DecodePropertyMap(context.Background(), decoder, *v)
	if err != nil {
		return nil, err
	}
	return pm, nil
}
