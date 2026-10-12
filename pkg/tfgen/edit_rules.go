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
	"slices"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func defaultEditRules() editRules {
	return slices.Concat([]tfbridge.DocsEdit{
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
	},
		// Replace "providers.tf" with "Pulumi.yaml"
		proseReplace(`providers.tf`, `Pulumi.yaml`),
		proseReplace(`terraform init`, `pulumi up`),
		// Replace all " T/terraform" with " P/pulumi"
		proseReplace(`Terraform`, `Pulumi`),
		proseReplace(`terraform`, `pulumi`),
		// Replace all "H/hashicorp" strings
		proseReplace(`Hashicorp`, `Pulumi`),
		proseReplace(`hashicorp`, `pulumi`),
		// Reformat certain headers
		proseReplace(`The following arguments are supported`,
			`The following configuration inputs are supported`),
		proseReplace(`The provider supports the following arguments`,
			`The following configuration inputs are supported`),
		proseReplace(`Argument Reference`,
			`Configuration Reference`),
		proseReplace(`# Arguments`,
			`# Configuration Reference`),
		proseReplace(`# Configuration Schema`,
			`# Configuration Reference`),
		proseReplace(`# Schema`,
			`# Configuration Reference`),
		proseReplace("### Optional\n", ""),
		proseReplace(`block contains the following arguments`,
			`input has the following nested fields`),
		proseReplace(`provider block`, `provider configuration`),
		proseReplace("`provider` block", "provider configuration"),
		proseReplace("Data Source", "Function"),
		proseReplace("data source", "function"),
		proseReplace("Datasource", "Function"),
		proseReplace("datasource", "function"),
	)
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

// proseReplace creates a regex based replace for text that is not translated code.
//
// It returns two edits. Before code translation, the comments inside code blocks that code translation will
// convert (see isHCL) are edited, so the converter carries the change into every generated language while the
// HCL itself stays valid. After code translation, everything outside translated code is edited: prose and the
// code blocks that were never translated.
func proseReplace(from, to string) []tfbridge.DocsEdit {
	r := regexp.MustCompile(from)
	bTo := []byte(to)
	replace := func(b []byte) []byte { return r.ReplaceAll(b, bTo) }
	keep := func(b []byte) []byte { return b }
	return []tfbridge.DocsEdit{
		{
			Path: "*",
			Edit: func(_ string, content []byte) ([]byte, error) {
				return editAroundCodeBlocks(content, keep, func(block codeBlock, span []byte) []byte {
					codeStart := bytes.IndexByte(span, '\n') + 1
					codeEnd := block.end - block.start
					code := span[codeStart:codeEnd]
					if !isHCL(block.language, string(code)) {
						return span
					}
					edited := append([]byte{}, span[:codeStart]...)
					edited = append(edited, replaceHCLComments(code, replace)...)
					return append(edited, span[codeEnd:]...)
				}), nil
			},
			Phase: info.PreCodeTranslation,
		},
		{
			Path: "*",
			Edit: func(_ string, content []byte) ([]byte, error) {
				return editAroundTranslatedCode(content, replace), nil
			},
			Phase: info.PostCodeTranslation,
		},
	}
}

// editAroundCodeBlocks rebuilds content by applying prose to each stretch of text outside a fenced code block
// and block to each fenced code block. The span passed to block runs from the opening fence to the end of the
// closing fence line.
func editAroundCodeBlocks(
	content []byte, prose func([]byte) []byte, block func(codeBlock, []byte) []byte,
) []byte {
	var result []byte
	start := 0
	for _, b := range findCodeBlocks(content) {
		// b.end is the first byte of the closing fence line. An unclosed block runs to the end of the
		// document.
		end := len(content)
		if eol := bytes.IndexByte(content[b.end:], '\n'); eol >= 0 {
			end = b.end + eol
		}
		result = append(result, prose(content[start:b.start])...)
		result = append(result, block(b, content[b.start:end])...)
		start = end
	}
	return append(result, prose(content[start:])...)
}

// replaceHCLComments applies replace to each comment token in code and copies every other byte through.
//
// The lexer still returns tokens for invalid HCL, which matters because isHCL guesses the language of untagged
// blocks, so its diagnostics are not needed.
func replaceHCLComments(code []byte, replace func([]byte) []byte) []byte {
	tokens, _ := hclsyntax.LexConfig(code, "", hcl.InitialPos)
	var result []byte
	last := 0
	for _, tok := range tokens {
		if tok.Type != hclsyntax.TokenComment {
			continue
		}
		start, end := tok.Range.Start.Byte, tok.Range.End.Byte
		result = append(result, code[last:start]...)
		result = append(result, replace(code[start:end])...)
		last = end
	}
	return append(result, code[last:]...)
}

// translatedCodeMarkers pairs the start and end of each region that code translation writes: the chooser that
// convertExample emits for installation docs and the code chooser comments that convertExamplesInner emits for
// resource docs.
//
// TODO: A provider configuration example without resource code skips the chooser (see the early return in
// convertExample) and is emitted as a bare YAML code block. That block is translated code too, but nothing
// marks it, so post-translation edits still apply to it.
var translatedCodeMarkers = [][2]string{
	{chooserStart, chooserEnd},
	{startPulumiCodeChooser, endPulumiCodeChooser},
}

// editAroundTranslatedCode applies edit to every stretch of content outside translated code.
func editAroundTranslatedCode(content []byte, edit func([]byte) []byte) []byte {
	var result []byte
	for {
		start, end := nextTranslatedCode(content)
		if start < 0 {
			return append(result, edit(content)...)
		}
		result = append(result, edit(content[:start])...)
		result = append(result, content[start:end]...)
		content = content[end:]
	}
}

// nextTranslatedCode returns the bounds of the first translated code region in content, or -1, -1 when there is
// none. A region without an end marker runs to the end of content.
func nextTranslatedCode(content []byte) (start, end int) {
	start, end = -1, -1
	for _, markers := range translatedCodeMarkers {
		s := bytes.Index(content, []byte(markers[0]))
		if s < 0 || (start >= 0 && s > start) {
			continue
		}
		e := len(content)
		if i := bytes.Index(content[s:], []byte(markers[1])); i >= 0 {
			e = s + i + len(markers[1])
		}
		start, end = s, e
	}
	return start, end
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
