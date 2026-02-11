// Copyright 2016-2022, Pulumi Corporation.
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

package tfgen

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func defaultEditRules() editRules {
	return editRules{
		// Replace content such as "`terraform plan`" with "`pulumi preview`"
		boundedReplace("[tT]erraform [pP]lan", "pulumi preview"),
		// Replace content such as " Terraform Apply." with " pulumi up."
		boundedReplace("[tT]erraform [aA]pply", "pulumi up"),
		reReplace(`"([mM])ade (by|with) [tT]erraform"`, `"Made $2 Pulumi"`, info.PreCodeTranslation),
		// A markdown link that has terraform in the link component.
		reReplace(`\[([^\]]*)\]\([^\)]*terraform([^\)]*)\)`, "$1", info.PreCodeTranslation),
		reReplace("Terraform [Ww]orkspace", "Pulumi Stack", info.PreCodeTranslation),
		fixupImports(),
		// Replace content such as "jdoe@hashicorp.com" with "jdoe@example.com"
		reReplace("@hashicorp.com", "@example.com", info.PreCodeTranslation),
		reReplace(`"Managed by Terraform"`, `"Managed by Pulumi"`, info.PreCodeTranslation),

		// The following edit rules may be applied after translating the code sections in a document.
		// Their primary use case is for the docs translation approach spearheaded in installation_docs.go.
		// These edit rules allow us to safely transform certain strings that we would otherwise need in the
		// code translation or nested type discovery process.
		// These rules are currently only called when generating installation docs.
		// TODO[https://github.com/pulumi/pulumi-terraform-bridge/issues/2459] Call info.PostCodeTranslation rules
		// on all docs.
		skipSectionHeadersEdit(),
		removeTfVersionMentions(),
		// Replace "providers.tf" with "Pulumi.yaml"
		reReplace(`providers.tf`, `Pulumi.yaml`, info.PostCodeTranslation),
		reReplace(`terraform init`, `pulumi up`, info.PostCodeTranslation),
		// Replace all " T/terraform" with " P/pulumi"
		reReplace(`Terraform`, `Pulumi`, info.PostCodeTranslation),
		reReplace(`terraform`, `pulumi`, info.PostCodeTranslation),
		// Replace all "H/hashicorp" strings
		reReplace(`Hashicorp`, `Pulumi`, info.PostCodeTranslation),
		reReplace(`hashicorp`, `pulumi`, info.PostCodeTranslation),
		// Reformat certain headers
		reReplace(`The following arguments are supported`,
			`The following configuration inputs are supported`, info.PostCodeTranslation),
		reReplace(`The provider supports the following arguments`,
			`The following configuration inputs are supported`, info.PostCodeTranslation),
		reReplace(`Argument Reference`,
			`Configuration Reference`, info.PostCodeTranslation),
		reReplace(`# Arguments`,
			`# Configuration Reference`, info.PostCodeTranslation),
		reReplace(`# Configuration Schema`,
			`# Configuration Reference`, info.PostCodeTranslation),
		reReplace(`# Schema`,
			`# Configuration Reference`, info.PostCodeTranslation),
		reReplace("### Optional\n", "", info.PostCodeTranslation),
		reReplace(`block contains the following arguments`,
			`input has the following nested fields`, info.PostCodeTranslation),
		reReplace(`provider block`, `provider configuration`, info.PostCodeTranslation),
		reReplace("`provider` block", "provider configuration", info.PostCodeTranslation),
		reReplace("Data Source", "Function", info.PostCodeTranslation),
		reReplace("data source", "function", info.PostCodeTranslation),
		reReplace("Datasource", "Function", info.PostCodeTranslation),
		reReplace("datasource", "function", info.PostCodeTranslation),
	}
}

type editRules []tfbridge.DocsEdit

func (rr editRules) apply(fileName string, contents []byte, phase info.EditPhase) ([]byte, error) {
	for _, rule := range rr {
		match, err := filepath.Match(rule.Path, fileName)
		if err != nil {
			return nil, fmt.Errorf("invalid glob: %q: %w", rule.Path, err)
		}
		if !match || (rule.Phase != phase) {
			continue
		}
		contents, err = rule.Edit(fileName, contents)
		if err != nil {
			return nil, fmt.Errorf("replace failed: %w", err)
		}
	}
	return contents, nil
}

// Get the replace rule set for a DocRuleInfo.
//
// getEditRules is only called once during `tfgen`, so we move the cost of compiling
// regexes into getEditRules, avoiding a marginal startup time penalty.
func getEditRules(info *tfbridge.DocRuleInfo) editRules {
	defaults := defaultEditRules()
	if info == nil || info.EditRules == nil {
		return defaults
	}
	return info.EditRules(defaults)
}

// Create a regexp based replace rule that is bounded by non-ascii letter text.
//
// This function is not appropriate to be called in hot loops.
func boundedReplace(from, to string) tfbridge.DocsEdit {
	r := regexp.MustCompile(fmt.Sprintf(`([^a-zA-Z]|^)%s([^a-zA-Z]|$)`, from))
	bTo := []byte(fmt.Sprintf("${1}%s${%d}", to, r.NumSubexp()))
	return tfbridge.DocsEdit{
		Path: "*",
		Edit: func(_ string, content []byte) ([]byte, error) {
			return r.ReplaceAll(content, bTo), nil
		},
	}
}

// reReplace creates a regex based replace.
func reReplace(from, to string, phase info.EditPhase) tfbridge.DocsEdit {
	r := regexp.MustCompile(from)
	bTo := []byte(to)
	return tfbridge.DocsEdit{
		Path: "*",
		Edit: func(_ string, content []byte) ([]byte, error) {
			return r.ReplaceAll(content, bTo), nil
		},
		Phase: phase,
	}
}

func fixupImports() tfbridge.DocsEdit {
	inlineImportRegexp := regexp.MustCompile("% [tT]erraform import.*")
	quotedImportRegexp := regexp.MustCompile("`[tT]erraform import`")

	// (?s) makes the '.' match newlines (in addition to everything else).
	blockImportRegexp := regexp.MustCompile("(?s)In [tT]erraform v[0-9]+\\.[0-9]+\\.[0-9]+ and later," +
		".*?`import` block.*?```.+?```\n")

	return tfbridge.DocsEdit{
		Path: "*",
		Edit: func(_ string, content []byte) ([]byte, error) {
			// Strip import blocks
			content = blockImportRegexp.ReplaceAllLiteral(content, nil)
			content = inlineImportRegexp.ReplaceAllFunc(content, func(match []byte) []byte {
				match = bytes.ReplaceAll(match, []byte("terraform"), []byte("pulumi"))
				match = bytes.ReplaceAll(match, []byte("Terraform"), []byte("Pulumi"))
				return match
			})
			content = quotedImportRegexp.ReplaceAllLiteral(content, []byte("`pulumi import`"))
			return content, nil
		},
	}
}
