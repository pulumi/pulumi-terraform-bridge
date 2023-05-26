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
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/grpc/codes"

	rprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"github.com/agext/levenshtein"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/convert"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/defaults"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
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

	// Transform news to apply Pulumi-level defaults.
	news := defaults.ApplyDefaultInfoValues(ctx, defaults.ApplyDefaultInfoValuesArgs{
		SchemaMap:      p.schemaOnlyProvider.Schema(),
		SchemaInfos:    p.info.Config,
		PropertyMap:    inputs,
		ProviderConfig: inputs,
	})

	// It is currently a breaking change to call PreConfigureCallback with unknown values. The user code does not
	// expect them and may panic.
	//
	// Currently we do not call it at all if there are any unknowns.
	//
	// See pulumi/pulumi-terraform-bridge#1087
	if !news.ContainsUnknowns() {
		wc := &wrappedConfig{news}

		if p.info.PreConfigureCallback != nil {
			// NOTE: the user code may modify news in-place.
			validationErrors := p.info.PreConfigureCallback(news, wc)
			if validationErrors != nil {
				return nil, nil, validationErrors
			}
		}

		if p.info.PreConfigureCallbackWithLogger != nil {
			// Usually logSink is a HostClient; PreConfigureCallbackWithLogger type should have better been
			// expressed in terms of a diag.Sink.
			hc, ok := p.logSink.(*rprovider.HostClient)
			if !ok {
				hc = &rprovider.HostClient{}
			}
			// NOTE: the user code may modify news in-place.
			validationErrors := p.info.PreConfigureCallbackWithLogger(ctx, hc, news, wc)
			if validationErrors != nil {
				return nil, nil, validationErrors
			}
		}
	}

	// Store for use in subsequent ApplyDefaultInfoValues.
	p.lastKnownProviderConfig = news

	missingKeys, checkFailures, err := p.validateProviderConfig(ctx, urn, news)
	if err != nil {
		return nil, nil, err
	}

	if len(missingKeys) > 0 {
		err := rpcerror.WithDetails(
			rpcerror.New(codes.InvalidArgument, "required configuration keys were missing"),
			&pulumirpc.ConfigureErrorMissingKeys{MissingKeys: missingKeys})
		return nil, checkFailures, err
	}

	// Ensure propreties marked secret in the schema have secret values.
	secretNews := tfbridge.MarkSchemaSecrets(ctx, p.schemaOnlyProvider.Schema(), p.info.Config,
		resource.NewObjectProperty(news)).ObjectValue()

	return secretNews, checkFailures, nil
}

func (p *provider) validateProviderConfig(
	ctx context.Context,
	urn resource.URN,
	inputs resource.PropertyMap,
) ([]*pulumirpc.ConfigureErrorMissingKeys_MissingKey, []plugin.CheckFailure, error) {
	config, err := convert.EncodePropertyMapToDynamic(p.configEncoder, p.configType, inputs)
	if err != nil {
		err = fmt.Errorf("cannot encode provider configuration to call ValidateProviderConfig: %w", err)
		return nil, nil, err
	}
	req := &tfprotov6.ValidateProviderConfigRequest{
		Config: config,
	}
	resp, err := p.tfServer.ValidateProviderConfig(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("error calling ValidateProviderConfig: %w", err)
	}

	// Note: according to the docs on resp.PrepareConfig for new providers it typically is equal to config passed in
	// ValidateProviderConfigRequest so the code here ignores it for now.

	missingKeys := []*pulumirpc.ConfigureErrorMissingKeys_MissingKey{}
	remainingDiagnostics := []*tfprotov6.Diagnostic{}

	schemaMap := p.schemaOnlyProvider.Schema()
	schemaInfos := p.info.Config
	checkFailures := []plugin.CheckFailure{}

	for _, diag := range resp.Diagnostics {
		if k := detectMissingKey(ctx, schemaMap, schemaInfos, diag); k != nil {
			missingKeys = append(missingKeys, k)
			continue
		}
		if cf, ok := p.detectCheckFailure(ctx, urn, true /*isProvider*/, schemaMap, schemaInfos, diag); ok {
			checkFailures = append(checkFailures, cf)
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
		if k == "version" {
			continue
		}
		n := tfbridge.PulumiToTerraformName(string(k), p.schemaOnlyProvider.Schema(), p.info.GetConfig())
		_, known := p.configType.AttributeTypes[n]
		if !known {
			reason := p.formatFailureReason(ctx, urn, true /*isProvider*/, urn.Name().String(), schemaMap,
				schemaInfos, &tfprotov6.Diagnostic{
					Attribute: tftypes.NewAttributePath().WithAttributeName(n),
					Summary:   "Invalid or unknown key",
				})
			checkFailures = append(checkFailures, plugin.CheckFailure{
				Reason: reason,
			})
		}
	}

	if err := p.processDiagnostics(remainingDiagnostics); err != nil {
		return nil, nil, err
	}
	return missingKeys, checkFailures, nil
}

func detectMissingKey(
	ctx context.Context,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
	diag *tfprotov6.Diagnostic,
) *pulumirpc.ConfigureErrorMissingKeys_MissingKey {
	if diag.Summary != "Missing Configuration for Required Attribute" {
		return nil
	}
	if len(diag.Attribute.Steps()) < 1 {
		return nil
	}

	mk := pulumirpc.ConfigureErrorMissingKeys_MissingKey{}

	if diag.Attribute != nil {
		path, err := formatAttributePathAsPulumiPath(schemaMap, schemaInfos, diag.Attribute)
		if err != nil {
			tflog.Debug(ctx, fmt.Sprintf("detectMissingKey ignored an error: %v", err))
		} else {
			mk.Name = path
		}

		s, err := walk.LookupSchemaMapPath(attrPathToSchemaPath(diag.Attribute), schemaMap)
		if err == nil && s != nil {
			// TF descriptions often have newlines in inopportune positions. This makes them present a
			// little better in our console output.
			mk.Description = strings.ReplaceAll(s.Description(), "\n", " ")
		}
	}

	return &mk
}

func (p *provider) detectCheckFailure(
	ctx context.Context,
	urn resource.URN,
	isProvider bool,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
	diag *tfprotov6.Diagnostic,
) (plugin.CheckFailure, bool) {
	if diag.Attribute == nil || len(diag.Attribute.Steps()) < 1 {
		return plugin.CheckFailure{}, false
	}
	reason := p.formatFailureReason(ctx, urn, isProvider, urn.Name().String(), schemaMap, schemaInfos, diag)
	return plugin.CheckFailure{
		Reason: reason,
	}, true
}

func (p *provider) formatFailureReason(
	ctx context.Context,
	urn resource.URN,
	isProvider bool,
	prefix string,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
	diag *tfprotov6.Diagnostic,
) string {
	reason := diag.Summary
	if diag.Detail != "" {
		reason = fmt.Sprintf("%s. %s", reason, diag.Detail)
	}

	attributePath, err := formatAttributePathAsPulumiPath(schemaMap, schemaInfos, diag.Attribute)
	if err != nil {
		tflog.Debug(ctx, fmt.Sprintf("Ignoring error from formatAttributePathAsPulumiPath: %v", err))
	}

	if isProvider {
		// Provider configuration can be using an explicit provider or the default provider, use a heuristic
		// here based on URN, to detect the default provider.
		isExplicit := !strings.Contains(urn.Name().String(), "default")
		reason = fmt.Sprintf("could not validate provider configuration: %s", reason)
		if key, got := providerKey(p.info.Name, schemaMap, schemaInfos, diag.Attribute); !isExplicit && got {
			reason = fmt.Sprintf("%s. Check `pulumi config get %s`.", reason, key)

			// Try to find and suggest spelling corrections. Currently this only works for top-level keys.
			if strings.Contains(reason, "Invalid or unknown key") && len(diag.Attribute.Steps()) == 1 {
				if sugg := suggestedKeys(p.info.Name, key, schemaMap, schemaInfos); len(sugg) > 0 {
					quoted := []string{}
					for _, s := range sugg {
						quoted = append(quoted, fmt.Sprintf("`%s`", s))
					}
					reason = fmt.Sprintf("%s Did you mean %s?", reason,
						strings.Join(quoted, " or "))
				}
			}

		}
		if attributePath != "" && isExplicit {
			reason = fmt.Sprintf("%s. Examine values at '%s'.", reason, prefix+"."+attributePath)
		}
		return reason
	}

	if attributePath != "" {
		reason += fmt.Sprintf(". Examine values at '%s'.", prefix+"."+attributePath)
	}

	return reason
}

func suggestedKeys(
	module string,
	key string,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
) []string {

	var allKeys []string

	schemaMap.Range(func(key string, value shim.Schema) bool {
		p := tftypes.NewAttributePath().WithAttributeName(key)
		if k, ok := providerKey(module, schemaMap, schemaInfos, p); ok {
			allKeys = append(allKeys, k)
		}
		return true
	})

	similar := []string{}

	for _, k := range allKeys {
		if levenshtein.Distance(k, key, levenshtein.NewParams()) <= 2 {
			similar = append(similar, k)
		}
	}

	return similar
}

func providerKey(
	module string,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
	path *tftypes.AttributePath,
) (key string, got bool) {
	if len(path.Steps()) < 1 {
		return
	}
	step, ok := path.Steps()[0].(tftypes.AttributeName)
	if !ok {
		return
	}
	n := tfbridge.TerraformToPulumiNameV2(string(step), schemaMap, schemaInfos)
	return fmt.Sprintf("%s:%s", module, n), true
}

func formatAttributePathAsPulumiPath(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
	attrPath *tftypes.AttributePath,
) (string, error) {
	steps := attrPath.Steps()

	var buf bytes.Buffer
	for i, s := range steps {
		switch s := s.(type) {
		case tftypes.AttributeName:
			here := tftypes.NewAttributePathWithSteps(steps[0 : i+1])
			schPath := attrPathToSchemaPath(here)
			name, err := tfbridge.TerraformToPulumiNameAtPath(schPath, schemaMap, schemaInfos)
			if err != nil {
				return "", err
			}
			if i > 0 {
				fmt.Fprintf(&buf, ".")
			}
			fmt.Fprintf(&buf, "%s", name)
		case tftypes.ElementKeyInt:
			fmt.Fprintf(&buf, "[%d]", int64(s))
		case tftypes.ElementKeyString:
			fmt.Fprintf(&buf, "[%q]", string(s))
		case tftypes.ElementKeyValue:
			// Sets will be represented as lists in Pulumi; more could be done here to find the right index.
			fmt.Fprintf(&buf, "[?]")
		default:
			contract.Failf("Unhandled match case for tftypes.AttributePathStep")
		}
	}

	return buf.String(), nil
}

func attrPathToSchemaPath(attrPath *tftypes.AttributePath) walk.SchemaPath {
	p := walk.NewSchemaPath()
	for _, s := range attrPath.Steps() {
		switch s := s.(type) {
		case tftypes.AttributeName:
			p = p.GetAttr(string(s))
		case tftypes.ElementKeyInt, tftypes.ElementKeyString, tftypes.ElementKeyValue:
			p = p.Element()
		default:
			contract.Failf("Unhandled match case for tftypes.AttributePathStep")
		}
	}
	return p
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
