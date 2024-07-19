package tfgen

import (
	"fmt"
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

	//TODO: See https://github.com/pulumi/pulumi-terraform-bridge/issues/2078
	// - translate code blocks with code choosers
	// - reformat TF names
	// - Ability to omit irrelevant sections

	// Apply edit rules to transform the doc for Pulumi-ready presentation
	contentBytes, err := applyEditRules([]byte(contentStr), docFile)
	if err != nil {
		return nil, err
	}
	return contentBytes, nil
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
