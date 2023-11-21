package tfbridge

import (
	"regexp"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

func replaceSubstringsRegex(s string, replacements map[string]string) (string, error) {
	regexPattern := `"(`
	for k := range replacements {
		regexPattern += regexp.QuoteMeta(k) + "|"
	}
	regexPattern += `)"`
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return s, err
	}

	return re.ReplaceAllStringFunc(s, func(match string) string {
		// Strip the quotes
		key := match[1 : len(match)-1]
		return "\"" + replacements[key] + "\""
	}), nil
}

func getConfigReplacements(
	config map[string]*SchemaInfo,
	schema shim.SchemaMap,
) map[string]string {
	renames := make(map[string]string)
	schema.Range(func(key string, value shim.Schema) bool {
		pulumiName := TerraformToPulumiNameV2(key, schema, config)
		renames[key] = pulumiName
		return true
	})
	return renames
}

// ReplaceConfigProperties replaces all Terraform config property names
// in the given message with their Pulumi equivalents.
// This method is made public to facilitate cross-package use by the
// bridge framework and is not intended for use by provider authors.
// This only works for top-level properties currently.
// TODO https://github.com/pulumi/pulumi-terraform-bridge/issues/1533
func ReplaceConfigProperties(msg string, config map[string]*SchemaInfo, schema shim.SchemaMap) (string, error) {
	return replaceSubstringsRegex(msg, getConfigReplacements(config, schema))
}
