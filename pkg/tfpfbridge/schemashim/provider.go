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

package schemashim

import (
	"context"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"

	pfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfpfbridge/pfutils"
)

type schemaOnlyProvider struct {
	ctx context.Context
	tf  pfprovider.Provider
}

var _ shim.Provider = (*schemaOnlyProvider)(nil)

func (p *schemaOnlyProvider) Schema() shim.SchemaMap {
	ctx := p.ctx
	schemaResp := &pfprovider.SchemaResponse{}
	p.tf.Schema(ctx, pfprovider.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		panic("GetSchema returned error diags")
	}
	return newSchemaMap(pfutils.FromProviderSchema(schemaResp.Schema))
}

func (p *schemaOnlyProvider) ResourcesMap() shim.ResourceMap {
	resources, err := pfutils.GatherResources(context.TODO(), p.tf)
	if err != nil {
		panic(err)
	}
	return &schemaOnlyResourceMap{resources}
}

func (p *schemaOnlyProvider) DataSourcesMap() shim.ResourceMap {
	dataSources, err := pfutils.GatherDatasources(context.TODO(), p.tf)
	if err != nil {
		panic(err)
	}
	return &schemaOnlyDataSourceMap{dataSources}
}

func (p *schemaOnlyProvider) Validate(c shim.ResourceConfig) ([]string, []error) {
	panic("schemaOnlyProvider does not implement runtime operation Validate")
}

func (p *schemaOnlyProvider) ValidateResource(t string, c shim.ResourceConfig) ([]string, []error) {
	panic("schemaOnlyProvider does not implement runtime operation ValidateResource")
}

func (p *schemaOnlyProvider) ValidateDataSource(
	t string, c shim.ResourceConfig) ([]string, []error) {
	panic("schemaOnlyProvider does not implement runtime operation ValidateDataSource")
}

func (p *schemaOnlyProvider) Configure(c shim.ResourceConfig) error {
	panic("schemaOnlyProvider does not implement runtime operation Configure")
}

func (p *schemaOnlyProvider) Diff(t string, s shim.InstanceState,
	c shim.ResourceConfig) (shim.InstanceDiff, error) {
	panic("schemaOnlyProvider does not implement runtime operation Diff")
}

func (p *schemaOnlyProvider) Apply(t string, s shim.InstanceState,
	d shim.InstanceDiff) (shim.InstanceState, error) {
	panic("schemaOnlyProvider does not implement runtime operation Apply")
}

func (p *schemaOnlyProvider) Refresh(t string, s shim.InstanceState) (shim.InstanceState, error) {
	panic("schemaOnlyProvider does not implement runtime operation Refresh")
}

func (p *schemaOnlyProvider) ReadDataDiff(t string,
	c shim.ResourceConfig) (shim.InstanceDiff, error) {
	panic("schemaOnlyProvider does not implement runtime operation ReadDataDiff")
}

func (p *schemaOnlyProvider) ReadDataApply(t string,
	d shim.InstanceDiff) (shim.InstanceState, error) {
	panic("schemaOnlyProvider does not implement runtime operation ReadDataApply")
}

func (p *schemaOnlyProvider) Meta() interface{} {
	panic("schemaOnlyProvider does not implement runtime operation Meta")
}

func (p *schemaOnlyProvider) Stop() error {
	panic("schemaOnlyProvider does not implement runtime operation Stop")
}

func (p *schemaOnlyProvider) InitLogging() {
	panic("schemaOnlyProvider does not implement runtime operation InitLogging")
}

func (p *schemaOnlyProvider) NewDestroyDiff() shim.InstanceDiff {
	panic("schemaOnlyProvider does not implement runtime operation NewDestroyDiff")
}

func (p *schemaOnlyProvider) NewResourceConfig(object map[string]interface{}) shim.ResourceConfig {
	panic("schemaOnlyProvider does not implement runtime operation ReourceConfig")
}

func (p *schemaOnlyProvider) IsSet(v interface{}) ([]interface{}, bool) {
	panic("schemaOnlyProvider does not implement runtime operation IsSet")
}
