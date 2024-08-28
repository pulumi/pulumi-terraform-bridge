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
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/parse"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/parse/section"
)

func plainDocsParser(docFile *DocFile, g *Generator) ([]byte, error) {
	// Get file content without front matter
	content := trimFrontMatter(docFile.Content)
	// Add pulumi-specific front matter
	// Generate pulumi-specific front matter
	frontMatter := writeFrontMatter(g.info.Name)

	// Generate pulumi-specific installation instructions
	installationInstructions := writeInstallationInstructions(g.info.Golang.ImportBasePath, g.info.Name)

	// Add instructions to top of file
	contentStr := frontMatter + installationInstructions + string(content)

	//Translate code blocks to Pulumi
	contentStr, err := translateCodeBlocks(contentStr, g)
	if err != nil {
		return nil, err
	}

	// Apply edit rules to transform the doc for Pulumi-ready presentation
	contentBytes, err := applyEditRules([]byte(contentStr), docFile.FileName, g)
	if err != nil {
		return nil, err
	}

	// Remove the title. A title gets populated from Hugo frontmatter; we do not want two.
	contentBytes, err = removeTitle(contentBytes)
	if err != nil {
		return nil, err
	}

	//Reformat field names. Configuration fields are camelCased like nodejs.
	contentStr, _ = reformatText(infoContext{
		language: "nodejs",
		pkg:      g.pkg,
		info:     g.info,
	}, string(contentBytes), nil)

	return []byte(contentStr), nil
}

func writeFrontMatter(providerName string) string {
	// Capitalize the package name
	capitalize := cases.Title(language.English)
	title := capitalize.String(providerName)

	return fmt.Sprintf(delimiter+
		"title: %[1]s Provider\n"+
		"meta_desc: Provides an overview on how to configure the Pulumi %[1]s provider.\n"+
		"layout: package\n"+
		delimiter+
		"\n",
		title)
}

// writeInstallationInstructions renders the following for any provider:
// ****
// Installation
// The Foo provider is available as a package in all Pulumi languages:
//
// JavaScript/TypeScript: @pulumi/foo
// Python: pulumi-foo
// Go: github.com/pulumi/pulumi-foo/sdk/v3/go/foo
// .NET: Pulumi.foo
// Java: com.pulumi/foo
// ****
func writeInstallationInstructions(goImportBasePath, providerName string) string {

	// Capitalize the package name for C#
	capitalize := cases.Title(language.English)
	cSharpName := capitalize.String(providerName)

	return fmt.Sprintf(
		"## Installation\n\n"+
			"The %[1]s provider is available as a package in all Pulumi languages:\n\n"+
			"* JavaScript/TypeScript: [`@pulumi/%[1]s`](https://www.npmjs.com/package/@pulumi/%[1]s)\n"+
			"* Python: [`pulumi-%[1]s`](https://pypi.org/project/pulumi-%[1]s/)\n"+
			"* Go: [`%[3]s`](https://github.com/pulumi/pulumi-%[1]s)\n"+
			"* .NET: [`Pulumi.%[2]s`](https://www.nuget.org/packages/Pulumi.%[2]s)\n"+
			"* Java: [`com.pulumi/%[1]s`](https://central.sonatype.com/artifact/com.pulumi/%[1]s)\n\n",
		providerName,
		cSharpName,
		goImportBasePath,
	)
}

func applyEditRules(contentBytes []byte, docFile string, g *Generator) ([]byte, error) {
	// Obtain edit rules passed by the provider
	edits := g.editRules
	// Additional edit rules for installation files
	edits = append(edits,
		skipSectionHeadersEdit(docFile),
		removeTfVersionMentions(docFile),
		// Replace all "T/terraform" with "P/pulumi"
		reReplace(`Terraform`, `Pulumi`),
		reReplace(`terraform`, `pulumi`),
		// Replace all "H/hashicorp" strings
		reReplace(`Hashicorp`, `Pulumi`),
		reReplace(`hashicorp`, `pulumi`),
		// Reformat certain headers
		reReplace(`The following arguments are supported`,
			`The following configuration inputs are supported`),
		reReplace(`Argument Reference`,
			`Configuration Reference`),
		reReplace(`Schema`,
			`Configuration Reference`),
		reReplace("### Optional\n", ""),
		reReplace(`block contains the following arguments`,
			`input has the following nested fields`),
	)
	var err error
	for _, rule := range edits {
		contentBytes, err = rule.Edit(docFile, contentBytes)
		if err != nil {
			return nil, err
		}
	}
	return contentBytes, nil
}
func translateCodeBlocks(contentStr string, g *Generator) (string, error) {
	var returnContent string
	// Extract code blocks
	codeFence := "```"
	var codeBlocks []codeBlock
	for i := 0; i < (len(contentStr) - len(codeFence)); i++ {
		block, found := findCodeBlock(contentStr, i)
		if found {
			codeBlocks = append(codeBlocks, block)
			i = block.end + 1
		}
	}
	if len(codeBlocks) == 0 {
		return contentStr, nil
	}
	startIndex := 0
	for i, block := range codeBlocks {
		// Write the content up to the start of the code block
		returnContent = returnContent + contentStr[startIndex:block.start]
		nextNewLine := strings.Index(contentStr[block.start:block.end], "\n")
		if nextNewLine == -1 {
			//Write the inline block
			returnContent = returnContent + contentStr[block.start:block.end] + codeFence + "\n"
			continue
		}
		fenceLanguage := contentStr[block.start : block.start+nextNewLine+1]
		code := contentStr[block.start+nextNewLine+1 : block.end]
		// Only convert code blocks that we have reasonable suspicion of actually being Terraform.
		if isHCL(fenceLanguage, code) {
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
	if pulumiYAML == "" {
		return pulumiYAML
	}
	// Replace the project name from the default `/` to a more descriptive name
	nameRegex := regexp.MustCompile(`name: /*`)
	pulumiYAMLFile := nameRegex.ReplaceAllString(pulumiYAML, "name: configuration-example")
	// Replace the runtime with the language specified.
	//Unfortunately, lang strings don't quite map to runtime strings.
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

	langs := genLanguageToSlice(g.language)
	const (
		chooserStart = `{{< chooser language "typescript,python,go,csharp,java,yaml" >}}` + "\n"
		chooserEnd   = "{{< /chooser >}}\n"
		choosableEnd = "\n{{% /choosable %}}\n"
	)
	exampleContent := chooserStart

	// Generate each language in turn and mark up the output with the correct Hugo shortcodes.
	for _, lang := range langs {
		choosableStart := fmt.Sprintf("{{%% choosable language %s %%}}\n", lang)

		// Generate the Pulumi.yaml config file for each language
		configFile := pclExample.PulumiYAML
		pulumiYAML := processConfigYaml(configFile, lang)
		// Generate language example
		convertedLang, err := converter.singleExampleFromPCLToLanguage(pclExample, lang)
		if err != nil {
			g.warn(err.Error())
		}
		exampleContent += choosableStart + pulumiYAML + convertedLang + choosableEnd
	}
	exampleContent += chooserEnd
	return exampleContent, nil
}

type titleRemover struct {
}

var _ parser.ASTTransformer = titleRemover{}

func (tr titleRemover) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	// Walk to find sections that should be skipped.
	// Walk() loses information on subsequent nodes when nodes are removed during the walk, so we only gather them here.
	err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		// The first header we encounter should be the document title.
		if header, ok := n.(*ast.Heading); ok && entering {
			if header.Level == 1 {
				parent := n.Parent()
				if parent == nil {
					panic("PARENT IS NIL")
				}
				// Removal here is safe, as we want to remove only the first header anyway.
				n.Parent().RemoveChild(parent, header)
				return ast.WalkStop, nil
			}
		}
		return ast.WalkContinue, nil
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
func skipSectionHeadersEdit(docFile string) tfbridge.DocsEdit {
	defaultHeaderSkipRegexps := getDefaultHeadersToSkip()
	return tfbridge.DocsEdit{
		Path: docFile,
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
		regexp.MustCompile("[Tt]erraform Cloud"),
	}
	return defaultHeaderSkipRegexps
}

func getTfVersionsToRemove() []*regexp.Regexp {
	tfVersionsToRemove := []*regexp.Regexp{
		regexp.MustCompile(`It requires [tT]erraform [v0-9]+\.?[0-9]?\.?[0-9]? or later.`),
		regexp.MustCompile(`(?s)(For )?[tT]erraform [v0-9]+\.?[0-9]?\.?[0-9]? and (later|earlier):`),
	}
	return tfVersionsToRemove
}

func removeTfVersionMentions(docFile string) tfbridge.DocsEdit {
	tfVersionsToRemove := getTfVersionsToRemove()
	return tfbridge.DocsEdit{
		Path: docFile,
		Edit: func(_ string, content []byte) ([]byte, error) {
			for _, tfVersion := range tfVersionsToRemove {
				content = tfVersion.ReplaceAll(content, nil)
			}
			return content, nil
		},
	}

}
