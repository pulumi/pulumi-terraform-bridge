// Copyright 2016-2023, Pulumi Corporation.
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

package plugin

import (
	"context"
	"io"

	p "github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// A version of Provider interface that is enhanced by giving access to the request Context.
type ProviderWithContext interface {
	io.Closer

	PkgWithContext(ctx context.Context) tokens.Package

	GetSchemaWithContext(ctx context.Context, version int) ([]byte, error)

	CheckConfigWithContext(ctx context.Context, urn resource.URN, olds, news resource.PropertyMap,
		allowUnknowns bool) (resource.PropertyMap, []p.CheckFailure, error)

	DiffConfigWithContext(ctx context.Context, urn resource.URN, olds, news resource.PropertyMap,
		allowUnknowns bool, ignoreChanges []string) (p.DiffResult, error)

	ConfigureWithContext(ctx context.Context, inputs resource.PropertyMap) error

	CheckWithContext(ctx context.Context, urn resource.URN, olds, news resource.PropertyMap,
		allowUnknowns bool, randomSeed []byte) (resource.PropertyMap, []p.CheckFailure, error)

	DiffWithContext(ctx context.Context, urn resource.URN, id resource.ID, olds resource.PropertyMap,
		news resource.PropertyMap, allowUnknowns bool, ignoreChanges []string) (p.DiffResult, error)

	CreateWithContext(ctx context.Context, urn resource.URN, news resource.PropertyMap, timeout float64,
		preview bool) (resource.ID, resource.PropertyMap, resource.Status, error)

	ReadWithContext(ctx context.Context, urn resource.URN, id resource.ID,
		inputs, state resource.PropertyMap) (p.ReadResult, resource.Status, error)

	UpdateWithContext(ctx context.Context, urn resource.URN, id resource.ID,
		olds resource.PropertyMap, news resource.PropertyMap, timeout float64,
		ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error)

	DeleteWithContext(ctx context.Context, urn resource.URN, id resource.ID,
		props resource.PropertyMap, timeout float64) (resource.Status, error)

	ConstructWithContext(ctx context.Context, info p.ConstructInfo, typ tokens.Type, name tokens.QName,
		parent resource.URN, inputs resource.PropertyMap,
		options p.ConstructOptions) (p.ConstructResult, error)

	InvokeWithContext(ctx context.Context, tok tokens.ModuleMember,
		args resource.PropertyMap) (resource.PropertyMap, []p.CheckFailure, error)

	StreamInvokeWithContext(
		ctx context.Context,
		tok tokens.ModuleMember,
		args resource.PropertyMap,
		onNext func(resource.PropertyMap) error) ([]p.CheckFailure, error)

	CallWithContext(ctx context.Context, tok tokens.ModuleMember, args resource.PropertyMap, info p.CallInfo,
		options p.CallOptions) (p.CallResult, error)

	GetPluginInfoWithContext(ctx context.Context) (workspace.PluginInfo, error)

	SignalCancellationWithContext(ctx context.Context) error

	GetMappingWithContext(ctx context.Context, key string) ([]byte, string, error)
}
