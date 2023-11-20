package tfbridge

import (
	"strings"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func replaceSubstring(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
}

func replaceSubstrings(s string, replacements map[string]string) string {
	for k, v := range replacements {
		s = replaceSubstring(s, k, v)
	}
	return s
}

func getConfigReplacements(
	providerName string,
	config map[string]*SchemaInfo,
	schema shim.SchemaMap,
) map[string]string {
	renames := make(map[string]string)
	schema.Range(func(key string, value shim.Schema) bool {
		pulumiName := TerraformToPulumiNameV2(key, schema, config)
		renames[key] = providerName + ":" + pulumiName
		return true
	})
	return renames
}

// ReplaceConfigProperties replaces all Terraform config property names
// in the given message with their Pulumi equivalents.
// This only works for top-level properties currently.
// TODO https://github.com/pulumi/pulumi-terraform-bridge/issues/1533
func ReplaceConfigProperties(msg string, providerName string, config map[string]*SchemaInfo, schema shim.SchemaMap) string {
	return replaceSubstrings(msg, getConfigReplacements(providerName, config, schema))
}
