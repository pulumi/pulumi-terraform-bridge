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
	"regexp"
	"strings"

	"github.com/agext/levenshtein"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
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
	schemaPath walk.SchemaPath

	// valuePath is a form suitable to pass to `pulumi config get`.
	//
	// "accessToken" is a top-level property example, can be used as:
	//
	//	pulumi config get aws:accessToken
	//
	// "assumeRoleArn.roleArn" is a nested example, can be used as:
	//
	//	pulumi config get --path aws:assumeRoleArn.roleArn
	//
	// For complicated literals it may resort to quoting as needed:
	//
	//      foo["c o m p l i c a t e d"]`
	//
	// It form is also approximately a TypeScript-like literal and is used as such in error
	// message suggestions.
	valuePath string

	schemaMap   shim.SchemaMap
	schemaInfos map[string]*SchemaInfo
}

func NewCheckFailurePath(
	schemaMap shim.SchemaMap, schemaInfos map[string]*SchemaInfo, prop string,
) CheckFailurePath {
	pulumiName := TerraformToPulumiNameV2(prop, schemaMap, schemaInfos)
	return CheckFailurePath{
		schemaPath:  walk.NewSchemaPath().GetAttr(prop),
		valuePath:   pulumiName,
		schemaMap:   schemaMap,
		schemaInfos: schemaInfos,
	}
}

func (p CheckFailurePath) valuePathDot(prop string) string {
	r := regexp.MustCompile("^[_a-zA-Z][_-a-zA-Z0-9]*$")
	if r.MatchString(prop) {
		return fmt.Sprintf("%s.%s", p.valuePath, prop)
	}
	return fmt.Sprintf("%s[%q]", p.valuePath, prop)
}

func (p CheckFailurePath) Attribute(name string) CheckFailurePath {
	path := p.schemaPath.GetAttr(name)
	pulumiName, err := TerraformToPulumiNameAtPath(path, p.schemaMap, p.schemaInfos)
	if err != nil {
		pulumiName = name
	}
	return CheckFailurePath{
		schemaPath:  path,
		valuePath:   p.valuePathDot(pulumiName),
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
		valuePath = p.valuePathDot(s)
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

func (p CheckFailurePath) len() int {
	return len(p.schemaPath)
}

func (p CheckFailurePath) topLevelPropertyKey(
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
) (resource.PropertyKey, bool) {
	if len(p.schemaPath) == 0 {
		return "", false
	}
	s, ok := p.schemaPath[0].(walk.GetAttrStep)
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
		reason = fmt.Sprintf("%s. Examine values at '%s.%s'.", reason, urn.Name(), pp.valuePath)
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
		if pp != nil && pp.valuePath != "" {
			reason = fmt.Sprintf("%s. Examine values at '%s.%s'.", reason, urn.Name(),
				pp.valuePath)
		}
		// Similarly to normal resources, do not populate Property here as that changes the UX of how it
		// displays in CLI. We may want to consider this change at some point.
		//
		//     return plugin.CheckFailure{Reason: reason, Property: resource.PropertyKey(pp.valuePath)}
		//
		return plugin.CheckFailure{Reason: reason}
	}
	return formatDefaultProviderCheckFailure(reasonType, reason, pp, configPrefix, schemaMap, schemaInfos)
}

func formatDefaultProviderCheckFailure(
	reasonType CheckFailureReason,
	reason string,
	pp *CheckFailurePath,
	configPrefix string,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
) plugin.CheckFailure {
	if pp != nil && reasonType == InvalidKey {
		return formatDefaultProviderInvalidKey(*pp, configPrefix, schemaMap, schemaInfos)
	}
	if pp != nil {
		getExpr := "pulumi config get " + pulumiConfigExpr(configPrefix, *pp)
		reason = fmt.Sprintf("%s. Check `%s`.", reason, getExpr)
	}
	return plugin.CheckFailure{
		Reason: reason,
		// Do not populate Property here intentionally as currently CLI would display that with a default
		// provider URN as if it is a resource, which is confusing. Instead Reason contains instructions on how
		// to set the value via a `pulumi config` command.
	}
}

// This error arises when the user sets `pulumi config set aws:foo` but the AWS provider does not
// recognize foo. The upstream error message is not very actionable:
//
//	could not validate provider configuration: Invalid or unknown key.
//
// So this function provides a Pulumi-specific message instead, anchoring it with the key compatible
// with `pulumi config get --path key`.
func formatDefaultProviderInvalidKey(
	p CheckFailurePath,
	prefix string,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
) plugin.CheckFailure {
	sentences := []string{}
	invalid := fmt.Sprintf("`%s` is not a valid configuration key for the %s provider.",
		pulumiConfigExpr(prefix, p), prefix)
	sentences = append(sentences, invalid)
	if sugg := keySuggestions(p, schemaMap, schemaInfos); len(sugg) > 0 {
		quoted := []string{}
		for _, s := range sugg {
			quoted = append(quoted, fmt.Sprintf("`%s:%s`", prefix, string(s)))
		}
		dym := fmt.Sprintf("Did you mean %s?", strings.Join(quoted, " or "))
		sentences = append(sentences, dym)
	}
	suggest := fmt.Sprintf("If the key is not intended for the provider, please "+
		"choose a different namespace from `%s:`.", prefix)
	sentences = append(sentences, suggest)
	return plugin.CheckFailure{Reason: strings.Join(sentences, " ")}
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
		if string(k) == string(c) {
			continue
		}
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

func pulumiConfigExpr(configPrefix string, pp CheckFailurePath) string {
	if pp.len() > 1 {
		return fmt.Sprintf("--path %s:%s", configPrefix, pp.valuePath)
	}
	return fmt.Sprintf("%s:%s", configPrefix, pp.valuePath)
}

// Provider configuration can be using an explicit provider or the default provider, use a heuristic here based on URN,
// to detect the default provider.
func isExplicitProvider(urn resource.URN) bool {
	return urn != "" && !strings.HasPrefix(urn.Name(), "default")
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
	reason := "Missing required property"

	if pp != nil {
		reason = fmt.Sprintf("Missing required property '%s'", pp.valuePath)
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
		// Do not populate Property here as that changes the UX of how it displays in CLI. We may want to
		// consider this change at some point.
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
