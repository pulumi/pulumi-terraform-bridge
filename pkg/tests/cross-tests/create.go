// Copyright 2016-2024, Pulumi Corporation.
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
package crosstests

import (
	"context"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tests/pulcheck"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func Create(
	t T, resource map[string]*schema.Schema, tfConfig map[string]cty.Value, puConfig resource.PropertyMap,
	options ...CreateOption,
) {
	var opts createOpts
	for _, f := range options {
		f(&opts)
	}

	type result struct {
		data *schema.ResourceData
		meta any
	}
	var tfResult, puResult result

	makeResource := func(writeTo *result) *schema.Resource {
		return &schema.Resource{
			Schema: resource,
			CreateContext: func(_ context.Context, rd *schema.ResourceData, meta any) diag.Diagnostics {
				*writeTo = result{rd, meta}
				rd.SetId("someid") // CreateContext must pick an ID
				return nil
			},
		}
	}

	tfwd := t.TempDir()
	tfd := newTFResDriver(t, tfwd, defProviderShortName, defRtype, makeResource(&tfResult))
	tfd.writePlanApply(t, resource, defRtype, "example", tfConfig, lifecycleArgs{}) // TODO: Make tfConfig nativly accept a map[string]cty.Value

	resMap := map[string]*schema.Resource{defRtype: makeResource(&puResult)}
	tfp := &schema.Provider{ResourcesMap: resMap}
	bridgedProvider := pulcheck.BridgedProvider(
		t, defProviderShortName, tfp,
		pulcheck.WithResourceInfo(map[string]*info.Resource{defRtype: opts.resourceInfo}),
	)
	pd := &pulumiDriver{
		name:                defProviderShortName,
		pulumiResourceToken: defRtoken,
		tfResourceName:      defRtype,
		objectType:          tc.ObjectType, // TODO: Make objectType nativly accept a resource.PropertyMap
	}
	yamlProgram := pd.generateYAML(t, bridgedProvider.P.ResourcesMap(), puConfig)

	pt := pulcheck.PulCheck(t, bridgedProvider, string(yamlProgram))

	pt.Up(t)

	assert.Equal(t, tfResult, puResult)
}

type createOpts struct {
	resourceInfo *info.Resource
}

type CreateOption func(*createOpts)

func CreateResourceInfo(info info.Resource) CreateOption {
	return func(o *createOpts) { o.resourceInfo = &info }
}
