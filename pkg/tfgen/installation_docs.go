package tfgen

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	markdown "github.com/teekennedy/goldmark-markdown"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/parse"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/parse/section"
)

func plainDocsParser(docFile *DocFile, g *Generator) ([]byte, error) {
	// Apply pre-code translation edit rules. This applies all default edit rules and provider-supplied edit rules in
	// the default pre-code translation phase.
	contentBytes, err := g.editRules.apply(docFile.FileName, docFile.Content, info.PreCodeTranslation)
	if err != nil {
		return nil, err
	}

	// Get file content without front matter
	content := trimFrontMatter(contentBytes)

	providerDisplayName := getProviderDisplayName(g)

	// Add pulumi-specific front matter
	// Generate pulumi-specific front matter
	frontMatter := writeFrontMatter(providerDisplayName)

	// Remove the title. A title gets populated from Hugo frontmatter; we do not want two.
	content, err = removeTitle(content)
	if err != nil {
		return nil, err
	}

	// Strip the tfplugindocs generation HTML comment
	content = stripSchemaGeneratedByTFPluginDocs(content)

	// Generate pulumi-specific installation instructions
	installationInstructions := writeInstallationInstructions(
		g.info.Golang.ImportBasePath,
		providerDisplayName,
		g.pkg.Name().String(),
		g.info.GitHubOrg,
		g.info.Repository,
	)

	// Determine if we should write an overview header.
	overviewHeader := getOverviewHeader(content)

	// Translate code blocks to Pulumi
	contentStr, err := translateCodeBlocks(string(content), g)
	if err != nil {
		return nil, err
	}

	// If the code translation resulted in an empty examples section, remove it
	content, err = removeEmptySection("Example Usage", []byte(contentStr))
	if err != nil {
		return nil, err
	}

	// Apply post-code translation edit rules. This applies all default edit rules and provider-supplied edit rules in
	// the post-code translation phase.
	contentBytes, err = g.editRules.apply(docFile.FileName, content, info.PostCodeTranslation)
	if err != nil {
		return nil, err
	}
	// Reformat field names. Configuration fields are camelCased like nodejs.
	contentStr, _ = reformatText(infoContext{
		language: "nodejs",
		pkg:      g.pkg,
		info:     g.info,
	}, string(contentBytes), nil)

	// Add instructions to top of file
	// Instructions need to be added _after_ the editRules are called,
	// because if "hashicorp" or "terraform" show up in the dynamic provider source, we want that to remain.
	contentStr = frontMatter + installationInstructions + overviewHeader + contentStr

	return []byte(contentStr), nil
}

func writeFrontMatter(providerDisplayName string) string {
	return fmt.Sprintf(delimiter+
		"# *** WARNING: This file was auto-generated. "+
		"Do not edit by hand unless you're certain you know what you are doing! ***\n"+
		"title: %[1]s Provider\n"+
		"meta_desc: Provides an overview on how to configure the Pulumi %[1]s provider.\n"+
		"layout: package\n"+
		delimiter+
		"\n",
		providerDisplayName)
}

// writeInstallationInstructions renders the following for a pulumi-maintained provider:
//
//	Installation
//	The Foo provider is available as a package in all Pulumi languages:
//
//	JavaScript/TypeScript: @pulumi/foo
//	Python: pulumi-foo
//	Go: github.com/pulumi/pulumi-foo/sdk/v3/go/foo
//	.NET: Pulumi.foo
//	Java: com.pulumi/foo
//
// A dynamically bridged provider receives the following instead:
//
//	## Generate Provider
//
//	The Foo provider must be installed as a Local Package by following the instructions for Any Terraform Provider:
//	(link)
//	```bash
//	pulumi package add terraform-provider org/foo
//	```
func writeInstallationInstructions(goImportBasePath, displayName, pkgName, ghOrg, sourceRepo string) string {
	// Capitalize the package name for C#
	capitalize := cases.Title(language.English)
	cSharpName := capitalize.String(pkgName)

	installInstructions := fmt.Sprintf(
		"## Installation\n\n"+
			"The %[1]s provider is available as a package in all Pulumi languages:\n\n"+
			"* JavaScript/TypeScript: [`@pulumi/%[2]s`](https://www.npmjs.com/package/@pulumi/%[2]s)\n"+
			"* Python: [`pulumi-%[2]s`](https://pypi.org/project/pulumi-%[2]s/)\n"+
			"* Go: [`%[4]s`](https://github.com/pulumi/pulumi-%[2]s)\n"+
			"* .NET: [`Pulumi.%[3]s`](https://www.nuget.org/packages/Pulumi.%[3]s)\n"+
			"* Java: [`com.pulumi/%[2]s`](https://central.sonatype.com/artifact/com.pulumi/%[2]s)\n\n",
		displayName,
		pkgName,
		cSharpName,
		goImportBasePath,
	)

	generateInstructions := fmt.Sprintf("## Generate Provider\n\n"+
		"The %[1]s provider must be installed as a Local Package by following the "+
		"[instructions for Any Terraform Provider]"+
		"(https://www.pulumi.com/registry/packages/terraform-provider/):\n\n"+
		"```bash\n"+
		"pulumi package add terraform-provider %[2]s/%[3]s\n"+
		"```\n",
		displayName,
		ghOrg,
		pkgName,
	)

	deprecatedNote := fmt.Sprintf("~> **NOTE:** This provider was previously published as @pulumi/%[1]s.\n"+
		"However, that package is no longer being updated."+
		"Going forward, it is available as a [Local Package](https://www.pulumi.com/blog/any-terraform-provider/)"+
		" instead.\n"+
		"Please see the [provider's repository](https://github.com/pulumi/pulumi-%[1]s) for details.\n\n", pkgName)

	if strings.Contains(sourceRepo, "pulumi") {
		return installInstructions
	}
	for _, provider := range getDeprecatedProviderNames() {
		if provider == pkgName {
			// append the deprecation note
			generateInstructions = generateInstructions + deprecatedNote
		}
	}
	return generateInstructions
}

// TODO: remove this note after 90 days: https://github.com/pulumi/pulumi-terraform-bridge/issues/2885
func getDeprecatedProviderNames() []string {
	providerNames := []string{
		"civo",
		"rke",
		"libvirt",
		"sumologic",
	}
	return providerNames
}

func getOverviewHeader(content []byte) string {
	const overviewHeader = "## Overview\n\n"
	// If content starts with an H2, or is otherwise empty then we don't want to add this header
	content = bytes.TrimSpace(content)
	if bytes.HasPrefix(content, []byte("## ")) || len(content) == 0 {
		return ""
	}
	return overviewHeader
}

var tfplugindocsComment = regexp.MustCompile("<!-- schema generated by tfplugindocs -->")

func stripSchemaGeneratedByTFPluginDocs(content []byte) []byte {
	content = tfplugindocsComment.ReplaceAll(content, nil)
	return content
}

func translateCodeBlocks(contentStr string, g *Generator) (string, error) {
	var returnContent string
	// Extract code blocks
	const codeFence string = "```"
	codeBlocks := findCodeBlocks([]byte(contentStr))
	if len(codeBlocks) == 0 {
		return contentStr, nil
	}
	startIndex := 0
	for i, block := range codeBlocks {
		// Write the content up to the start of the code block
		returnContent = returnContent + contentStr[startIndex:block.start]
		nextNewLine := strings.Index(contentStr[block.start:block.end], "\n")
		if nextNewLine == -1 {
			// Write the inline block
			returnContent = returnContent + contentStr[block.start:block.end] + codeFence + "\n"
			continue
		}

		// Only convert code blocks that we have reasonable suspicion of actually being Terraform.
		if code := block.code([]byte(contentStr)); isHCL(block.language, code) {
			exampleContent, err := convertExample(g, code, i)
			if err != nil {
				return "", err
			}
			returnContent += exampleContent
			startIndex = block.end + len(codeFence)
		} else {
			// Write any code block as-is.
			returnContent = returnContent + contentStr[block.start:block.end+len(codeFence)]
			startIndex = block.end + len(codeFence)
		}
	}
	// Write any remainder.
	returnContent = returnContent + contentStr[codeBlocks[len(codeBlocks)-1].end+len(codeFence):]
	return returnContent, nil
}

// This function renders the Pulumi.yaml config file for a given language if configuration is included in the example.
func processConfigYaml(pulumiYAML, lang string) string {
	// Replace the project name from the default `/` to a more descriptive name
	nameRegex := regexp.MustCompile(`name: /*`)
	pulumiYAMLFile := nameRegex.ReplaceAllString(pulumiYAML, "name: configuration-example")
	// Replace the runtime with the language specified.
	// Unfortunately, lang strings don't quite map to runtime strings.
	if lang == "typescript" {
		lang = "nodejs"
	}
	if lang == "csharp" {
		lang = "dotnet"
	}
	runtimeRegex := regexp.MustCompile(`runtime: terraform`)
	pulumiYAMLFile = runtimeRegex.ReplaceAllString(pulumiYAMLFile, fmt.Sprintf("runtime: %s", lang))
	// Add descriptive code comment
	pulumiYAMLFile = "```yaml\n" +
		"# Pulumi.yaml provider configuration file\n" +
		pulumiYAMLFile + "\n```\n"
	return pulumiYAMLFile
}

func convertExample(g *Generator, code string, exampleNumber int) (string, error) {
	// Make an example to record in the cliConverter.
	converter := g.cliConverter()
	fileName := fmt.Sprintf("configuration-installation-%d", exampleNumber)
	pclExample, err := converter.singleExampleFromHCLToPCL(fileName, code)
	if err != nil {
		return "", err
	}

	// If both PCL and PulumiYAML fields are empty, we can return.
	if pclExample.PulumiYAML == "" && pclExample.PCL == "" {
		return "", nil
	}

	// If we have a valid provider config but no additional code, we only render a YAML configuration block
	// with no choosers and an empty language runtime field
	if pclExample.PulumiYAML != "" && pclExample.PCL == "" {
		if pclExample.PCL == "" {
			return processConfigYaml(pclExample.PulumiYAML, ""), nil
		}
	}

	langs := genLanguageToSlice(g.language)
	const (
		chooserStart = `{{< chooser language "typescript,python,go,csharp,java,yaml" >}}` + "\n"
		chooserEnd   = "{{< /chooser >}}\n"
		choosableEnd = "\n{{% /choosable %}}\n"
	)
	exampleContent := chooserStart
	successfulConversion := false

	// Generate each language in turn and mark up the output with the correct Hugo shortcodes.
	for _, lang := range langs {
		choosableStart := fmt.Sprintf("{{%% choosable language %s %%}}\n", lang)

		// Generate the Pulumi.yaml config file for each language
		var pulumiYAML string
		if pclExample.PulumiYAML != "" {
			pulumiYAML = processConfigYaml(pclExample.PulumiYAML, lang)
		}

		// Generate language example
		convertedLang, err := converter.singleExampleFromPCLToLanguage(pclExample, lang)
		if err != nil {
			g.warn(err.Error())
		}
		if convertedLang != exampleUnavailable {
			successfulConversion = true
		}
		exampleContent += choosableStart + pulumiYAML + convertedLang + choosableEnd
	}

	if successfulConversion {
		return exampleContent + chooserEnd, nil
	}
	return "", nil
}

type titleRemover struct{}

var _ parser.ASTTransformer = titleRemover{}

func (tr titleRemover) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		// The first header we encounter should be the document title.
		header, found := n.(*ast.Heading)
		if !found || header.Level != 1 || !entering {
			return ast.WalkContinue, nil
		}

		parent := n.Parent()
		contract.Assertf(parent != nil, "parent cannot be nil")
		// Removal here is safe, as we want to remove only the first header anyway.
		n.Parent().RemoveChild(parent, header)
		return ast.WalkStop, nil
	})
	contract.AssertNoErrorf(err, "impossible: ast.Walk should never error")
}

func removeTitle(
	content []byte,
) ([]byte, error) {
	// Instantiate our transformer
	titleRemover := titleRemover{}
	gm := goldmark.New(
		goldmark.WithExtensions(parse.TFRegistryExtension),
		goldmark.WithParserOptions(parser.WithASTTransformers(
			util.Prioritized(titleRemover, 1000),
		)),
		goldmark.WithRenderer(markdown.NewRenderer()),
	)
	var buf bytes.Buffer
	// Convert parses the source, applies transformers, and renders output to buf
	err := gm.Convert(content, &buf)
	return buf.Bytes(), err
}

type sectionSkipper struct {
	shouldSkipHeader func(headerText string) bool
}

var _ parser.ASTTransformer = sectionSkipper{}

func (t sectionSkipper) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()
	var sectionsToSkip []ast.Node

	// Walk to find sections that should be skipped.
	// Walk() loses information on subsequent nodes when nodes are removed during the walk, so we only gather them here.
	err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if section, ok := n.(*section.Section); ok && entering {
			headerText := section.FirstChild().(*ast.Heading).Text(source)
			if t.shouldSkipHeader(string(headerText)) {
				parent := section.Parent()
				if parent == nil {
					panic("PARENT IS NIL")
				}
				sectionsToSkip = append(sectionsToSkip, section)
				return ast.WalkSkipChildren, nil
			}
		}
		return ast.WalkContinue, nil
	})
	contract.AssertNoErrorf(err, "impossible: ast.Walk should never error")

	// Remove the sections
	for _, section := range sectionsToSkip {
		parent := section.Parent()
		parent.RemoveChild(parent, section)
	}
}

// SkipSectionByHeaderContent removes headers where shouldSkipHeader(header) returns true,
// along with any text under the header.
// content is assumed to be Github flavored markdown when parsing.
//
// shouldSkipHeader is called on the raw header text, like this:
//
//	shouldSkipHeader("My Header")
//
// *not* like this:
//
//	// This is wrong
//	shouldSkipHeader("## My Header\n")
//
// Example of removing the first header and its context
//
//	result := SkipSectionByHeaderContent(byte[](
//	`
//	## First Header
//
//	First content
//
//	## Second Header
//
//	Second content
//	`,
//	func(headerText string) bool) ([]byte, error) {
//		return headerText == "First Header"
//	})
//
// The result will only now contain the second result:
//
//	## Second Header
//
//	Second content
func SkipSectionByHeaderContent(
	content []byte,
	shouldSkipHeader func(headerText string) bool,
) ([]byte, error) {
	// Instantiate our transformer
	sectionSkipper := sectionSkipper{shouldSkipHeader}
	gm := goldmark.New(
		goldmark.WithExtensions(parse.TFRegistryExtension),
		goldmark.WithParserOptions(parser.WithASTTransformers(
			util.Prioritized(sectionSkipper, 902),
		)),
		goldmark.WithRenderer(markdown.NewRenderer()),
	)
	var buf bytes.Buffer
	// Convert parses the source, applies transformers, and renders output to buf
	err := gm.Convert(content, &buf)
	return buf.Bytes(), err
}

// Edit Rule for skipping headers.
func skipSectionHeadersEdit() tfbridge.DocsEdit {
	defaultHeaderSkipRegexps := getDefaultHeadersToSkip()
	return tfbridge.DocsEdit{
		Path: "*",
		Edit: func(_ string, content []byte) ([]byte, error) {
			return SkipSectionByHeaderContent(content, func(headerText string) bool {
				for _, header := range defaultHeaderSkipRegexps {
					if header.Match([]byte(headerText)) {
						return true
					}
				}
				return false
			})
		},
		Phase: info.PostCodeTranslation,
	}
}

func getDefaultHeadersToSkip() []*regexp.Regexp {
	defaultHeaderSkipRegexps := []*regexp.Regexp{
		regexp.MustCompile("[Ll]ogging"),
		regexp.MustCompile("[Ll]ogs"),
		regexp.MustCompile("[Tt]esting"),
		regexp.MustCompile("[Dd]evelopment"),
		regexp.MustCompile("[Dd]ebugging"),
		regexp.MustCompile("[Tt]erraform CLI"),
		regexp.MustCompile("[Tt]erraform [Cc]loud"),
		regexp.MustCompile("Delete Protection"),
		regexp.MustCompile("[Cc]ontributing"),
	}
	return defaultHeaderSkipRegexps
}

func getTfVersionsToRemove() []*regexp.Regexp {
	tfVersionsToRemove := []*regexp.Regexp{
		regexp.MustCompile(`(It|This provider) requires( at least)? [tT]erraform [v0-9]+\.?[0-9]?\.?[0-9]?( or later)?.`),
		regexp.MustCompile(`(?s)(For )?[tT]erraform [v0-9]+\.?[0-9]?\.?[0-9]? and (later|earlier):`),
		regexp.MustCompile(`A minimum of [tT]erraform [v0-9]+\.?[0-9]?\.?[0-9]? is recommended.`),
		regexp.MustCompile("[tT]erraform `[v0-9.]+` (and|or) later:"),
	}
	return tfVersionsToRemove
}

func removeTfVersionMentions() tfbridge.DocsEdit {
	tfVersionsToRemove := getTfVersionsToRemove()
	return tfbridge.DocsEdit{
		Path: "*",
		Edit: func(_ string, content []byte) ([]byte, error) {
			for _, tfVersion := range tfVersionsToRemove {
				content = tfVersion.ReplaceAll(content, nil)
			}
			return content, nil
		},
		Phase: info.PostCodeTranslation,
	}
}

func getProviderDisplayName(g *Generator) string {
	providerName := g.info.DisplayName
	if providerName != "" {
		return providerName
	}
	// If the provider hasn't set an explicit Display Name, we infer from the package name.
	providerName = g.pkg.Name().String()
	// For display purposes, we'll capitalize the name.
	// This won't always work well - "aws" --> "Aws" isn't necessarily what we want
	// but it's a reasonable fallback option when info.DisplayName isn't set.
	capitalize := cases.Title(language.English)
	return capitalize.String(providerName)
}

func removeEmptySection(title string, contentBytes []byte) ([]byte, error) {
	if !isMarkdownSectionEmpty(title, contentBytes) {
		return contentBytes, nil
	}
	return SkipSectionByHeaderContent(contentBytes, func(headerText string) bool {
		return headerText == title
	})
}

func isMarkdownSectionEmpty(title string, contentBytes []byte) bool {
	gm := goldmark.New(goldmark.WithExtensions(parse.TFRegistryExtension))
	astNode := gm.Parser().Parse(text.NewReader(contentBytes))

	isEmpty := false

	err := ast.Walk(astNode, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if section, ok := n.(*section.Section); ok && entering {
			if section.HasChildren() {
				// A titled section is empty if it has only one child - the title.
				// If the child's text matches the title, the section is empty.
				sectionText := string(section.FirstChild().Text(contentBytes))
				if section.FirstChild() == section.LastChild() && sectionText == title {
					isEmpty = true
					return ast.WalkStop, nil
				}
			}
		}
		return ast.WalkContinue, nil
	})
	contract.AssertNoErrorf(err, "impossible: ast.Walk should never error")

	return isEmpty
}
