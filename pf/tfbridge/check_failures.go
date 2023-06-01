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
	"fmt"
	"strings"

	"github.com/agext/levenshtein"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
)

type checkFailureReason int64

const (
	miscFailure checkFailureReason = 0
	missingKey  checkFailureReason = 1
	invalidKey  checkFailureReason = 2
)

type propertyPath struct {
	schemaPath walk.SchemaPath
	valuePath  string
}

func (pp propertyPath) Len() int {
	return len(pp.schemaPath)
}

func (pp propertyPath) TopLevelPropertyKey(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
) (resource.PropertyKey, bool) {
	if len(pp.schemaPath) == 0 {
		return "", false
	}
	s, ok := pp.schemaPath[0].(walk.GetAttrStep)
	if !ok {
		return "", false
	}
	n := tfbridge.TerraformToPulumiNameV2(s.Name, schemaMap, schemaInfos)
	return resource.PropertyKey(n), true
}

func formatCheckFailure(
	reasonType checkFailureReason,
	reason string,
	pp propertyPath,
	urn resource.URN,
	isProvider bool,
	configPrefix string,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
) plugin.CheckFailure {
	if reasonType == missingKey {
		if isProvider {
			return missingProviderKey(urn, configPrefix, pp, schemaMap)
		}
		return missingRequiredKey(pp, schemaMap)
	}

	if isProvider {
		return formatProviderCheckFailure(reasonType, reason, pp, urn, configPrefix, schemaMap, schemaInfos)
	}

	if pp.valuePath != "" {
		reason = fmt.Sprintf("%s. Examine values at '%s.%s'.", reason, urn.Name().String(), pp.valuePath)
	}

	return plugin.CheckFailure{
		Reason: reason,
		// Do not populate Property here as that changes the UX of how it displays in CLI. We may want to
		// consider this change at some point.
	}

}

func formatProviderCheckFailure(
	reasonType checkFailureReason,
	reason string,
	pp propertyPath,
	urn resource.URN,
	configPrefix string,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
) plugin.CheckFailure {
	reason = "could not validate provider configuration: " + reason
	if isExplicitProvider(urn) {
		return formatExplicitProviderCheckFailure(reason, pp, urn)
	}
	return formatDefaultProviderCheckFailure(reasonType, reason, pp, configPrefix, schemaMap, schemaInfos)
}

func formatExplicitProviderCheckFailure(reason string, pp propertyPath, urn resource.URN) plugin.CheckFailure {
	if pp.valuePath != "" && isExplicitProvider(urn) {
		reason = fmt.Sprintf("%s. Examine values at '%s.%s'.", reason, urn.Name().String(), pp.valuePath)
	}
	return plugin.CheckFailure{
		Reason: reason,
	}
}

func formatDefaultProviderCheckFailure(
	reasonType checkFailureReason,
	reason string,
	pp propertyPath,
	configPrefix string,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
) plugin.CheckFailure {
	getExpr := "pulumi config get " + pulumiConfigExpr(configPrefix, pp)
	reason = fmt.Sprintf("%s. Check `%s`.", reason, getExpr)
	if reasonType == invalidKey {
		if sugg := suggestedKeys(pp, schemaMap, schemaInfos); len(sugg) > 0 {
			quoted := []string{}
			for _, s := range sugg {
				quoted = append(quoted, fmt.Sprintf("`%s:%s`", configPrefix, string(s)))
			}
			reason = fmt.Sprintf("%s Did you mean %s?", reason, strings.Join(quoted, " or "))
		}
	}
	return plugin.CheckFailure{
		Reason: reason,
		// Do not populate Property here intentionally as currently CLI would display that with a default
		// provider URN as if it is a resource, which is confusing. Instead Reason contains instructions on how
		// to set the value via a `pulumi config` command.
	}
}

func suggestedKeys(
	pp propertyPath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*tfbridge.SchemaInfo,
) []resource.PropertyKey {
	// Only supported for top-level keys for now.
	if pp.Len() != 1 {
		return nil
	}
	k, ok := pp.TopLevelPropertyKey(schemaMap, schemaInfos)
	if !ok {
		return nil
	}
	var allKeys []resource.PropertyKey
	schemaMap.Range(func(key string, value shim.Schema) bool {
		translatedKey := tfbridge.TerraformToPulumiNameV2(key, schemaMap, schemaInfos)
		allKeys = append(allKeys, resource.PropertyKey(translatedKey))
		return true
	})
	similar := []resource.PropertyKey{}
	for _, c := range allKeys {
		if levenshtein.Distance(string(k), string(c), levenshtein.NewParams()) <= 2 {
			similar = append(similar, c)
		}
	}
	return similar
}

func missingDefaultProviderKey(configPrefix string, pp propertyPath, schemaMap shim.SchemaMap) plugin.CheckFailure {
	configSetCommand := "pulumi config set " + pulumiConfigExpr(configPrefix, pp)
	reason := fmt.Sprintf("Provider is missing a required configuration key, try `%s`", configSetCommand)
	desc := lookupDescription(pp, schemaMap)
	if desc != "" {
		reason += ": " + desc
	}
	return plugin.CheckFailure{
		Reason: reason,
		// Do not populate Property here intentionally as currently CLI would display that with a default
		// provider URN as if it is a resource, which is confusing. Instead Reason contains instructions on how
		// to set the value via a `pulumi config` command.
	}
}

func pulumiConfigExpr(configPrefix string, pp propertyPath) (expr string) {
	if pp.Len() > 1 {
		expr = fmt.Sprintf("--path %s:%s", configPrefix, pp.valuePath)
	} else {
		expr = fmt.Sprintf("%s:%s", configPrefix, pp.valuePath)
	}
	return
}

// Provider configuration can be using an explicit provider or the default provider, use a heuristic here based on URN,
// to detect the default provider.
func isExplicitProvider(urn resource.URN) bool {
	return urn != "" && !strings.Contains(urn.Name().String(), "default")
}

func missingProviderKey(
	urn resource.URN,
	configPrefix string,
	pp propertyPath,
	schemaMap shim.SchemaMap,
) plugin.CheckFailure {
	if isExplicitProvider(urn) {
		return missingRequiredKey(pp, schemaMap)
	}
	return missingDefaultProviderKey(configPrefix, pp, schemaMap)
}

func missingRequiredKey(pp propertyPath, schemaMap shim.SchemaMap) plugin.CheckFailure {
	reason := "Missing a required property"
	desc := lookupDescription(pp, schemaMap)
	if desc != "" {
		reason += ": " + desc
	}
	return plugin.CheckFailure{
		// Assuming nested properties "a.b" seem to report OK through the CLI although they are not technically
		// of type resource.PropertyKey.
		Property: resource.PropertyKey(pp.valuePath),
		Reason:   reason,
	}
}

func lookupDescription(pp propertyPath, schemaMap shim.SchemaMap) (desc string) {
	s, err := walk.LookupSchemaMapPath(pp.schemaPath, schemaMap)
	if err == nil && s != nil {
		// TF descriptions often have newlines in inopportune positions. This makes them present a
		// little better in our console output.
		desc = strings.ReplaceAll(s.Description(), "\n", " ")
	}
	return
}
