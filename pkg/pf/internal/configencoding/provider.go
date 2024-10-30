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

package configencoding

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	pl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/plugin"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

type ProviderWithContextAndConfig interface {
	pl.ProviderWithContext

	Attach(address string) error

	// GetConfigEncoding supplies the wrapping [Provider] with the (possibly unstable)
	// encoding used by the wrapped ProviderWithContextAndConfig.
	//
	// This is necessary since dynamic providers change their encoding in response to
	// gRPC calls.
	GetConfigEncoding(context.Context) *tfbridge.ConfigEncoding
}

type provider[T any] struct{ ProviderWithContextAndConfig }

type Provider[T any] interface {
	pl.ProviderWithContext

	Unwrap() T
}

// New creates a new [plugin.Provider] that wraps prov.
//
// The returned provider will correctly handle JSON encoded property values according to
// the passed encoding.
func New[T ProviderWithContextAndConfig](prov T) Provider[T] {
	return &provider[T]{prov}
}

// Unwrap returns the original [pl.ProviderWithContext] used to create the [Provider].
func (p *provider[T]) Unwrap() T { return p.ProviderWithContextAndConfig.(T) }

func (p *provider[T]) CheckConfigWithContext(
	ctx context.Context, urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	encoding := p.GetConfigEncoding(ctx)
	olds, err := encoding.UnfoldProperties(olds)
	if err != nil {
		return nil, nil, err
	}
	news, err = encoding.UnfoldProperties(news)
	if err != nil {
		return nil, nil, err
	}

	props, failures, err := p.ProviderWithContextAndConfig.
		CheckConfigWithContext(ctx, urn, olds, news, allowUnknowns)
	if err != nil {
		return nil, nil, err
	}
	props, err = encoding.FoldProperties(props)
	if err != nil {
		return nil, nil, err
	}

	return props, failures, err
}

func (p *provider[T]) DiffConfigWithContext(
	ctx context.Context, urn resource.URN, oldInputs, olds, news resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string) (plugin.DiffResult, error) {
	encoding := p.GetConfigEncoding(ctx)
	oldInputs, err := encoding.UnfoldProperties(oldInputs)
	if err != nil {
		return plugin.DiffConfigResponse{}, err
	}

	olds, err = encoding.UnfoldProperties(olds)
	if err != nil {
		return plugin.DiffConfigResponse{}, err
	}

	news, err = encoding.UnfoldProperties(news)
	if err != nil {
		return plugin.DiffConfigResponse{}, err
	}

	return p.ProviderWithContextAndConfig.DiffConfigWithContext(
		ctx, urn, oldInputs, olds, news, allowUnknowns, ignoreChanges)
}

func (p *provider[T]) ConfigureWithContext(ctx context.Context, inputs resource.PropertyMap) error {
	inputs, err := p.GetConfigEncoding(ctx).UnfoldProperties(inputs)
	if err != nil {
		return err
	}

	return p.ProviderWithContextAndConfig.ConfigureWithContext(ctx, inputs)
}
