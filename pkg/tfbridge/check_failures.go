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

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/walk"
)

// Enumerates categories of failing checks. See [NewCheckFailure].
type CheckFailureReason int64

const (
	MiscFailure CheckFailureReason = 0
	MissingKey  CheckFailureReason = 1
	InvalidKey  CheckFailureReason = 2
)

// Identifies a path to the property that is failing checks. See [NewCheckFailure].
type CheckFailurePath struct {
	schemaPath  walk.SchemaPath
	valuePath   string
	schemaMap   shim.SchemaMap
	schemaInfos map[string]*SchemaInfo
}

func NewCheckFailurePath(schemaMap shim.SchemaMap, schemaInfos map[string]*SchemaInfo, prop string) CheckFailurePath {
	pulumiName := TerraformToPulumiNameV2(prop, schemaMap, schemaInfos)
	return CheckFailurePath{
		schemaPath:  walk.NewSchemaPath().GetAttr(prop),
		valuePath:   pulumiName,
		schemaMap:   schemaMap,
		schemaInfos: schemaInfos,
	}
}

func (p CheckFailurePath) Attribute(name string) CheckFailurePath {
	path := p.schemaPath.GetAttr(name)
	pulumiName, err := TerraformToPulumiNameAtPath(path, p.schemaMap, p.schemaInfos)
	if err != nil {
		pulumiName = name
	}
	return CheckFailurePath{
		schemaPath:  path,
		valuePath:   fmt.Sprintf("%s.%s", p.valuePath, pulumiName),
		schemaMap:   p.schemaMap,
		schemaInfos: p.schemaInfos,
	}
}

func (p CheckFailurePath) isMaxItemsOne() bool {
	s, i, err := LookupSchemas(p.schemaPath, p.schemaMap, p.schemaInfos)
	if err != nil {
		return false
	}
	return IsMaxItemsOne(s, i)
}

func (p CheckFailurePath) ListElement(n int64) CheckFailurePath {
	valuePath := p.valuePath
	if !p.isMaxItemsOne() {
		valuePath = fmt.Sprintf("%s[%d]", p.valuePath, n)
	}
	return CheckFailurePath{
		schemaPath:  p.schemaPath.Element(),
		valuePath:   valuePath,
		schemaMap:   p.schemaMap,
		schemaInfos: p.schemaInfos,
	}
}

func (p CheckFailurePath) MapElement(s string) CheckFailurePath {
	valuePath := p.valuePath
	if !p.isMaxItemsOne() {
		valuePath = fmt.Sprintf("%s[%q]", p.valuePath, s)
	}
	return CheckFailurePath{
		schemaPath:  p.schemaPath.Element(),
		valuePath:   valuePath,
		schemaMap:   p.schemaMap,
		schemaInfos: p.schemaInfos,
	}
}

func (p CheckFailurePath) SetElement() CheckFailurePath {
	// Sets will be represented as lists in Pulumi; more could be done here to find the right index.
	valuePath := p.valuePath
	if p.isMaxItemsOne() {
		valuePath = fmt.Sprintf("%s[?]", p.valuePath)
	}
	return CheckFailurePath{
		schemaPath:  p.schemaPath.Element(),
		valuePath:   valuePath,
		schemaMap:   p.schemaMap,
		schemaInfos: p.schemaInfos,
	}
}

func (pp CheckFailurePath) len() int {
	return len(pp.schemaPath)
}

func (pp CheckFailurePath) topLevelPropertyKey(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
) (resource.PropertyKey, bool) {
	if len(pp.schemaPath) == 0 {
		return "", false
	}
	s, ok := pp.schemaPath[0].(walk.GetAttrStep)
	if !ok {
		return "", false
	}
	n := TerraformToPulumiNameV2(s.Name, schemaMap, schemaInfos)
	return resource.PropertyKey(n), true
}

// Formats a CheckFailure to report an issue with a resource or provider configuration. This method is made public to
// facilitate cross-package use by the bridge framework and is not intended for use by provider authors.
func NewCheckFailure(
	reasonType CheckFailureReason,
	reason string,
	pp *CheckFailurePath,
	urn resource.URN,
	isProvider bool,
	configPrefix string,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
) plugin.CheckFailure {
	if reasonType == MissingKey {
		if isProvider {
			return missingProviderKey(urn, configPrefix, pp, schemaMap, schemaInfos)
		}
		return missingRequiredKey(pp, configPrefix, schemaMap, schemaInfos)
	}
	if isProvider {
		return formatProviderCheckFailure(reasonType, reason, pp, urn, configPrefix, schemaMap, schemaInfos)
	}
	if pp != nil && pp.valuePath != "" {
		reason = fmt.Sprintf("%s. Examine values at '%s.%s'.", reason, urn.Name().String(), pp.valuePath)
	}
	return plugin.CheckFailure{
		Reason: reason,
		// Do not populate Property here as that changes the UX of how it displays in CLI. We may want to
		// consider this change at some point.
	}
}

func formatProviderCheckFailure(
	reasonType CheckFailureReason,
	reason string,
	pp *CheckFailurePath,
	urn resource.URN,
	configPrefix string,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
) plugin.CheckFailure {
	reason = "could not validate provider configuration: " + reason
	if isExplicitProvider(urn) {
		return formatExplicitProviderCheckFailure(reason, pp, urn)
	}
	return formatDefaultProviderCheckFailure(reasonType, reason, pp, configPrefix, schemaMap, schemaInfos)
}

func formatExplicitProviderCheckFailure(reason string, pp *CheckFailurePath, urn resource.URN) plugin.CheckFailure {
	if pp != nil && pp.valuePath != "" && isExplicitProvider(urn) {
		reason = fmt.Sprintf("%s. Examine values at '%s.%s'.", reason, urn.Name().String(), pp.valuePath)
	}
	return plugin.CheckFailure{
		Reason: reason,
	}
}

func formatDefaultProviderCheckFailure(
	reasonType CheckFailureReason,
	reason string,
	pp *CheckFailurePath,
	configPrefix string,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
) plugin.CheckFailure {
	if pp != nil {
		getExpr := "pulumi config get " + pulumiConfigExpr(configPrefix, *pp)
		reason = fmt.Sprintf("%s. Check `%s`.", reason, getExpr)
	}
	if pp != nil && reasonType == InvalidKey {
		if sugg := keySuggestions(*pp, schemaMap, schemaInfos); len(sugg) > 0 {
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

func keySuggestions(
	pp CheckFailurePath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
) []resource.PropertyKey {
	// Only supported for top-level keys for now.
	if pp.len() != 1 {
		return nil
	}
	k, ok := pp.topLevelPropertyKey(schemaMap, schemaInfos)
	if !ok {
		return nil
	}
	var allKeys []resource.PropertyKey
	schemaMap.Range(func(key string, value shim.Schema) bool {
		translatedKey := TerraformToPulumiNameV2(key, schemaMap, schemaInfos)
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

func missingDefaultProviderKey(
	configPrefix string,
	pp *CheckFailurePath,
	schemaMap shim.SchemaMap,
) plugin.CheckFailure {
	reason := "Provider is missing a required configuration key"
	if pp != nil {
		configSetCommand := "pulumi config set " + pulumiConfigExpr(configPrefix, *pp)
		reason = fmt.Sprintf("%s, try `%s`", reason, configSetCommand)
		desc := lookupDescription(*pp, schemaMap)
		if desc != "" {
			reason += ": " + desc
		}
	}
	return plugin.CheckFailure{
		Reason: reason,
		// Do not populate Property here intentionally as currently CLI would display that with a default
		// provider URN as if it is a resource, which is confusing. Instead Reason contains instructions on how
		// to set the value via a `pulumi config` command.
	}
}

func pulumiConfigExpr(configPrefix string, pp CheckFailurePath) (expr string) {
	if pp.len() > 1 {
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
	pp *CheckFailurePath,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
) plugin.CheckFailure {
	if isExplicitProvider(urn) {
		return missingRequiredKey(pp, configPrefix, schemaMap, schemaInfos)
	}
	return missingDefaultProviderKey(configPrefix, pp, schemaMap)
}

func missingRequiredKey(
	pp *CheckFailurePath,
	configPrefix string,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
) plugin.CheckFailure {
	reason := fmt.Sprintf("Missing required property '%s'", pp.valuePath)

	if pp != nil {
		if defaultConfig, ok := lookupDefaultConfig(*pp, schemaInfos); ok {
			expr := fmt.Sprintf("pulumi config set %s:%s <value>", configPrefix, defaultConfig)
			reason = fmt.Sprintf("%s. Either set it explicitly or configure it with '%s'", reason, expr)
		}
		desc := lookupDescription(*pp, schemaMap)
		if desc != "" {
			reason += ": " + desc
		}
	}

	return plugin.CheckFailure{
		Reason: reason,
	}
}

func lookupDefaultConfig(
	pp CheckFailurePath,
	schemaInfos map[string]*SchemaInfo,
) (string, bool) {
	info := LookupSchemaInfoMapPath(pp.schemaPath, schemaInfos)
	if info == nil {
		return "", false
	}
	if info.Default == nil {
		return "", false
	}
	if info.Default.Config == "" {
		return "", false
	}
	return info.Default.Config, true
}

func lookupDescription(pp CheckFailurePath, schemaMap shim.SchemaMap) (desc string) {
	s, err := walk.LookupSchemaMapPath(pp.schemaPath, schemaMap)
	if err == nil && s != nil {
		// TF descriptions often have newlines in inopportune positions. This makes them present a
		// little better in our console output.
		desc = strings.ReplaceAll(s.Description(), "\n", " ")
	}
	return
}
