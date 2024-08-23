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
)

func defaultEditRules() editRules {
	return editRules{
		// Replace content such as "`terraform plan`" with "`pulumi preview`"
		boundedReplace("[tT]erraform [pP]lan", "pulumi preview"),
		// Replace content such as " Terraform Apply." with " pulumi up."
		boundedReplace("[tT]erraform [aA]pply", "pulumi up"),
		reReplace(`"([mM])ade (by|with) [tT]erraform"`, `"Made $2 Pulumi"`),
		// A markdown link that has terraform in the link component.
		reReplace(`\[([^\]]*)\]\([^\)]*terraform([^\)]*)\)`, "$1"),
		fixupImports(),
		// Replace content such as "jdoe@hashicorp.com" with "jdoe@example.com"
		reReplace("@hashicorp.com", "@example.com"),
	}
}

type editRules []tfbridge.DocsEdit

func (rr editRules) apply(fileName string, contents []byte) ([]byte, error) {
	for _, rule := range rr {
		match, err := filepath.Match(rule.Path, fileName)
		if err != nil {
			return nil, fmt.Errorf("invalid glob: %q: %w", rule.Path, err)
		}
		if !match {
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
func reReplace(from, to string) tfbridge.DocsEdit {
	r := regexp.MustCompile(from)
	bTo := []byte(to)
	return tfbridge.DocsEdit{
		Path: "*",
		Edit: func(_ string, content []byte) ([]byte, error) {
			return r.ReplaceAll(content, bTo), nil
		},
	}
}

func fixupImports() tfbridge.DocsEdit {

	var inlineImportRegexp = regexp.MustCompile("% [tT]erraform import.*")
	var quotedImportRegexp = regexp.MustCompile("`[tT]erraform import`")

	// (?s) makes the '.' match newlines (in addition to everything else).
	var blockImportRegexp = regexp.MustCompile("(?s)In [tT]erraform v[0-9]+\\.[0-9]+\\.[0-9]+ and later," +
		" use an `import` block.*?```.+?```\n")

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
