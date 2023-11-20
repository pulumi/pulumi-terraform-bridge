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

package tfbridge

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/opentracing/opentracing-go"

	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/convert"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/defaults"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	rprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// CheckConfig validates the configuration for this resource provider.
func (p *provider) CheckConfigWithContext(
	ctx context.Context,
	urn resource.URN,
	_ resource.PropertyMap, // olds aka priorState, not used currently
	inputs resource.PropertyMap, // aka news
	_ bool, // a flag that is always true, historical artifact, ignore here
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	ctx = p.initLogging(ctx, p.logSink, urn)

	checkConfigSpan, ctx := opentracing.StartSpanFromContext(ctx, "pf.CheckConfig",
		opentracing.Tag{Key: "provider", Value: p.info.Name},
		opentracing.Tag{Key: "version", Value: p.version.String()},
		opentracing.Tag{Key: "inputs", Value: resource.NewObjectProperty(inputs).String()},
		opentracing.Tag{Key: "urn", Value: string(urn)},
	)
	defer checkConfigSpan.Finish()

	// Transform news to apply Pulumi-level defaults.
	news := defaults.ApplyDefaultInfoValues(ctx, defaults.ApplyDefaultInfoValuesArgs{
		SchemaMap:      p.schemaOnlyProvider.Schema(),
		SchemaInfos:    p.info.Config,
		PropertyMap:    inputs,
		ProviderConfig: inputs,
	})

	if !news.DeepEquals(inputs) {
		checkConfigSpan.SetTag("inputsWithPulumiDefaults", resource.NewObjectProperty(inputs).String())
	}

	// It is currently a breaking change to call PreConfigureCallback with unknown values. The user code does not
	// expect them and may panic.
	//
	// Currently we do not call it at all if there are any unknowns.
	//
	// See pulumi/pulumi-terraform-bridge#1087
	if !news.ContainsUnknowns() {
		if err := p.runPreConfigureCallback(ctx, news); err != nil {
			return nil, nil, err
		}
		if err := p.runPreConfigureCallbackWithLogger(ctx, news); err != nil {
			return nil, nil, err
		}
	}

	// Store for use in subsequent ApplyDefaultInfoValues.
	p.lastKnownProviderConfig = news

	var checkFailures []plugin.CheckFailure
	var err error

	if !p.info.SkipValidateProviderConfigForPluginFramework {
		checkFailures, err = p.validateProviderConfig(ctx, urn, news)
		if err != nil {
			return nil, nil, err
		}
	}

	// Ensure properties marked secret in the schema have secret values.
	secretNews := tfbridge.MarkSchemaSecrets(ctx, p.schemaOnlyProvider.Schema(), p.info.Config,
		resource.NewObjectProperty(news)).ObjectValue()

	checkConfigSpan.SetTag("checkedInputs", resource.NewObjectProperty(secretNews).String())
	return secretNews, checkFailures, nil
}

func (p *provider) runPreConfigureCallback(ctx context.Context, news resource.PropertyMap) error {
	if p.info.PreConfigureCallback == nil {
		return nil
	}
	span, ctx := opentracing.StartSpanFromContext(ctx, "pf.PreConfigureCallback")
	defer span.Finish()
	wc := &wrappedConfig{news}
	// NOTE: the user code may modify news in-place.
	return p.info.PreConfigureCallback(news, wc)
}

func (p *provider) runPreConfigureCallbackWithLogger(ctx context.Context, news resource.PropertyMap) error {
	if p.info.PreConfigureCallbackWithLogger == nil {
		return nil
	}
	span, ctx := opentracing.StartSpanFromContext(ctx, "pf.PreConfigureCallbackWithLogger")
	defer span.Finish()
	// Usually logSink is a HostClient; PreConfigureCallbackWithLogger type should have better been
	// expressed in terms of a diag.Sink.
	hc, ok := p.logSink.(*rprovider.HostClient)
	if !ok {
		hc = &rprovider.HostClient{}
	}
	wc := &wrappedConfig{news}
	// NOTE: the user code may modify news in-place.
	return p.info.PreConfigureCallbackWithLogger(ctx, hc, news, wc)
}

func (p *provider) validateProviderConfig(
	ctx context.Context,
	urn resource.URN,
	inputs resource.PropertyMap,
) ([]plugin.CheckFailure, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "pf.ValidateProviderConfig")
	defer span.Finish()

	config, err := convert.EncodePropertyMapToDynamic(p.configEncoder, p.configType, inputs)
	if err != nil {
		err = fmt.Errorf("cannot encode provider configuration to call ValidateProviderConfig: %w", err)
		return nil, err
	}
	req := &tfprotov6.ValidateProviderConfigRequest{
		Config: config,
	}
	resp, err := p.tfServer.ValidateProviderConfig(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error calling ValidateProviderConfig: %w", err)
	}

	// Note: according to the docs on resp.PrepareConfig for new providers it typically is equal to config passed in
	// ValidateProviderConfigRequest so the code here ignores it for now.

	remainingDiagnostics := []*tfprotov6.Diagnostic{}

	schemaMap := p.schemaOnlyProvider.Schema()
	schemaInfos := p.info.Config
	checkFailures := []plugin.CheckFailure{}

	for _, diag := range resp.Diagnostics {
		if k := p.detectCheckFailure(ctx, urn, true /*isProvider*/, schemaMap, schemaInfos, diag); k != nil {
			checkFailures = append(checkFailures, *k)
			continue
		}
		remainingDiagnostics = append(remainingDiagnostics, diag)
	}

	// Currently the convert.EncodePropertyMapToDynamic silently filters out keys that are not part of the schema,
	// but pkg/v3 bridge generated CheckFailures for these. Here is some extra code to compensate.
	for k := range inputs {
		// Skip reserved keys such as __defaults.
		if strings.HasPrefix(string(k), "__") {
			continue
		}
		// Ignoring version key as it seems to be special.
		if k == "version" || k == "pluginDownloadURL" {
			continue
		}
		n := tfbridge.PulumiToTerraformName(string(k), p.schemaOnlyProvider.Schema(), p.info.GetConfig())
		_, known := p.configType.AttributeTypes[n]
		if !known {
			pp := tfbridge.NewCheckFailurePath(schemaMap, schemaInfos, n)
			isProvider := true
			checkFailure := tfbridge.NewCheckFailure(
				tfbridge.InvalidKey, "Invalid or unknown key", &pp, urn, isProvider,
				p.info.Name, schemaMap, schemaInfos)
			checkFailures = append(checkFailures, checkFailure)
		}
	}

	if err := p.processDiagnostics(remainingDiagnostics); err != nil {
		return nil, err
	}

	return checkFailures, nil
}

type wrappedConfig struct {
	config resource.PropertyMap
}

func (wc *wrappedConfig) IsSet(key string) bool {
	pk := resource.PropertyKey(key)
	_, isSet := wc.config[pk]
	return isSet
}

var _ shim.ResourceConfig = &wrappedConfig{}
