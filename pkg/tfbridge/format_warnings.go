// Deprecation Warning Translation — SDK v2 and SDK v1 Providers
//
// This file contains the core logic for translating Terraform validation warnings
// into user-friendly Pulumi warnings with camelCase property names.
//
// For Plugin Framework (PF) and dynamic bridged provider behavior, see
// pkg/pf/tfbridge/provider_diagnostics.go.
//
// # SDK v2 Providers
//
// Code path:
//
//	TF SDK v2 ValidateResource() generates diag.Diagnostic with Severity, Summary,
//	Detail, and AttributePath (cty.Path).
//	  → warningsAndErrors() in pkg/tfshim/sdk-v2/diagnostics.go
//	      Creates diagnostics.ValidationWarning preserving all three fields verbatim.
//	  → Consumer site calls formatValidationWarning() with schema context:
//	      - Check:       pkg/tfbridge/provider.go (Check method)
//	      - CheckConfig: pkg/tfbridge/provider.go (CheckConfig method)
//	      - Invoke:      pkg/tfbridge/provider.go (Invoke method)
//	  → formatValidationWarning() in this file:
//	      1. Picks Detail over Summary for the message text
//	      2. Applies CleanTerraformLanguage() to the message
//	      3. Applies TranslateFieldNamesInMessage() to replace snake_case field names
//	         in the message text with their Pulumi camelCase equivalents (only for
//	         fields that exist in the current schema)
//	      4. Calls formatAttributePathAsPropertyPath() (pkg/tfbridge/adapt_check_failures.go)
//	         which calls NewCheckFailurePath() → TerraformToPulumiNameV2()
//	         converting e.g. "resource_group_name" → "resourceGroupName"
//	      5. If Summary contains "deprecated" (case-insensitive):
//	         Returns: property "resourceGroupName" is deprecated: <cleaned message>
//	         Otherwise returns: property "resourceGroupName": <cleaned message>
//
// Behavior: Full fidelity. The cty.Path from the TF diagnostic is preserved through
// the shim layer and translated to Pulumi camelCase property names at the consumer
// sites. CleanTerraformLanguage rewrites TF-specific version references and "Terraform"
// mentions. Custom SchemaInfo.Name overrides are respected if present.
//
// What the user sees (Check):
//
//	warning: <URN> verification warning: property "resourceGroupName" is deprecated: use newField instead
//
// What the user sees (CheckConfig):
//
//	warning: provider config warning: property "resourceGroupName" is deprecated: use newField instead
//
// # SDK v1 Providers
//
// Code path:
//
//	TF SDK v1 ValidateResource() returns ([]string, []error) — no structured diagnostics.
//	  → wrapStringWarnings() in pkg/tfshim/sdk-v1/provider.go
//	      Creates ValidationWarning with Summary = the warning string, Detail = "",
//	      AttributePath = empty.
//	  → Consumer site calls formatValidationWarning() with schema context.
//	  → formatValidationWarning():
//	      1. msg = warn.Summary (since Detail is empty)
//	      2. CleanTerraformLanguage() is applied to msg
//	      3. formatAttributePathAsPropertyPath() returns nil (empty path)
//	      4. Falls back to warn.String() — returns the original Summary text
//	         unmodified. NOTE: the cleaned msg is discarded; warn.String() uses the
//	         original Summary.
//
// Behavior: Degraded. No AttributePath is available from SDK v1, so property names in
// the warning text remain in their original Terraform form. The fallback to
// warn.String() means CleanTerraformLanguage is effectively NOT applied (the cleaned
// msg variable is only used in the structured path branch). The warning passes through
// as-is from the upstream provider. This is acceptable because SDK v1 is legacy and
// very few providers still use it.
//
// What the user sees:
//
//	warning: <URN> verification warning: "resource_group_name": [DEPRECATED] use new_field instead
//
// # Summary Table
//
//	| Provider Type     | Path Available? | Path Type              | Name Translation            | CleanTerraformLanguage | Applies During                     |
//	|-------------------|-----------------|------------------------|-----------------------------|------------------------|------------------------------------|
//	| SDK v2            | Yes             | cty.Path               | Full (respects SchemaInfo)  | Yes                    | Check, CheckConfig, Invoke         |
//	| SDK v1            | No              | N/A                    | None                        | No (fallback path)     | N/A                                |
//	| PF                | Yes             | tftypes.AttributePath  | Full (respects SchemaInfo)  | Yes                    | Check, CheckConfig, Invoke         |
//	| PF                | Yes             | tftypes.AttributePath  | None                        | No                     | Diff, Create, Update, Read, Delete |
//	| Dynamic (v5/v6)   | Yes             | tftypes.AttributePath  | Default camelCase only      | Yes                    | Check, CheckConfig, Invoke         |
//	| Dynamic (v5/v6)   | Yes             | tftypes.AttributePath  | None                        | No                     | Diff, Create, Update, Read, Delete |
package tfbridge

import (
	"fmt"
	"regexp"
	"strings"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/diagnostics"
)

// IsDeprecationMessage returns true if the diagnostic summary indicates a deprecation.
// Terraform frameworks signal deprecation via the Summary field:
//   - SDK v2: "Argument is deprecated"
//   - Plugin Framework: "Argument deprecated" / "Attribute Deprecated"
//
// A case-insensitive check for "deprecated" reliably distinguishes deprecation warnings
// from other property-level warnings (e.g. validation warnings).
func IsDeprecationMessage(summary string) bool {
	return strings.Contains(strings.ToLower(summary), "deprecated")
}

// formatValidationWarning translates a structured ValidationWarning into a user-friendly string
// with Pulumi property names instead of Terraform attribute names.
func formatValidationWarning(
	warn diagnostics.ValidationWarning,
	schemaMap shim.SchemaMap,
	schemaInfos map[string]*SchemaInfo,
) string {
	msg := warn.Summary
	if warn.Detail != "" {
		msg = warn.Detail
	}
	msg = CleanTerraformLanguage(msg)
	msg = TranslateFieldNamesInMessage(msg, schemaMap, schemaInfos)

	pp := formatAttributePathAsPropertyPath(schemaMap, schemaInfos, warn.AttributePath)
	if pp != nil {
		if IsDeprecationMessage(warn.Summary) {
			return fmt.Sprintf("property %q is deprecated: %s", pp.ValuePath(), msg)
		}
		return fmt.Sprintf("property %q: %s", pp.ValuePath(), msg)
	}

	return warn.String()
}

// CleanTerraformLanguage replaces Terraform-specific language in deprecation messages
// with Pulumi-appropriate equivalents.
func CleanTerraformLanguage(msg string) string {
	// Replace version-specific removal notices with generic future version message.
	msg = tfVersionRemovalRegex.ReplaceAllString(msg, "will be removed in a future version")

	// Replace "Terraform" references in deprecation/warning contexts.
	msg = terraformRefRegex.ReplaceAllString(msg, "${1}the upstream provider${2}")

	return msg
}

// Matches patterns like "will be removed in X.Y.Z of the <Name> Provider" or
// "will be removed in a future version of the <Name> Provider" or
// "will be removed in version X.Y of the <Name> provider".
var tfVersionRemovalRegex = regexp.MustCompile(
	`will be removed in (?:version )?[v]?[\d]+\.[\d]+(?:\.[\d]+)? of the \w+ [Pp]rovider`)

// Matches "Terraform" when used to refer to the provider system (not as part of a compound word).
var terraformRefRegex = regexp.MustCompile(`(^|\W)Terraform(\W|$)`)

// snakeCaseIdentifier matches snake_case identifiers containing at least one underscore.
var snakeCaseIdentifier = regexp.MustCompile(`[a-z][a-z0-9]*(?:_[a-z0-9]+)+`)

// TranslateFieldNamesInMessage replaces snake_case field names in the message text
// with their Pulumi camelCase equivalents, but only when the token exactly matches
// a known field in the schema. This is conservative: unknown tokens pass through unchanged.
func TranslateFieldNamesInMessage(msg string, schemaMap shim.SchemaMap, schemaInfos map[string]*SchemaInfo) string {
	if schemaMap == nil || schemaMap.Len() == 0 {
		return msg
	}

	// Build lookup of tf_name -> pulumi_name for fields where they differ.
	lookup := make(map[string]string)
	schemaMap.Range(func(tfName string, _ shim.Schema) bool {
		pulumiName := TerraformToPulumiNameV2(tfName, schemaMap, schemaInfos)
		if pulumiName != tfName {
			lookup[tfName] = pulumiName
		}
		return true
	})

	if len(lookup) == 0 {
		return msg
	}

	return snakeCaseIdentifier.ReplaceAllStringFunc(msg, func(match string) string {
		if replacement, ok := lookup[match]; ok {
			return replacement
		}
		return match
	})
}
