package tfgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func plainDocsParser(docFile *DocFile, g *Generator) ([]byte, error) {
	// Get file content without front matter, and split title
	contentStr, title := getBodyAndTitle(string(docFile.Content))
	// Generate pulumi-specific front matter
	frontMatter := writeFrontMatter(title)

	// Generate pulumi-specific installation instructions
	installationInstructions := writeInstallationInstructions(g.info.Golang.ImportBasePath, g.info.Name)

	// Add instructions to top of file
	contentStr = frontMatter + installationInstructions + contentStr
	//TODO: See https://github.com/pulumi/pulumi-terraform-bridge/issues/2078
	// - Ability to omit irrelevant sections
	// - Include pulumi config set instructions somewhere, with example

	//Translate code blocks to Pulumi
	contentStr, err := translateCodeBlocks(contentStr, g)
	if err != nil {
		return nil, err
	}

	// Implement default edit rules for documentation files

	// Replace content such as "`terraform plan`" with "`pulumi preview`"
	contentBytes, err := boundedReplace("[tT]erraform [pP]lan", "pulumi preview").Edit(docFile.FileName, []byte(contentStr))
	if err != nil {
		return nil, err
	}
	// Replace content such as "Terraform Apply" with " pulumi up"
	contentBytes, err = boundedReplace("[tT]erraform [aA]pply", "pulumi up").Edit(docFile.FileName, contentBytes)
	if err != nil {
		return nil, err
	}

	// A markdown link that has terraform in the link component.
	contentBytes, err = reReplace(`\[([^\]]*)\]\(.*terraform([^\)]*)\)`, "$1").Edit(docFile.FileName, contentBytes)
	if err != nil {
		return nil, err
	}
	// Replace content such as "jdoe@hashicorp.com" with "jdoe@example.com"
	contentBytes, err = reReplace("@hashicorp.com", "@example.com").Edit(docFile.FileName, contentBytes)
	if err != nil {
		return nil, err
	}
	// Replace all "T/terraform" with "P/pulumi"
	contentBytes, err = reReplace(`[tT]erraform`, `[pP]ulumi`).Edit(docFile.FileName, contentBytes)
	if err != nil {
		return nil, err
	}
	// Replace all "H/hashicorp strings
	contentBytes, err = reReplace(`[hH]ashicorp`, `[pP]ulumi`).Edit(docFile.FileName, contentBytes)
	if err != nil {
		return nil, err
	}
	// Reformat certain headers
	contentBytes, err = reReplace(`The following arguments are supported:`,
		`The following configuration inputs are supported:`).Edit(docFile.FileName, contentBytes)

	// Reformat certain headers
	contentBytes, err = reReplace(`Argument Reference`,
		`Configuration Reference`).Edit(docFile.FileName, contentBytes)
	// Reformat certain headers
	contentBytes, err = reReplace(`block contains the following arguments`,
		`input has the following nested fields`).Edit(docFile.FileName, contentBytes)

	// TODO: Remove upstream-specific sections

	//Reformat field names. Configuration fields are camelCased like nodejs.
	contentStr, _ = reformatText(infoContext{
		language: "nodejs",
		pkg:      g.pkg,
		info:     g.info,
	}, string(contentBytes), nil)

	return []byte(contentStr), nil
}

// writeFrontMatter prepends the registry's expected Hugo frontmatter to our installation-configuration.md doc.
func writeFrontMatter(title string) string {
	return fmt.Sprintf(delimiter+
		"title: %[1]s Installation & Configuration\n"+
		"meta_desc: Provides an overview on how to configure the Pulumi %[1]s.\n"+
		"layout: package\n"+
		delimiter+
		"\n",
		title)
}

// writeIndexFrontMatter prepends the registry's expected Hugo frontmatter to our _index.md doc.
func writeIndexFrontMatter(displayName string) string {
	return fmt.Sprintf(delimiter+
		"title: %s\n"+
		"meta_desc: The %s provider for Pulumi can be used to provision any of the cloud resources available in %s.\n"+
		"layout: package\n"+
		delimiter,
		displayName, displayName, displayName)
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
			"* Python: [`pulumi-%[1]s`](https://pypi.org/project/pulumi-%[1]s/)\n"+ //TODO: get the Go mod version somehow :blood_sob:
			"* Go: [`%[3]s`](https://github.com/pulumi/pulumi-%[1]s)\n"+
			"* .NET: [`Pulumi.%[2]s`](https://www.nuget.org/packages/Pulumi.%[2]s)\n"+ //TODO: use capitalized
			"* Java: [`com.pulumi/%[1]s`](https://central.sonatype.com/artifact/com.pulumi/%[1]s)\n\n",
		providerName,
		cSharpName,
		goImportBasePath,
	)
}

func getBodyAndTitle(content string) (string, string) {
	// The first header in `index.md` is the package name, of the format `# Foo Provider`.
	titleIndex := strings.Index(content, "# ")
	// Get the location for the next newline
	nextNewLine := strings.Index(content[titleIndex:], "\n") + titleIndex
	// Get the title line, without the h1 anchor
	title := content[titleIndex+2 : nextNewLine]
	// strip the title and any front matter
	return content[nextNewLine+1:], title
}

func translateCodeBlocks(contentStr string, g *Generator) (string, error) {

	var returnContent string

	examples := map[string]string{}
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
	outDir, err := os.MkdirTemp("", "configuration-example")
	if err != nil {
		return "", err
	}
	mappingsDir := filepath.Join(outDir, "mappings")
	mappings := []tfbridge.ProviderInfo{
		g.cliConverter().info,
	}
	// Prepare mappings folder
	if len(mappings) > 0 {
		if err := os.MkdirAll(mappingsDir, 0755); err != nil {
			return "", fmt.Errorf("convertViaPulumiCLI: failed to write mappings folder: %w", err)
		}
	}

	// Write out mappings files if necessary.
	for _, info := range mappings {
		info := info // remove aliasing lint
		mpi := tfbridge.MarshalProviderInfo(&info)
		bytes, err := json.Marshal(mpi)
		if err != nil {
			return "", fmt.Errorf("convertViaPulumiCLI: failed to write mappings folder: %w", err)
		}
		mf := g.cliConverter().mappingsFile(mappingsDir, info)
		if err := os.WriteFile(mf, bytes, 0600); err != nil {
			return "", fmt.Errorf("convertViaPulumiCLI: failed to write mappings file: %w", err)
		}
	}

	var mappingsArgs []string
	for _, info := range mappings {
		mappingsArgs = append(mappingsArgs, "--mappings", g.cliConverter().mappingsFile(mappingsDir, info))
	}
	defer func() {
		if err := os.RemoveAll(outDir); err != nil {
			err = fmt.Errorf("failed to clean up installation-examples-output dir: %w", err)
		}
	}()
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
		if fenceLanguage == "```terraform\n" || fenceLanguage == "```hcl\n" ||
			(fenceLanguage == "```\n" && guessIsHCL(code)) {

			//Generate the main.tf file for converting the config file, since --convert-examples doesn't support
			// the Pulumi.yaml config file
			err := os.WriteFile(filepath.Join(outDir, "main.tf"), []byte(code), 0644)
			if err != nil {
				return "", fmt.Errorf("convertViaPulumiCLI: failed to write main.tf file: %w", err)
			}

			// Prepare examples
			// Make an example to record in the cliConverter.
			fileName := fmt.Sprintf("configuration-installation-%d", i)
			examples[fileName] = code

			// Convert to PCL with convertViaPulumiCLI.
			// This gives us a map of translatedExamples.
			translatedExamples, err := g.cliConverter().convertViaPulumiCLI(examples, []tfbridge.ProviderInfo{
				g.cliConverter().info,
			})
			if err != nil {
				return "", err
			}

			// Write the result to the pcls map of our cli converter.
			g.cliConverter().pcls[code] = translatedExample{
				PCL: translatedExamples[fileName].PCL,
			}

			exPath := examplePath{fullPath: fileName}
			var conversionResult *Example
			conversionResult = g.coverageTracker.getOrCreateExample(
				exPath.String(), code)

			langs := genLanguageToSlice(g.language)
			chooserStart := `{{< chooser language "typescript,python,go,csharp,java,yaml" >}}` + "\n"
			returnContent = returnContent + chooserStart
			chooserEnd := "{{< /chooser >}}\n"

			// generate each language in turn, so that we can mark up the output with the correct Hugo shortcodes.
			// TODO: we want to use pulumi code choosers in the future.
			for _, lang := range langs {
				langSlice := []string{lang}
				choosableStart := fmt.Sprintf("{{%% choosable language %s %%}}\n", lang)
				choosableEnd := "\n{{% /choosable %}}\n"

				// Generate the Pulumi.yaml config file for each language
				configFile, err := writeConfigYamlViaCLI(outDir, lang, mappingsArgs)
				// Generate example itself
				convertedLang, err := g.convertHCL(conversionResult, code, exPath.String(), langSlice)
				if err != nil {
					return "", err
				}
				returnContent = returnContent + choosableStart + configFile + convertedLang + choosableEnd
			}
			returnContent += chooserEnd
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

// This function obtains the Pulumi.yaml config file for a given language.
// It relies on the presence of a `main.tf` file for conversion.
func writeConfigYamlViaCLI(outDir, lang string, mappingsArgs []string) (string, error) {
	// Check if there's a main.tf file in the current working directory (outDir)
	if _, err := os.Stat(filepath.Join(outDir, "main.tf")); err != nil {
		return "", fmt.Errorf("expected main.tf file %w", err)
	}
	// Get the pulumi command
	pulumiPath, err := exec.LookPath("pulumi")
	if err != nil {
		return "", fmt.Errorf("couldn't find pulumi path")
	}

	cmdArgs := []string{
		"convert",
		"--from",
		"terraform",
		"--language",
		lang,
		"--generate-only",
	}
	cmdArgs = append(cmdArgs, mappingsArgs...)

	cmd := exec.Command(pulumiPath, cmdArgs...)
	cmd.Dir = outDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("convertViaPulumiCLI: pulumi command failed: %w\n"+
			"Stdout:\n%s\n\n"+
			"Stderr:\n%s\n\n",
			err, stdout.String(), stderr.String())
	}
	pulumiYAMLFile := filepath.Join(outDir, "Pulumi.yaml")
	pulumiYAMLFileBytes, err := os.ReadFile(pulumiYAMLFile)
	if err != nil {
		return "", err
	}
	// Remove the temp directory's random number from project name
	regex := regexp.MustCompile(`configuration-example.*`)
	pulumiYAMLFileBytes = regex.ReplaceAll(pulumiYAMLFileBytes, []byte("configuration-example"))

	configFile := "```yaml\n" +
		"# Provider configuration file\n" +
		string(pulumiYAMLFileBytes) + "\n```\n"
	return configFile, nil
}
