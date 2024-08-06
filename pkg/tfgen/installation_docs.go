package tfgen

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func plainDocsParser(docFile *DocFile, g *Generator) ([]byte, error) {
	// Get file content without front matter, and split title
	contentStr, title := getBodyAndTitle(string(docFile.Content))
	// Add pulumi-specific front matter
	// Generate pulumi-specific front matter
	frontMatter := writeFrontMatter(title)

	// Generate pulumi-specific installation instructions
	installationInstructions := writeInstallationInstructions(g.info.Golang.ImportBasePath, g.info.Name)

	// Add instructions to top of file
	contentStr = frontMatter + installationInstructions + contentStr

	//Translate code blocks to Pulumi
	langs := genLanguageToSlice(g.language)

	conv, err := newExampleConverterFacade(g)
	if err != nil {
		return nil, err
	}

	contentStr, err = translateCodeBlocks(conv, langs, contentStr)
	if err != nil {
		// TODO we should not error here but have prior behavior, if pulumi CLI-based conversion is not available.
		return nil, err
	}

	//TODO: See https://github.com/pulumi/pulumi-terraform-bridge/issues/2078
	// - Ability to omit irrelevant sections

	// Apply edit rules to transform the doc for Pulumi-ready presentation
	contentBytes, err := applyEditRules([]byte(contentStr), docFile)
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

func writeFrontMatter(title string) string {
	return fmt.Sprintf(delimiter+
		"title: %[1]s Installation & Configuration\n"+
		"meta_desc: Provides an overview on how to configure the Pulumi %[1]s.\n"+
		"layout: package\n"+
		delimiter+
		"\n",
		title)
}

func writeIndexFrontMatter(displayName string) string {
	return fmt.Sprintf(delimiter+
		"title: %[1]s\n"+
		"meta_desc: The %[1]s provider for Pulumi "+
		"can be used to provision any of the cloud resources available in %[1]s.\n"+
		"layout: package\n"+
		delimiter,
		displayName)
}

func getBodyAndTitle(content string) (string, string) {
	// The first header in `index.md` is the package name, of the format `# Foo Provider`.
	titleIndex := strings.Index(content, "# ")
	// Get the location fo the next newline
	nextNewLine := strings.Index(content[titleIndex:], "\n") + titleIndex
	// Get the title line, without the h1 anchor
	title := content[titleIndex+2 : nextNewLine]
	// strip the title and any front matter
	return content[nextNewLine+1:], title
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

func applyEditRules(contentBytes []byte, docFile *DocFile) ([]byte, error) {
	// Obtain default edit rules for documentation files
	edits := defaultEditRules()

	// Additional edit rules for installation files
	edits = append(edits,
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
		reReplace(`block contains the following arguments`,
			`input has the following nested fields`))
	var err error
	for _, rule := range edits {
		contentBytes, err = rule.Edit(docFile.FileName, contentBytes)
		if err != nil {
			return nil, err
		}
	}
	return contentBytes, nil
}

func translateCodeBlocks(conv exampleConverterFacade, langs []string, contentStr string) (string, error) {
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
			path := configurationInstallationExamplePath(i)
			exampleContent, err := convertExample(conv, code, path, langs)
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

func configurationInstallationExamplePath(exampleNumber int) examplePath {
	fileName := fmt.Sprintf("configuration-installation-%d", exampleNumber)
	return examplePath{fullPath: fileName}
}

func convertExample(conv exampleConverterFacade, hclCode string, path examplePath, langs []string) (string, error) {
	const (
		chooserStart = `{{< chooser language "typescript,python,go,csharp,java,yaml" >}}` + "\n"
		chooserEnd   = "{{< /chooser >}}\n"
		choosableEnd = "\n{{% /choosable %}}\n"
	)
	exampleContent := chooserStart

	example, err := conv.FromHCL(path, hclCode)
	if err != nil {
		return "", err
	}

	configFile := example.PulumiYAML

	// Generate each language in turn and mark up the output with the correct Hugo shortcodes.
	// TODO: we want to use pulumi code choosers in the future, but resourcedocsgen does not support this yet.
	for _, lang := range langs {
		choosableStart := fmt.Sprintf("{{%% choosable language %s %%}}\n", lang)

		// Generate the Pulumi.yaml config file for each language
		pulumiYAML := processConfigYaml(configFile, lang)
		// Generate language example
		convertedLang, err := conv.ToLanguage(example, lang)
		if err != nil {
			return "", err
		}
		exampleContent += choosableStart + pulumiYAML + convertedLang + choosableEnd
	}
	exampleContent += chooserEnd
	return exampleContent, nil
}
