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

// proto enables building a [shim.Provider] around a [tfprotov6.ProviderServer].
//
// It is intended to help with schema generation, and should not be used for "runtime"
// resource operations like [Provider.Apply], [Provider.Diff], etc.
//
// To view unsupported methods, see ./unsuported.go.
package proto

import (
	"context"
	"sync"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi-terraform-bridge/pf"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

var _ = pf.ShimProvider(Provider{})

// New projects a new [tfprotov6.ProviderServer] into a schema only [shim.Provider].
//
// Non-schema runtime methods will panic.
func New(ctx context.Context, server tfprotov6.ProviderServer) shim.Provider {
	return &Provider{
		ctx:    ctx,
		server: server,
		getSchema: sync.OnceValues(func() (*tfprotov6.GetProviderSchemaResponse, error) {
			return server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
		}),
	}
}

// Provider provides a shim from [tfprotov6.ProviderServer] to [shim.Provider].
//
// To create a Provider, use [New]. A zero value Provider will panic on most operations.
//
// As much as possible, Provider is designed to be a wrapper type. It is cheap to create
// and use because it doesn't call any methods on the underlying server until it needs to.
//
// Provider is part of an internal API and does not have any compatibility guarantees. It
// may be changed or removed in future updates.
type Provider struct {
	// The underlying server.
	server tfprotov6.ProviderServer

	// A cached GetSchema on the underlying server.
	getSchema func() (*tfprotov6.GetProviderSchemaResponse, error)

	// A cached copy of the ctx that the provider was originally created with.
	//
	// Used during Replace.
	ctx context.Context
}

// Get access to the underlying sever used in Provide
func (p Provider) Server(context.Context) (tfprotov6.ProviderServer, error) {
	return cachedSchemaProvider{p.server, p.getSchema}, nil
}

func (p Provider) Config(context.Context) (tftypes.Object, error) {
	v, err := p.getSchema()
	if err != nil {
		return tftypes.Object{}, err
	}
	return v.Provider.ValueType().(tftypes.Object), nil
}

type cachedSchemaProvider struct {
	tfprotov6.ProviderServer

	getSchema func() (*tfprotov6.GetProviderSchemaResponse, error)
}

func (p cachedSchemaProvider) GetProviderSchema(
	context.Context, *tfprotov6.GetProviderSchemaRequest,
) (*tfprotov6.GetProviderSchemaResponse, error) {
	return p.getSchema()
}

func (p Provider) Schema() shim.SchemaMap {
	v, err := p.getSchema()
	if err != nil {
		tfbridge.GetLogger(p.ctx).Error(err.Error())
		return nil
	}
	if provider := v.Provider; provider != nil {
		return blockMap{provider.Block}
	}
	return blockMap{&tfprotov6.SchemaBlock{}}
}

func (p Provider) ResourcesMap() shim.ResourceMap {
	v, err := p.getSchema()
	if err != nil {
		tfbridge.GetLogger(p.ctx).Error(err.Error())
		return nil
	}
	return resourceMap(v.ResourceSchemas)
}

func (p Provider) DataSourcesMap() shim.ResourceMap {
	v, err := p.getSchema()
	if err != nil {
		tfbridge.GetLogger(p.ctx).Error(err.Error())
		return nil
	}
	return resourceMap(v.DataSourceSchemas)
}
