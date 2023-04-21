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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/afero"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// argumentDocs contains the documentation metadata for an argument of the resource.
type argumentDocs struct {
	// The description for this argument.
	description string

	// (Optional) The names and descriptions for each argument of this argument.
	arguments map[string]string

	// Whether this argument was derived from a nested object. Used to determine
	// whether to append descriptions that have continued to the following line
	isNested bool
}

// Included for testing convenience.
func (ad argumentDocs) MarshalJSON() ([]byte, error) {
	j, err := json.Marshal(struct {
		Description string
		Arguments   map[string]string
		IsNested    bool
	}{
		Description: ad.description,
		Arguments:   ad.arguments,
		IsNested:    ad.isNested,
	})
	if err != nil {
		return nil, err
	}
	return j, nil
}

// entityDocs represents the documentation for a resource or datasource as extracted from TF markdown.
type entityDocs struct {
	// Description is the description of the resource
	Description string

	// Arguments maps the name of each argument of the resource to its metadata.
	// Each argument has a description. Some arguments have their own arguments.
	//
	// For example, using a couple arguments from s3_bucket.html.markdown, we
	// expect to see a map like this, where "bucket" and "website" are top-level
	// arguments, and "index_document" is an argument of "website":
	//  - bucket
	//  	- description: "(Optional, Forces new resource) The name of the bucket.
	//  		If omitted, Terraform will assign a random, unique name."
	//  - website
	//  	- description: "(Optional) A website object (documented below)."
	//  	- arguments:
	//  		- index_document: "(Required, unless using `redirect_all_requests_to`)
	//				Amazon S3 returns this index document when requests are made to the
	//			 	root domain or any of the subfolders."
	//  - index_document
	//  	- description: "(Required, unless using `redirect_all_requests_to`)
	// 			Amazon S3 returns this index document when requests are made to the
	//			root domain or any of the subfolders."
	//  	- isNested: true
	// "index_document" is recorded like a top level argument since sometimes object names in
	// the TF markdown are inconsistent. For example, see `cors_rule` in s3_bucket.html.markdown.
	Arguments map[string]*argumentDocs

	// Attributes includes the names and descriptions for each attribute of the resource
	Attributes map[string]string

	// Import is the import details for the resource
	Import string
}

func (ed *entityDocs) getOrCreateArgumentDocs(argumentName string) (*argumentDocs, bool) {
	if ed.Arguments == nil {
		ed.Arguments = make(map[string]*argumentDocs)
	}
	var created bool
	args, has := ed.Arguments[argumentName]
	if !has {
		args = &argumentDocs{arguments: make(map[string]string)}
		ed.Arguments[argumentName] = args
		created = true
	}
	return args, created
}

// DocKind indicates what kind of entity's documentation is being requested.
type DocKind string

const (
	// ResourceDocs indicates documentation pertaining to resource entities.
	ResourceDocs DocKind = "resources"
	// DataSourceDocs indicates documentation pertaining to data source entities.
	DataSourceDocs DocKind = "data-sources"
)

var repoPaths sync.Map

func getRepoPath(gitHost string, org string, provider string, version string) (string, error) {
	moduleCoordinates := fmt.Sprintf("%s/%s/terraform-provider-%s", gitHost, org, provider)
	if version != "" {
		moduleCoordinates = fmt.Sprintf("%s/%s", moduleCoordinates, version)
	}

	if repoPathsEnvVar, has := os.LookupEnv("PULUMI_REPO_PATHS"); has {
		path := findRepoPath(repoPathsEnvVar, moduleCoordinates)
		if path != "" {
			return path, nil
		}
	}

	if path, ok := repoPaths.Load(moduleCoordinates); ok {
		return path.(string), nil
	}

	curWd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("error finding current working directory: %w", err)
	}
	if filepath.Base(curWd) != "provider" {
		curWd = filepath.Join(curWd, "provider")
	}

	command := exec.Command("go", "mod", "download", "-json", moduleCoordinates)
	command.Dir = curWd
	output, err := command.CombinedOutput()
	if err != nil {
		msg := "error running 'go mod download -json' in %q dir for module: %w\n\nOutput: %s"
		return "", fmt.Errorf(msg, curWd, err, output)
	}

	target := struct {
		Version string
		Dir     string
		Error   string
	}{}

	if err := json.Unmarshal(output, &target); err != nil {
		return "", fmt.Errorf("error parsing output of 'go mod download -json' for module: %w", err)
	}

	if target.Error != "" {
		return "", fmt.Errorf("error from 'go mod download -json' for module: %s", target.Error)
	}

	repoPaths.Store(moduleCoordinates, target.Dir)

	return target.Dir, nil
}

// findRepoPath returns the value associated first match of the module coordinates.
// repoPathsEnvVar is in the format "github.com/foo/terraform-provider-bar=./terraform-provider-bar"
func findRepoPath(repoPathsEnvVar string, moduleCoordinates string) string {
	for _, provider := range strings.Split(repoPathsEnvVar, ",") {
		parts := strings.SplitN(provider, "=", 2)

		if parts[0] == moduleCoordinates {
			return parts[1]
		}
	}
	return ""
}

func getMarkdownDetails(sink diag.Sink, repoPath, org, provider string,
	resourcePrefix string, kind DocKind, rawname string,
	info tfbridge.ResourceOrDataSourceInfo, providerModuleVersion string, githost string) ([]byte, string, bool) {

	var docinfo *tfbridge.DocInfo
	if info != nil {
		docinfo = info.GetDocs()
	}
	if docinfo != nil && len(docinfo.Markdown) != 0 {
		return docinfo.Markdown, "", true
	}

	if repoPath == "" {
		var err error
		repoPath, err = getRepoPath(githost, org, provider, providerModuleVersion)
		if err != nil {
			msg := "Skip getMarkdownDetails(rawname=%q) because getRepoPath(%q, %q, %q, %q) failed: %v"
			sink.Debugf(&diag.Diag{Message: msg}, rawname, githost, org, provider, providerModuleVersion, err)
			return nil, "", false
		}
	}

	possibleMarkdownNames := []string{
		// Most frequently, docs leave off the provider prefix
		withoutPackageName(resourcePrefix, rawname) + ".html.markdown",
		withoutPackageName(resourcePrefix, rawname) + ".markdown",
		withoutPackageName(resourcePrefix, rawname) + ".html.md",
		withoutPackageName(resourcePrefix, rawname) + ".md",
		// But for some providers, the prefix is included in the name of the doc file
		rawname + ".html.markdown",
		rawname + ".markdown",
		rawname + ".html.md",
		rawname + ".md",
	}

	if docinfo != nil && docinfo.Source != "" {
		possibleMarkdownNames = append(possibleMarkdownNames, docinfo.Source)
	}

	markdownBytes, markdownFileName, found := readMarkdown(repoPath, kind, possibleMarkdownNames)
	if !found {
		return nil, "", false
	}

	return markdownBytes, markdownFileName, true
}

func (k DocKind) String() string {
	switch k {
	case DataSourceDocs:
		return "data source"
	case ResourceDocs:
		return "resource"
	default:
		return ""
	}
}

// formatEntityName checks for an aliases TF entity name (ending with "_legacy") and returns it formatted for end user
// consumption in log messages, etc. Without this conversion, users who do not have direct knowledge of how
// aliased/renamed entities will be confused by the implicit renaming because the "_legacy" resource mapping does not
// appear in the provider mapping, nor the upstream provider source.
func formatEntityName(rawname string) string {
	if strings.Contains(rawname, tfbridge.RenamedEntitySuffix) {
		return fmt.Sprintf("'%s' (aliased or renamed)", strings.Replace(rawname, tfbridge.RenamedEntitySuffix, "", -1))
	}
	return fmt.Sprintf("'%s'", rawname)
}

// getDocsForProvider extracts documentation details for the given package from
// TF website documentation markdown content
func getDocsForProvider(g *Generator, org string, provider string, resourcePrefix string, kind DocKind,
	rawname string, info tfbridge.ResourceOrDataSourceInfo, providerModuleVersion string,
	githost string) (entityDocs, error) {

	if g.skipDocs {
		return entityDocs{}, nil
	}

	markdownBytes, markdownFileName, found := getMarkdownDetails(g.sink, g.info.UpstreamRepoPath, org, provider,
		resourcePrefix, kind, rawname, info, providerModuleVersion, githost)
	if !found {
		entitiesMissingDocs++
		msg := fmt.Sprintf("could not find docs for %v %v. Override the Docs property in the %v mapping. See "+
			"type tfbridge.DocInfo for details.", kind, formatEntityName(rawname), kind)

		if isTruthy(os.Getenv("PULUMI_MISSING_DOCS_ERROR")) {
			g.error(msg)
			return entityDocs{}, fmt.Errorf(msg)
		}

		// Ideally, we would still want to still return an error here and let upstream callers handle it, but at the
		// time the option to fail the build for missing docs was added (see just above), there are multiple callers of
		// this function who do not expect docs not being found to return an error, and the cost of doing the idiomatic
		// thing (returning an error) was too high.
		g.warn(msg)
		return entityDocs{}, nil
	}

	doc, err := parseTFMarkdown(g, info, kind, string(markdownBytes), markdownFileName, resourcePrefix, rawname)
	if err != nil {
		return entityDocs{}, err
	}

	var docinfo *tfbridge.DocInfo
	if info != nil {
		docinfo = info.GetDocs()
	}
	if docinfo != nil {
		// Helper func for readability due to large number of params
		getSourceDocs := func(sourceFrom string) (entityDocs, error) {
			return getDocsForProvider(g, org, provider, resourcePrefix, kind, sourceFrom, nil, providerModuleVersion, githost)
		}

		if docinfo.IncludeAttributesFrom != "" {
			sourceDocs, err := getSourceDocs(docinfo.IncludeAttributesFrom)
			if err != nil {
				return doc, err
			}

			overlayAttributesToAttributes(sourceDocs, doc)
		}

		if docinfo.IncludeAttributesFromArguments != "" {
			sourceDocs, err := getSourceDocs(docinfo.IncludeAttributesFromArguments)
			if err != nil {
				return doc, err
			}

			overlayArgsToAttributes(sourceDocs, doc)
		}

		if docinfo.IncludeArgumentsFrom != "" {
			sourceDocs, err := getSourceDocs(docinfo.IncludeArgumentsFrom)
			if err != nil {
				return doc, err
			}

			overlayArgsToArgs(sourceDocs, doc)
		}
	}

	return doc, nil
}

func overlayAttributesToAttributes(sourceDocs entityDocs, targetDocs entityDocs) {
	for k, v := range sourceDocs.Attributes {
		targetDocs.Attributes[k] = v
	}
}

func overlayArgsToAttributes(sourceDocs entityDocs, targetDocs entityDocs) {
	for k, v := range sourceDocs.Arguments {
		targetDocs.Attributes[k] = v.description
		for kk, vv := range v.arguments {
			targetDocs.Attributes[kk] = vv
		}
	}
}

func overlayArgsToArgs(sourceDocs entityDocs, docs entityDocs) {
	for k, v := range sourceDocs.Arguments { // string -> argument
		arguments := sourceDocs.Arguments[k].arguments
		docArguments := make(map[string]string)
		for kk, vv := range arguments {
			docArguments[kk] = vv
		}
		docs.Arguments[k] = &argumentDocs{
			description: v.description,
			arguments:   docArguments,
		}
	}
}

// checkIfNewDocsExist checks if the new docs root exists
func checkIfNewDocsExist(repo string) bool {
	// Check if the new docs path exists
	newDocsPath := filepath.Join(repo, "docs", "resources")
	_, err := os.Stat(newDocsPath)
	return !os.IsNotExist(err)
}

// getDocsPath finds the correct docs path for the repo/kind
func getDocsPath(repo string, kind DocKind) string {
	// Check if the new docs path exists
	newDocsExist := checkIfNewDocsExist(repo)

	if !newDocsExist {
		// If the new path doesn't exist, use the old docs path.
		kindString := string([]rune(kind)[0]) // We only want the first letter because the old path uses "r" and "d"
		return filepath.Join(repo, "website", "docs", kindString)
	}

	// Otherwise use the new location path.
	kindString := string(kind)
	return filepath.Join(repo, "docs", kindString)
}

// readMarkdown searches all possible locations for the markdown content
func readMarkdown(repo string, kind DocKind, possibleLocations []string) ([]byte, string, bool) {
	locationPrefix := getDocsPath(repo, kind)

	for _, name := range possibleLocations {
		location := filepath.Join(locationPrefix, name)
		markdownBytes, err := os.ReadFile(location)
		if err == nil {
			return markdownBytes, name, true
		}
	}
	return nil, "", false
}

//nolint:lll
var (
	// For example:
	// [1]: https://docs.aws.amazon.com/lambda/latest/dg/welcome.html
	linkFooterRegexp = regexp.MustCompile(`(?m)^(\[\d+\]):\s(.*)`)

	argumentBulletRegexp = regexp.MustCompile(
		"^\\s*[*+-]\\s*`([a-zA-z0-9_]*)`\\s*(\\([a-zA-Z]*\\)\\s*)?\\s*[:–-]?\\s*(\\([^\\)]*\\)[-\\s]*)?(.*)",
	)

	attributeBulletRegexp = regexp.MustCompile(
		"^\\s*[*+-]\\s*`([a-zA-z0-9_]*)`\\s*[:–-]?\\s*(.*)",
	)

	attributionFormatString = "This Pulumi package is based on the [`%[1]s` Terraform Provider](https://%[3]s/%[2]s/terraform-provider-%[1]s)."
)

// groupLines take a slice of strings, lines, and returns a nested slice of strings. When groupLines encounters a line
// that in the input that starts with the supplied string sep, it will begin a new entry in the outer slice.
func groupLines(lines []string, sep string) [][]string {
	var buffer []string
	var sections [][]string
	for _, line := range lines {
		if strings.Index(line, sep) == 0 {
			sections = append(sections, buffer)
			buffer = []string{}
		}
		buffer = append(buffer, line)
	}
	if len(buffer) > 0 {
		sections = append(sections, buffer)
	}
	return sections
}

// splitGroupLines splits and groups a string, s, by a given separator, sep.
func splitGroupLines(s, sep string) [][]string {
	return groupLines(strings.Split(s, "\n"), sep)
}

// parseTFMarkdown takes a TF website markdown doc and extracts a structured representation for use in
// generating doc comments
func parseTFMarkdown(g *Generator, info tfbridge.ResourceOrDataSourceInfo, kind DocKind,
	markdown, markdownFileName, resourcePrefix, rawname string) (entityDocs, error) {

	p := &tfMarkdownParser{
		g:                g,
		info:             info,
		kind:             kind,
		markdown:         markdown,
		markdownFileName: markdownFileName,
		resourcePrefix:   resourcePrefix,
		rawname:          rawname,
	}
	return p.parse()
}

type tfMarkdownParser struct {
	g                *Generator
	info             tfbridge.ResourceOrDataSourceInfo
	kind             DocKind
	markdown         string
	markdownFileName string
	resourcePrefix   string
	rawname          string

	ret entityDocs
}

const (
	sectionOther               = 0
	sectionExampleUsage        = 1
	sectionArgsReference       = 2
	sectionAttributesReference = 3
	sectionFrontMatter         = 4
	sectionImports             = 5
)

func (p *tfMarkdownParser) parseSupplementaryExamples() (string, error) {
	examplesFileName := fmt.Sprintf("docs/%s/%s.examples.md", p.kind, p.rawname)
	absPath, err := filepath.Abs(examplesFileName)
	if err != nil {
		return "", err
	}
	fileBytes, err := os.ReadFile(absPath)
	if err != nil {
		p.g.error("explicitly marked resource documention for replacement, but found no file at %q", examplesFileName)
		return "", err
	}

	return string(fileBytes), nil
}

func (p *tfMarkdownParser) parse() (entityDocs, error) {
	p.ret = entityDocs{
		Arguments:  make(map[string]*argumentDocs),
		Attributes: make(map[string]string),
	}

	// Replace any Windows-style newlines.
	markdown := strings.Replace(p.markdown, "\r\n", "\n", -1)

	// Replace redundant comment.
	markdown = strings.Replace(markdown, "<!-- schema generated by tfplugindocs -->", "", -1)

	// Split the sections by H2 topics in the Markdown file.
	sections := splitGroupLines(markdown, "## ")

	// we are explicitly overwriting the Terraform examples here
	if p.info != nil && p.info.GetDocs() != nil && p.info.ReplaceExamplesSection() {
		for i, section := range sections {
			// Let's remove any existing examples usage we have in our parsed documentation
			if len(section) > 0 && strings.Contains(section[0], "Example Usage") {
				sections = append(sections[:i], sections[i+1:]...)

				break
			}
		}

		// now we are going to inject the new source of examples
		newExamples, err := p.parseSupplementaryExamples()
		if err != nil {
			return entityDocs{}, err
		}
		newSection := strings.Split(newExamples, "\n")
		sections = append(sections, newSection)
	} else {
		// Reparent examples that are peers of the "Example Usage" section (if any) and fixup some example titles.
		sections = reformatExamples(sections)
	}

	for _, section := range sections {
		if err := p.parseSection(section); err != nil {
			return entityDocs{}, err
		}
	}

	// Get links.
	footerLinks := getFooterLinks(markdown)

	doc, _ := cleanupDoc(p.rawname, p.g, p.ret, footerLinks)

	return doc, nil
}

// fixExampleTitles transforms H4 sections that contain code snippets into H3 sections.
func fixExampleTitles(lines []string) {
	inSection, sectionIndex := false, 0
	for i, line := range lines {
		if inSection && strings.HasPrefix(line, "```") {
			lines[sectionIndex] = strings.Replace(lines[sectionIndex], "#### ", "### ", 1)
			inSection = false
		} else if strings.HasPrefix(line, "#### ") {
			inSection, sectionIndex = true, i
		}
	}
}

var exampleHeaderRegexp = regexp.MustCompile(`(?i)^(## Example Usage\s*)(?:(?:(?:for|of|[\pP]+)\s*)?(.*?)\s*)?$`)

// reformatExamples reparents examples that are peers of the "Example Usage" section (if any) and fixup some example
// titles.
func reformatExamples(sections [][]string) [][]string {
	canonicalExampleUsageSectionIndex := -1
	var exampleUsageSection []string
	var exampleSectionIndices []int
	for i, s := range sections {
		matches := exampleHeaderRegexp.FindStringSubmatch(s[0])
		if len(matches) == 0 {
			continue
		}

		if len(matches[1]) == len(s[0]) {
			// This is the canonical example usage section. Prepend its contents to any other content we've collected.
			// If there are multiple canonical example usage sections, treat the first such section as the canonical
			// example usage section and append other sections under an H3.
			if canonicalExampleUsageSectionIndex == -1 {
				canonicalExampleUsageSectionIndex = i

				// Copy the section over. Note that we intentionally avoid copying the first line and any whitespace
				// that follows it, as we will overwrite that content with the canonical header later.
				for s = s[1:]; len(s) > 0 && isBlank(s[0]); s = s[1:] {
				}

				sectionCopy := make([]string, len(s)+2)
				copy(sectionCopy[2:], s)

				if len(exampleUsageSection) != 0 {
					exampleUsageSection = append(sectionCopy, exampleUsageSection...)
				} else {
					exampleUsageSection = sectionCopy
				}
			} else {
				exampleUsageSection = append(exampleUsageSection, "", "### Additional Examples")
				exampleUsageSection = append(exampleUsageSection, s[1:]...)
			}
		} else if strings.Contains(s[0], "## Example Usage -") {
			// this is a specific usecase where all of the examples are being requalified as top level examples with a
			// title. We should process these as children of the top level examples
			exampleUsageSection = append(exampleUsageSection, "### "+cases.Title(language.Und, cases.NoLower).String(matches[2]))
			exampleUsageSection = append(exampleUsageSection, s[1:]...)
		} else {
			// This is a qualified example usage section. Retitle it using an H3 and its qualifier, and append it to
			// the output.
			exampleUsageSection = append(exampleUsageSection, "", "### "+
				cases.Title(language.Und, cases.NoLower).String(matches[2]))
			exampleUsageSection = append(exampleUsageSection, s[1:]...)
		}

		exampleSectionIndices = append(exampleSectionIndices, i)
	}

	if len(exampleSectionIndices) == 0 {
		return sections
	}

	// If we did not find a canonical example usage section, prepend a blank line to the output. This line will be
	// replaced by the canonical example usage H2.
	if canonicalExampleUsageSectionIndex == -1 {
		canonicalExampleUsageSectionIndex = exampleSectionIndices[0]
		exampleUsageSection = append([]string{""}, exampleUsageSection...)
	}

	// Ensure that the output begins with the canonical example usage header.
	exampleUsageSection[0] = "## Example Usage"

	// Fixup example titles and replace the contents of the canonical example usage section with the output.
	fixExampleTitles(exampleUsageSection)
	sections[canonicalExampleUsageSectionIndex] = exampleUsageSection

	// If there is only one example section, we're done. Otherwise, we need to remove all non-canonical example usage
	// sections.
	if len(exampleSectionIndices) == 1 {
		return sections
	}

	result := sections[:0]
	for i, s := range sections {
		if len(exampleSectionIndices) > 0 && i == exampleSectionIndices[0] {
			exampleSectionIndices = exampleSectionIndices[1:]
			if i != canonicalExampleUsageSectionIndex {
				continue
			}
		}
		result = append(result, s)
	}
	return result
}

func (p *tfMarkdownParser) parseSection(h2Section []string) error {
	// Extract the header name, since this will drive how we process the content.
	if len(h2Section) == 0 {
		p.g.warn("Unparseable H2 doc section for %v; consider overriding doc source location", p.rawname)
		return nil
	}

	// Skip certain headers that we don't support.
	header := h2Section[0]
	if strings.Index(header, "## ") == 0 {
		header = header[3:]
	}

	sectionKind := sectionOther

	switch header {
	case "Timeout", "Timeouts", "User Project Override", "User Project Overrides":
		p.g.debug("Ignoring doc section [%v] for [%v]", header, p.rawname)
		ignoredDocHeaders[header]++
		return nil
	case "Example Usage":
		sectionKind = sectionExampleUsage
	case "Arguments Reference", "Argument Reference", "Argument reference", "Nested Blocks", "Nested blocks":
		sectionKind = sectionArgsReference
	case "Attributes Reference", "Attribute Reference", "Attribute reference":
		sectionKind = sectionAttributesReference
	case "Import", "Imports":
		sectionKind = sectionImports
	case "---":
		sectionKind = sectionFrontMatter
	case "Schema":
		p.parseSchemaWithNestedSections(h2Section)
		return nil
	}

	// Now split the sections by H3 topics. This is done because we'll ignore sub-sections with code
	// snippets that are unparseable (we don't want to ignore entire H2 sections).
	var wroteHeader bool
	for _, h3Section := range groupLines(h2Section[1:], "### ") {
		if len(h3Section) == 0 {
			// An unparseable H3 appears (as observed by building a few tier 1 providers) to typically be due to an
			// empty section resulting from how we parse sections earlier in the docs generation process. Therefore, we
			// log it as debug output:
			p.g.debug("empty or unparseable H3 doc section for %v; consider overriding doc source location", p.rawname, p.kind)
			continue
		}

		// Remove the "Open in Cloud Shell" button if any and check for the presence of code snippets.
		reformattedH3Section, hasExamples, isEmpty := p.reformatSubsection(h3Section)
		if isEmpty {
			// Skip empty subsections (they just add unnecessary padding and headers).
			continue
		}
		if hasExamples && sectionKind != sectionExampleUsage && sectionKind != sectionImports &&
			!p.info.ReplaceExamplesSection() {
			p.g.warn("Unexpected code snippets in section '%v' for %v '%v'. The HCL code will be converted if possible, "+
				"but may not display correctly in the generated docs.", header, p.kind, p.rawname)
			unexpectedSnippets++
		}

		// Now process the content based on the H2 topic. These are mostly standard across TF's docs.
		switch sectionKind {
		case sectionArgsReference:
			p.parseArgReferenceSection(reformattedH3Section)
		case sectionAttributesReference:
			p.parseAttributesReferenceSection(reformattedH3Section)
		case sectionFrontMatter:
			p.parseFrontMatter(reformattedH3Section)
		case sectionImports:
			p.parseImports(reformattedH3Section)
		default:
			// Determine if this is a nested argument section.
			_, isArgument := p.ret.Arguments[header]
			if isArgument || strings.HasSuffix(header, "Configuration Block") {
				p.parseArgReferenceSection(reformattedH3Section)
				continue
			}

			// For all other sections, append them to the description section.
			if !wroteHeader {
				p.ret.Description += fmt.Sprintf("## %s\n", header)
				wroteHeader = true
				if !isBlank(reformattedH3Section[0]) {
					p.ret.Description += "\n"
				}
			}
			p.ret.Description += strings.Join(reformattedH3Section, "\n") + "\n"
		}
	}

	return nil
}

func getFooterLinks(markdown string) map[string]string {
	links := make(map[string]string)
	lines := strings.Split(markdown, "\n")
	for _, line := range lines {
		matches := linkFooterRegexp.FindStringSubmatch(line)
		if len(matches) == 3 {
			links[matches[1]] = matches[2]
		}
	}
	return links
}

func (p *tfMarkdownParser) parseSchemaWithNestedSections(subsection []string) {
	node := parseNode(strings.Join(subsection, "\n"))
	topLevelSchema, err := parseTopLevelSchema(node, nil)
	if err != nil {
		p.g.warn(fmt.Sprintf("error: Failure in parsing resource name: %s, subsection: %s", p.rawname, subsection[0]))
		return
	}
	if topLevelSchema == nil {
		p.g.warn("Failed to parse top-level Schema section")
		return
	}
	parseTopLevelSchemaIntoDocs(&p.ret, topLevelSchema, p.g.warn)
}

// parseArgFromMarkdownLine takes a line of Markdown and attempts to parse it for a Terraform argument and its
// description
func parseArgFromMarkdownLine(line string) (string, string, bool) {
	matches := argumentBulletRegexp.FindStringSubmatch(line)

	if len(matches) > 4 {
		return matches[1], matches[4], true
	}

	return "", "", false
}

// getNestedBlockName take a line of a Terraform docs Markdown page and returns the name of the nested block it
// describes. If the line does not describe a nested block, an empty string is returned.
//
// Examples of nested blocks include (but are not limited to):
//
// - "The `private_cluster_config` block supports:" -> "private_cluster_config"
// - "The optional settings.backup_configuration subblock supports:" -> "settings.backup_configuration"
func getNestedBlockName(line string) string {
	nested := ""

	nestedObjectRegexps := []*regexp.Regexp{
		// For example:
		// s3_bucket.html.markdown: "The `website` object supports the following:"
		// ami.html.markdown: "When `virtualization_type` is "hvm" the following additional arguments apply:"
		regexp.MustCompile("`([a-z_]+)`.*following"),

		// For example:
		// athena_workgroup.html.markdown: "#### result_configuration Argument Reference"
		regexp.MustCompile("(?i)## ([a-z_]+).* argument reference"),

		// For example:
		// elasticsearch_domain.html.markdown: "### advanced_security_options"
		regexp.MustCompile("###+ ([a-z_]+).*"),

		// For example:
		// dynamodb_table.html.markdown: "### `server_side_encryption`"
		regexp.MustCompile("###+ `([a-z_]+).*`"),

		// For example:
		// route53_record.html.markdown: "### Failover Routing Policy"
		regexp.MustCompile("###+ ([a-zA-Z_ ]+).*"),

		// For example:
		// sql_database_instance.html.markdown:
		// "The optional `settings.ip_configuration.authorized_networks[]`` sublist supports:"
		regexp.MustCompile("`([a-zA-Z_.\\[\\]]+)`.*supports:"),
	}

	for _, match := range nestedObjectRegexps {
		matches := match.FindStringSubmatch(line)
		if len(matches) >= 2 {
			nested = strings.ToLower(matches[1])
			nested = strings.Replace(nested, " ", "_", -1)
			nested = strings.TrimSuffix(nested, "[]")
			parts := strings.Split(nested, ".")
			nested = parts[len(parts)-1]
			break
		}
	}

	return nested
}

func (p *tfMarkdownParser) parseArgReferenceSection(subsection []string) {
	var lastMatch, nested string
	for _, line := range subsection {
		name, desc, matchFound := parseArgFromMarkdownLine(line)

		if matchFound {
			// found a property bullet, extract the name and description
			if nested != "" {
				// We found this line within a nested field. We should record it as such.
				if p.ret.Arguments[nested] == nil {
					p.ret.Arguments[nested] = &argumentDocs{
						arguments: make(map[string]string),
					}
					totalArgumentsFromDocs++
				} else if p.ret.Arguments[nested].arguments == nil {
					p.ret.Arguments[nested].arguments = make(map[string]string)
				}
				p.ret.Arguments[nested].arguments[name] = desc

				// Also record this as a top-level argument just in case, since sometimes the recorded nested
				// argument doesn't match the resource's argument.
				// For example, see `cors_rule` in s3_bucket.html.markdown.
				if p.ret.Arguments[name] == nil {
					p.ret.Arguments[name] = &argumentDocs{
						description: desc,
						isNested:    true, // Mark that this argument comes from a nested field.
					}
				}
			} else {
				if !strings.HasSuffix(line, "supports the following:") {
					p.ret.Arguments[name] = &argumentDocs{description: desc}
					totalArgumentsFromDocs++
				}
			}
			lastMatch = name
		} else if !isBlank(line) && lastMatch != "" {
			// this is a continuation of the previous bullet
			if nested != "" {
				p.ret.Arguments[nested].arguments[lastMatch] += "\n" + strings.TrimSpace(line)

				// Also update the top-level argument if we took it from a nested field.
				if p.ret.Arguments[lastMatch].isNested {
					p.ret.Arguments[lastMatch].description += "\n" + strings.TrimSpace(line)
				}
			} else {
				p.ret.Arguments[lastMatch].description += "\n" + strings.TrimSpace(line)
			}
		} else {
			// This line might declare the beginning of a nested object.
			// If we do not find a "nested", then this is an empty line or there were no bullets yet.
			nestedBlockCurrentLine := getNestedBlockName(line)

			if nestedBlockCurrentLine != "" {
				nested = nestedBlockCurrentLine
			}

			// Clear the lastMatch.
			lastMatch = ""
		}
	}
}

func (p *tfMarkdownParser) parseAttributesReferenceSection(subsection []string) {
	var lastMatch string
	for _, line := range subsection {
		matches := attributeBulletRegexp.FindStringSubmatch(line)
		if len(matches) >= 2 {
			// found a property bullet, extract the name and description
			p.ret.Attributes[matches[1]] = matches[2]
			lastMatch = matches[1]
		} else if !isBlank(line) && lastMatch != "" {
			// this is a continuation of the previous bullet
			p.ret.Attributes[lastMatch] += "\n" + strings.TrimSpace(line)
		} else {
			// This is an empty line or there were no bullets yet - clear the lastMatch
			lastMatch = ""
		}
	}
}

func (p *tfMarkdownParser) parseImports(subsection []string) {
	// check for import overwrites
	info := p.info
	if info != nil {
		docInfo := info.GetDocs()
		if docInfo != nil {
			importDetails := docInfo.ImportDetails
			if importDetails != "" {
				p.ret.Import = fmt.Sprintf("## Import\n\n%s", importDetails)
				return
			}
		}
	}

	var importDocString []string
	for _, section := range subsection {
		if strings.Contains(section, "**NOTE:") || strings.Contains(section, "**Please Note:") ||
			strings.Contains(section, "**Note:**") {
			// This is a Terraform import specific comment that we don't need to parse or include in our docs
			continue
		}

		// Skip another redundant comment
		if strings.Contains(section, "Import is supported using the following syntax") {
			continue
		}

		// Remove the shell comment characters to avoid writing this line as a Markdown H1:
		section = strings.TrimPrefix(section, "# ")

		// There are multiple variations of codeblocks for import syntax
		section = strings.Replace(section, "```shell", "", -1)
		section = strings.Replace(section, "```sh", "", -1)
		section = strings.Replace(section, "```", "", -1)

		if strings.Contains(section, "terraform import") {
			// First, remove the `$`
			section := strings.Replace(section, "$ ", "", -1)
			// Next, remove `terraform import` from the codeblock
			section = strings.Replace(section, "terraform import ", "", -1)
			importString := ""
			parts := strings.Split(section, " ")
			for i, p := range parts {
				switch i {
				case 0:
					if !isBlank(p) {
						// split the string on . and take the last item
						// this gets the identifier broken from the tf resource
						ids := strings.Split(p, ".")
						name := ids[len(ids)-1]
						importString = fmt.Sprintf("%s %s", importString, name)
					}
				default:
					if !isBlank(p) {
						importString = fmt.Sprintf("%s %s", importString, p)
					}
				}
			}
			var tok string
			if p.info != nil && p.info.GetTok() != "" {
				tok = p.info.GetTok().String()
			} else {
				tok = "MISSING_TOK"
			}
			// We are going to use a placeholder here for the linebreak so that when we get into converting examples
			// we can format our Import section outside of the examples section
			importCommand := fmt.Sprintf("$ pulumi import %s%s", tok, importString)
			importDetails := []string{"<break><break>```sh<break>", importCommand, "<break>```<break><break>"}
			importDocString = append(importDocString, importDetails...)
		} else {
			if !isBlank(section) {
				importDocString = append(importDocString, section)
			}
		}
	}

	if len(importDocString) > 0 {
		p.ret.Import = fmt.Sprintf("## Import\n\n%s", strings.Join(importDocString, " "))
	}
}

func (p *tfMarkdownParser) parseFrontMatter(subsection []string) {
	// The header of the MarkDown will have two "---"s paired up to delineate the header. Skip this.
	var foundEndHeader bool
	for len(subsection) > 0 {
		curr := subsection[0]
		subsection = subsection[1:]
		if curr == "---" {
			foundEndHeader = true
			break
		}
	}
	if !foundEndHeader {
		p.g.warn("", "Expected to pair --- begin/end for resource %v's Markdown header", p.rawname)
	}

	// Now extract the description section. We assume here that the first H1 (line starting with #) is the name
	// of the resource, because we aren't detecting code fencing. Comments in HCL are prefixed with # (the
	// same as H1 in Markdown, so we treat further H1's in this section as part of the description. If there
	// are no matching H1s, we emit a warning for the resource as it is likely a problem with the documentation.
	lastBlank := true
	var foundH1Resource bool
	for _, line := range subsection {
		if strings.Index(line, "# ") == 0 {
			foundH1Resource = true
			lastBlank = true
		} else if !isBlank(line) || !lastBlank {
			p.ret.Description += line + "\n"
			lastBlank = false
		} else if isBlank(line) {
			lastBlank = true
		}
	}
	if !foundH1Resource {
		p.g.warn("Expected an H1 in markdown for resource %v", p.rawname)
	}
}

// isBlank returns true if the line is all whitespace.
func isBlank(line string) bool {
	return strings.TrimSpace(line) == ""
}

// reformatSubsection strips any "Open in Cloud Shell" buttons from the subsection and detects the presence of example
// code snippets.
func (p *tfMarkdownParser) reformatSubsection(lines []string) ([]string, bool, bool) {
	var result []string
	hasExamples, isEmpty := false, true

	var inOICSButton bool // True if we are removing an "Open in Cloud Shell" button.
	for i, line := range lines {
		if inOICSButton {
			if strings.Index(lines[i], "</div>") == 0 {
				inOICSButton = false
			}
		} else {
			if strings.Index(line, "<div") == 0 && strings.Contains(line, "oics-button") {
				inOICSButton = true
			} else {
				if strings.Index(line, "```") == 0 {
					hasExamples = true
				} else if !isBlank(line) {
					isEmpty = false
				}

				result = append(result, line)
			}
		}
	}

	return result, hasExamples, isEmpty
}

// convertExamples converts any code snippets in a subsection to Pulumi-compatible code. This conversion is done on a
// per-subsection basis; subsections with failing examples will be elided upon the caller's request.
func (g *Generator) convertExamples(docs string, path examplePath, stripSubsectionsWithErrors bool) string {
	if docs == "" {
		return ""
	}

	if strings.Contains(docs, "```typescript") || strings.Contains(docs, "```python") ||
		strings.Contains(docs, "```go") || strings.Contains(docs, "```yaml") ||
		strings.Contains(docs, "```csharp") || strings.Contains(docs, "```java") {
		// we have explicitly rewritten these examples and need to just return them directly rather than trying
		// to reconvert them. But we need to surround them in the examples shortcode for rendering on the registry
		return fmt.Sprintf("{{%% examples %%}}\n%s\n{{%% /examples %%}}", docs)
	}

	output := &bytes.Buffer{}

	writeTrailingNewline := func(buf *bytes.Buffer) {
		if b := buf.Bytes(); len(b) > 0 && b[len(b)-1] != '\n' {
			buf.WriteByte('\n')
		}
	}
	fprintf := func(w io.Writer, f string, args ...interface{}) {
		_, err := fmt.Fprintf(w, f, args...)
		contract.IgnoreError(err)
	}

	for _, section := range splitGroupLines(docs, "## ") {
		if len(section) == 0 {
			continue
		}

		isImportSection := false
		header, wroteHeader := section[0], false
		isFrontMatter, isExampleUsage := !strings.HasPrefix(header, "## "), header == "## Example Usage"

		if stripSubsectionsWithErrors && header == "## Import" {
			isImportSection = true
			isFrontMatter = false
			wroteHeader = true
		}

		sectionStart, sectionEnd := "", ""
		if isExampleUsage {
			sectionStart, sectionEnd = "{{% examples %}}\n", "{{% /examples %}}"
		}

		for _, subsection := range groupLines(section[1:], "### ") {

			// Each `Example ...` section contains one or more examples written in HCL, optionally separated by
			// comments about the examples. We will attempt to convert them using our `tf2pulumi` tool, and append
			// them to the description. If we can't, we'll simply log a warning and keep moving along.
			subsectionOutput := &bytes.Buffer{}
			skippedExamples, hasExamples := false, false
			inCodeBlock, codeBlockStart := false, 0
			for i, line := range subsection {
				if isImportSection {
					// we don't want to do anything with the import section
					continue
				}
				if inCodeBlock {
					if strings.Index(line, "```") != 0 {
						continue
					}

					if g.language.shouldConvertExamples() {
						hcl := strings.Join(subsection[codeBlockStart+1:i], "\n")

						// We've got some code -- assume it's HCL and try to convert it.
						g.coverageTracker.foundExample(path.String(), hcl)

						exampleTitle := ""
						if strings.Contains(subsection[0], "###") {
							exampleTitle = strings.Replace(subsection[0], "### ", "", -1)
						}

						langs := genLanguageToSlice(g.language)
						codeBlock, err := g.convertHCL(hcl, path.String(), exampleTitle, langs)

						if err != nil {
							skippedExamples = true
						} else {
							fprintf(subsectionOutput, "\n%s", codeBlock)
						}
					} else {
						skippedExamples = true
					}

					hasExamples = true
					inCodeBlock = false
				} else {
					if strings.Index(line, "```") == 0 {
						inCodeBlock, codeBlockStart = true, i
					} else {
						fprintf(subsectionOutput, "\n%s", line)
					}
				}
			}
			if inCodeBlock {
				skippedExamples = true
			}

			// If the subsection contained skipped examples and the caller has requested that we remove such subsections,
			// do not append its text to the output. Note that we never elide front matter.
			if skippedExamples && stripSubsectionsWithErrors && !isFrontMatter {
				continue
			}

			if !wroteHeader {
				if output.Len() > 0 {
					fprintf(output, "\n")
				}
				fprintf(output, "%s%s", sectionStart, header)
				wroteHeader = true
			}
			if hasExamples && isExampleUsage {
				writeTrailingNewline(output)
				fprintf(output, "{{%% example %%}}%s", subsectionOutput.String())
				writeTrailingNewline(output)
				fprintf(output, "{{%% /example %%}}")
			} else {
				fprintf(output, "%s", subsectionOutput.String())
			}
		}

		if isImportSection {
			section[0] = "\n\n## Import"
			importDetails := strings.Join(section, " ")
			importDetails = strings.Replace(importDetails, "  ", "\n\n", -1)
			importDetails = strings.Replace(importDetails, "<break>", "\n", -1)
			importDetails = strings.Replace(importDetails, ": ", "", -1)
			importDetails = strings.Replace(importDetails, " \n", "\n", -1)
			fprintf(output, "%s", importDetails)
			continue
		}

		if !wroteHeader {
			if isFrontMatter {
				fprintf(output, "%s", header)
			}
		} else if sectionEnd != "" {
			writeTrailingNewline(output)
			fprintf(output, "%s", sectionEnd)
		}
	}
	return output.String()
}

// A ConversionError occurs when convert.Convert yields a panic.
// This can be removed when https://github.com/pulumi/pulumi-terraform-bridge/issues/477
// is resolved. ConversionError exposes the stacktrace of the panic so callers
// can choose to pass the trace along to the user or swallow it.
type ConversionError struct {
	// panicArg is the argument that was passed to panic() during conversion.
	panicArg interface{}
	// trace is the captured stacktrace.
	trace string
	// wrappedErr is the error message provided by this struct.
	wrappedErr error
}

// construct a new ConversionError. The argument is expected to be
// the value that was recovered from the panic.
func newConversionError(panicArg interface{}, trace string) *ConversionError {
	var err = fmt.Errorf("panic converting HCL: %s", panicArg)
	return &ConversionError{
		panicArg:   panicArg,
		trace:      trace,
		wrappedErr: err,
	}
}

// StackTrace returns the stacktrace of the error.
func (err *ConversionError) StackTrace() string {
	return err.trace
}

// Return the err-representation of this struct.
func (err *ConversionError) Error() string {
	return err.wrappedErr.Error()
}

// Unwrap provides error as returned by the conversion panic.
func (err *ConversionError) Unwrap() error {
	return err.wrappedErr
}

// Statically enforce that ConversionError implements the Error interface.
var _ error = &ConversionError{}

// convert wraps convert.Convert so that it returns an error in the event of a panic in convert.Convert
//
// Note: If this issue is fixed, the call to convert.Convert can be unwrapped and this function can be deleted:
// https://github.com/pulumi/pulumi-terraform-bridge/issues/477
func (g *Generator) convert(input afero.Fs, languageName string) (files map[string][]byte, diags convert.Diagnostics,
	err error) {
	defer func() {
		v := recover()
		if v != nil {
			files = map[string][]byte{}
			diags = convert.Diagnostics{}
			var trace = string(debug.Stack())
			err = newConversionError(v, trace)
			g.coverageTracker.languageConversionPanic(languageName, fmt.Sprintf("%v", v))
		}
	}()

	files, diags, err = convert.Convert(convert.Options{
		Loader:                   newLoader(g.pluginHost),
		Root:                     input,
		TargetLanguage:           languageName,
		AllowMissingProperties:   true,
		AllowMissingVariables:    true,
		FilterResourceNames:      true,
		PackageCache:             g.packageCache,
		PluginHost:               g.pluginHost,
		ProviderInfoSource:       g.infoSource,
		SkipResourceTypechecking: true,
		TerraformVersion:         g.terraformVersion,
	})

	return
}

// convertHCLToString hides the implementation details of the upstream implementation for HCL conversion and provides
// simplified parameters and return values
func (g *Generator) convertHCLToString(hcl, path, languageName string) (string, error) {
	input := afero.NewMemMapFs()
	fileName := fmt.Sprintf("/%s.tf", strings.ReplaceAll(path, "/", "-"))
	f, err := input.Create(fileName)
	contract.AssertNoErrorf(err, "err != nil")
	_, err = f.Write([]byte(hcl))
	contract.AssertNoErrorf(err, "err != nil")
	contract.IgnoreClose(f)

	files, diags, err := g.convert(input, languageName)

	// By observation on the GCP provider, convert.Convert() will either panic (in which case the wrapped method above
	// will return an error) or it will return a non-zero value for diags.
	if err != nil {
		// Because this condition is presumably the result of a panic that we wrap as an error, we do not need to add
		// anything to g.coverageTracker - that's covered in the panic recovery above.
		var convErr *ConversionError
		if errors.As(err, &convErr) {
			g.debug("Printing stack trace for panic: %v", convErr.StackTrace)
		}
		return "", fmt.Errorf("failed to convert HCL for %s to %v: %w", path, languageName, err)
	}
	if diags.All.HasErrors() {
		// Remove the temp filename from the error, since it will be confusing to users of the bridge who do not know
		// we write an example to a temp file internally in order to pass to convert.Convert().
		//
		// fileName starts with a "/" which is not present in the resulting error, so we need to skip the first rune.
		errMsg := strings.ReplaceAll(diags.All.Error(), fileName[1:], "")

		g.warn("failed to convert HCL for %s to %v: %v", path, languageName, errMsg)
		g.coverageTracker.languageConversionFailure(languageName, diags.All)
		return "", fmt.Errorf(errMsg)
	}

	contract.Assertf(len(files) == 1, `len(files) == 1`)

	convertedHcl := ""
	for _, output := range files {
		convertedHcl = strings.TrimSpace(string(output))
	}

	g.coverageTracker.languageConversionSuccess(languageName, convertedHcl)
	return convertedHcl, nil
}

// So we can sort the keys of a map of examples in a deterministic order:
type languages []string

func (s languages) Len() int      { return len(s) }
func (s languages) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s languages) Less(i, j int) bool {

	notFound := -1

	indexOf := func(item string, data []string) int {
		for i, v := range data {
			if item == v {
				return i
			}
		}
		return notFound
	}

	var languages languages = []string{
		convert.LanguageTypescript,
		convert.LanguagePython,
		convert.LanguageCSharp,
		convert.LanguageGo,
	}

	ii := indexOf(s[i], languages)
	jj := indexOf(s[j], languages)

	if ii != notFound && jj != notFound {
		// Both are found, so just compare indices:
		return ii < jj
	} else if ii != notFound && jj == notFound {
		// Only the first item is found, so it must come first:
		return true
	} else if ii == notFound && jj != notFound {
		// Only the second item is found, so it must come first:
		return false
	} else {
		// Fall back to alphabetical if neither are found:
		return s[i] < s[j]
	}
}

// hclMapToString takes a map of hclConversions to various languages and returns a single Markdown string for use in
// Pulumi's docs. The key in hclConversions is expected to match the language hint in the generated code fences, e.g.
// "typescript" -> ```typescript, and the corresponding value is expected to be the converted code.
func hclConversionsToString(hclConversions map[string]string) string {
	var result strings.Builder

	// We have to use a custom comparator to get the keys to iterate in a deterministic order. We need a deterministic
	// order to ensure that we have no changes to the schema when deploying this change, as well as to establish
	// reliable tests for this function because Go iterates map keys in a non-deterministic fashion:
	var keys languages = []string{}
	for k := range hclConversions {
		keys = append(keys, k)
	}
	sort.Sort(keys)

	for _, key := range keys {
		convertedHcl := strings.TrimSpace(hclConversions[key])

		if convertedHcl == "" {
			continue
		}

		if result.Len() > 0 {
			result.WriteByte('\n')
		}

		_, err := fmt.Fprintf(&result, "```%s\n%s\n```", key, convertedHcl)
		contract.IgnoreError(err)
	}

	return result.String()
}

// convertHCL takes a string of example HCL, its path in the Pulumi schema, the title of the example in the upstream
// docs, and a slice of the languages to convert the sample to, and returns a string containing a series of Markdown
// code blocks with the example converted in each supplied language.
// If all languages fail to convert, the returned string will be "" and an error will be returned.
// If some languages fail to convert, the returned string contain any successful conversions and no error will be
// returned, but conversion failures will be logged via the Generator.
func (g *Generator) convertHCL(hcl, path, exampleTitle string, languages []string) (string, error) {
	g.debug("converting HCL for %s", path)

	// Fixup the HCL as necessary.
	if fixed, ok := fixHcl(hcl); ok {
		hcl = fixed
	}

	hclConversions := map[string]string{}
	var result strings.Builder
	var err error

	failedLangs := map[string]error{}

	for _, lang := range languages {
		var convertErr error
		hclConversions[lang], convertErr = g.convertHCLToString(hcl, path, lang)
		if convertErr != nil {
			failedLangs[lang] = convertErr
			err = multierror.Append(err, convertErr)
		}
	}

	result.WriteString(hclConversionsToString(hclConversions))
	if len(failedLangs) == 0 {
		return result.String(), nil
	}

	isCompleteFailure := len(failedLangs) == len(languages)

	if isCompleteFailure {
		hclAllLangsConversionFailures++
		if exampleTitle == "" {
			g.warn(fmt.Sprintf("unable to convert HCL example for Pulumi entity '%s': %v. The example will be dropped "+
				"from any generated docs or SDKs.", path, err))
		} else {
			g.warn(fmt.Sprintf("unable to convert HCL example '%s' for Pulumi entity '%s': %v. The example will be "+
				"dropped from any generated docs or SDKs.", exampleTitle, path, err))
		}

		return "", err
	}

	// Log the results when an example fails to convert to some languages, but not all
	var failedLangsStrings []string

	for lang := range failedLangs {
		failedLangsStrings = append(failedLangsStrings, lang)

		switch lang {
		case convert.LanguageTypescript:
			hclTypeScriptPartialConversionFailures++
		case convert.LanguagePython:
			hclPythonPartialConversionFailures++
		case convert.LanguageCSharp:
			hclCSharpPartialConversionFailures++
		case convert.LanguageGo:
			hclGoPartialConversionFailures++
		}

		if exampleTitle == "" {
			g.warn(fmt.Sprintf("unable to convert HCL example for Pulumi entity '%s' in the following language(s): "+
				"%s. Examples for these languages will be dropped from any generated docs or SDKs.",
				path, strings.Join(failedLangsStrings, ", ")))
		} else {
			g.warn(fmt.Sprintf("unable to convert HCL example '%s' for Pulumi entity '%s' in the following language(s): "+
				"%s. Examples for these languages will be dropped from any generated docs or SDKs.",
				exampleTitle, path, strings.Join(failedLangsStrings, ", ")))
		}

		// At least one language out of the given set has been generated, which is considered a success
		//nolint:ineffassign
		err = nil
	}

	return result.String(), nil
}

// genLanguageToSlice maps a Language on a Generator to a slice of strings suitable to pass to HCL conversion.
func genLanguageToSlice(input Language) []string {
	switch input {
	case NodeJS:
		return []string{convert.LanguageTypescript}
	case Python:
		return []string{convert.LanguagePython}
	case CSharp:
		return []string{convert.LanguageCSharp}
	case Golang:
		return []string{convert.LanguageGo}
	case PCL:
		return []string{convert.LanguagePulumi}
	case Schema:
		return []string{
			convert.LanguageTypescript,
			convert.LanguagePython,
			convert.LanguageCSharp,
			convert.LanguageGo,
			convert.LanguageYaml,
			convert.LanguageJava,
		}
	default:
		msg := fmt.Sprintf("Unable to convert generator language '%v' to a list of languages the Bridge understands.", input)
		panic(msg)
	}
}

func cleanupDoc(name string, g *Generator, doc entityDocs, footerLinks map[string]string) (entityDocs, bool) {
	elidedDoc := false
	newargs := make(map[string]*argumentDocs, len(doc.Arguments))

	for k, v := range doc.Arguments {
		g.debug("Cleaning up text for argument [%v] in [%v]", k, name)
		cleanedText, elided := reformatText(g, v.description, footerLinks)
		if elided {
			elidedArguments++
			g.warn("Found <elided> in docs for argument [%v] in [%v]. The argument's description will be dropped in "+
				"the Pulumi provider.", k, name)
			elidedDoc = true
		}

		newargs[k] = &argumentDocs{
			description: cleanedText,
			arguments:   make(map[string]string, len(v.arguments)),
			isNested:    v.isNested,
		}

		// Clean nested arguments (if any)
		for kk, vv := range v.arguments {
			g.debug("Cleaning up text for nested argument [%v] in [%v]", kk, name)
			cleanedText, elided := reformatText(g, vv, footerLinks)
			if elided {
				elidedNestedArguments++
				g.warn("Found <elided> in docs for nested argument [%v] in [%v]. The argument's description will be "+
					"dropped in the Pulumi provider.", kk, name)
				elidedDoc = true
			}
			newargs[k].arguments[kk] = cleanedText
		}
	}

	newattrs := make(map[string]string, len(doc.Attributes))
	for k, v := range doc.Attributes {
		g.debug("Cleaning up text for attribute [%v] in [%v]", k, name)
		cleanedText, elided := reformatText(g, v, footerLinks)
		if elided {
			elidedAttributes++
			g.warn("Found <elided> in docs for attribute [%v] in [%v]. The attribute's description will be dropped "+
				"in the Pulumi provider.", k, name)
			elidedDoc = true
		}
		newattrs[k] = cleanedText
	}

	g.debug("Cleaning up description text for [%v]", name)
	cleanupText, elided := reformatText(g, doc.Description, footerLinks)
	if elided {
		g.debug("Found <elided> in the description. Attempting to extract examples from the description and " +
			"reformat examples only.")

		// Attempt to keep the Example Usage if the elided text was only in the description:
		// TODO: *Also* attempt to keep the description if the elided text is only in the Example Usage
		examples := extractExamples(doc.Description)
		if examples == "" {
			g.debug("Unable to find any examples in the description text. The entire description will be discarded.")

			elidedDescriptions++
			g.warn("Found <elided> in description for [%v]. The description and any examples will be dropped in the "+
				"Pulumi provider.", name)
			elidedDoc = true
		} else {
			g.debug("Found examples in the description text. Attempting to reformat the examples.")

			cleanedupExamples, examplesElided := reformatText(g, examples, footerLinks)
			if examplesElided {
				elidedDescriptions++
				g.warn("Found <elided> in description for [%v]. The description and any examples will be dropped in "+
					"the Pulumi provider.", name)
				elidedDoc = true
			} else {
				elidedDescriptionsOnly++
				g.warn("Found <elided> in description for [%v], but was able to preserve the examples. The description "+
					"proper will be dropped in the Pulumi provider.", name)
				cleanupText = cleanedupExamples
			}
		}
	}

	return entityDocs{
		Description: cleanupText,
		Arguments:   newargs,
		Attributes:  newattrs,
		Import:      doc.Import,
	}, elidedDoc
}

var (
	// Match a [markdown](link)
	markdownLink = regexp.MustCompile(`\[([^\]]*)\]\(([^\)]*)\)`)

	// Match a ```fenced code block```.
	codeBlocks = regexp.MustCompile(`(?ms)\x60\x60\x60[^\n]*?$.*?\x60\x60\x60\s*$`)

	codeLikeSingleWord = regexp.MustCompile(`` + // trick gofmt into aligning the rest of the string
		// Match code_like_words inside code and plain text
		`((?P<open>[\s"\x60\[])(?P<name>([0-9a-z]+_)+[0-9a-z]+)(?P<close>[\s"\x60\]]))` +

		// Match `code` words
		`|(\x60(?P<name>[0-9a-z]+)\x60)`)
)

// Regex for catching reference links, e.g. [1]: /docs/providers/aws/d/networ_interface.html
var markdownPageReferenceLink = regexp.MustCompile(`\[[1-9]+\]: /docs/providers(?:/[a-z1-9_]+)+\.[a-z]+`)

const elidedDocComment = "<elided>"

func fixupPropertyReferences(language Language, pkg tokens.Package, info tfbridge.ProviderInfo, text string) string {
	formatModulePrefix := func(mod tokens.ModuleName) string {
		modname := mod.String()
		if mod == indexMod {
			modname = ""
		}
		if modname != "" {
			modname += "."
		}
		return modname
	}

	return codeLikeSingleWord.ReplaceAllStringFunc(text, func(match string) string {
		parts := codeLikeSingleWord.FindStringSubmatch(match)

		var open, name, close string
		if parts[2] != "" {
			open, name, close = parts[2], parts[3], parts[5]
		} else {
			open, name, close = "`", parts[7], "`"
		}

		if resInfo, hasResourceInfo := info.Resources[name]; hasResourceInfo {
			// This is a resource name
			resname, mod := resourceName(info.GetResourcePrefix(), name, resInfo, false)
			modname := formatModulePrefix(parentModuleName(mod))
			switch language {
			case Golang, Python:
				// Use `ec2.Instance` format
				return open + modname + resname.String() + close
			default:
				// Use `aws.ec2.Instance` format
				return open + pkg.String() + "." + modname + resname.String() + close
			}
		} else if dataInfo, hasDatasourceInfo := info.DataSources[name]; hasDatasourceInfo {
			// This is a data source name
			getname, mod := dataSourceName(info.GetResourcePrefix(), name, dataInfo)
			modname := formatModulePrefix(parentModuleName(mod))
			switch language {
			case Golang:
				// Use `ec2.getAmi` format
				return open + modname + getname.String() + close
			case Python:
				// Use `ec2.get_ami` format
				return open + python.PyName(modname+getname.String()) + close
			default:
				// Use `aws.ec2.getAmi` format
				return open + pkg.String() + "." + modname + getname.String() + close
			}
		}
		// Else just treat as a property name
		switch language {
		case NodeJS, Golang:
			// Use `camelCase` format
			pname := propertyName(name, nil, nil)
			return open + pname + close
		default:
			return match
		}
	})
}

// extractExamples attempts to separate the description proper from the "Example Usage" section of an entity's
// (resource or data source) description. If unable to gracefully separate these 2 parts, an empty string is returned.
func extractExamples(description string) string {
	separator := "## Example Usage"
	parts := strings.Split(description, separator)

	if len(parts) != 2 {
		return ""
	}

	return strings.Replace(description, parts[0], "", -1)
}

// reformatText processes markdown strings from TF docs and cleans them for inclusion in Pulumi docs
func reformatText(g *Generator, text string, footerLinks map[string]string) (string, bool) {

	cleanupText := func(text string) (string, bool) {
		// Remove incorrect documentation that should have been cleaned up in our forks.
		// TODO: fail the build in the face of such text, once we have a processes in place.
		if strings.Contains(text, "Terraform") || strings.Contains(text, "terraform") {
			return "", true
		}

		// Replace occurrences of "->" or "~>" with just ">", to get a proper MarkDown note.
		text = strings.Replace(text, "-> ", "> ", -1)
		text = strings.Replace(text, "~> ", "> ", -1)

		// Trim Prefixes we see when the description is spread across multiple lines.
		text = strings.TrimPrefix(text, "\n(Required)\n")
		text = strings.TrimPrefix(text, "\n(Optional)\n")

		// Find markdown Terraform docs site reference links.
		text = markdownPageReferenceLink.ReplaceAllStringFunc(text, func(referenceLink string) string {
			parts := strings.Split(referenceLink, " ")
			// Add Terraform domain to avoid broken links.
			return fmt.Sprintf("%s https://www.terraform.io%s", parts[0], parts[1])
		})

		// Find links from the footer links.
		text = replaceFooterLinks(text, footerLinks)

		// Find URLs and re-write local links
		text = markdownLink.ReplaceAllStringFunc(text, func(link string) string {
			parts := markdownLink.FindStringSubmatch(link)
			url := parts[2]
			if strings.HasPrefix(url, "http") {
				// Absolute URL, return as-is
				return link
			} else if strings.HasPrefix(url, "/") {
				// Relative URL to the root of the Terraform docs site, rewrite to absolute
				return fmt.Sprintf("[%s](https://www.terraform.io%s)", parts[1], url)
			} else if strings.HasPrefix(url, "#") {
				// Anchor in current page,  can't be resolved currently so remove the link.
				// Note: This throws away potentially valuable information in the name of not having broken links.
				return parts[1]
			}
			// Relative URL to the current page, can't be resolved currently so remove the link.
			// Note: This throws away potentially valuable information in the name of not having broken links.
			return parts[1]
		})

		// Fixup resource and property name references
		text = fixupPropertyReferences(g.language, g.pkg, g.info, text)

		return text, false
	}

	// Detect all code blocks in the text so we can avoid processing them.
	codeBlocks := codeBlocks.FindAllStringIndex(text, -1)

	var parts []string
	start, end := 0, 0
	for _, codeBlock := range codeBlocks {
		end = codeBlock[0]

		clean, elided := cleanupText(text[start:end])
		if elided {
			return "", true
		}
		parts = append(parts, clean)

		start = codeBlock[1]
		parts = append(parts, text[end:start])
	}
	if start != len(text) {
		clean, elided := cleanupText(text[start:])
		if elided {
			return "", true
		}
		parts = append(parts, clean)
	}

	return strings.TrimSpace(strings.Join(parts, "")), false
}

// For example:
// [What is AWS Lambda?][1]
var linkWithFooterRefRegexp = regexp.MustCompile(`(\[[a-zA-Z?.! ]+\])(\[[0-9]+\])`)

// replaceFooterLinks replaces all links with a reference to a footer link.
func replaceFooterLinks(text string, footerLinks map[string]string) string {
	if len(footerLinks) == 0 {
		return text
	}
	return linkWithFooterRefRegexp.ReplaceAllStringFunc(text, func(link string) string {
		parts := linkWithFooterRefRegexp.FindStringSubmatch(link)
		linkText := parts[1]
		linkRef := parts[2]

		// If we have a footer link for the reference, we need to replace it.
		if footerLink, ok := footerLinks[linkRef]; ok {
			return linkText + "(" + footerLink + ")"
		}
		return link
	})
}
