// Copyright 2016-2018, Pulumi Corporation.
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
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfbridge"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/tf2pulumi/convert"
	"github.com/spf13/afero"
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

	// URL is the source documentation URL page
	URL string
}

// DocKind indicates what kind of entity's documentation is being requested.
type DocKind string

const (
	// ResourceDocs indicates documentation pertaining to resource entities.
	ResourceDocs DocKind = "r"
	// DataSourceDocs indicates documentation pertaining to data source entities.
	DataSourceDocs DocKind = "d"
)

func getRepoPath(org string, provider string) (string, error) {
	gomod, err := LoadGoMod()
	if err != nil {
		return "", err
	}

	calculatedImportPath := fmt.Sprintf("github.com/%s/terraform-provider-%s", org, provider)
	importPath, version, err := FindEffectiveModuleForImportPath(gomod, calculatedImportPath)
	if err != nil {
		return "", err
	}

	repo := GetModuleRoot(importPath, version)
	if fileInfo, err := os.Stat(repo); err != nil || !fileInfo.IsDir() {
		return "", err
	}

	return repo, nil
}

func getMarkdownDetails(g *generator, org string, provider string, resourcePrefix string, kind DocKind,
	rawname string, info tfbridge.ResourceOrDataSourceInfo) ([]byte, string, bool) {

	repoPath, err := getRepoPath(org, provider)
	if err != nil {
		return nil, "", false
	}

	possibleMarkdownNames := []string{
		// Most frequently, docs leave off the provider prefix
		withoutPackageName(resourcePrefix, rawname) + ".html.markdown",
		withoutPackageName(resourcePrefix, rawname) + ".markdown",
		withoutPackageName(resourcePrefix, rawname) + ".html.md",
		// But for some providers, the prefix is included in the name of the doc file
		rawname + ".html.markdown",
		rawname + ".markdown",
		rawname + ".html.md",
	}

	var docinfo *tfbridge.DocInfo
	if info != nil {
		docinfo = info.GetDocs()
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

// getDocsForProvider extracts documentation details for the given package from
// TF website documentation markdown content
func getDocsForProvider(g *generator, org string, provider string, resourcePrefix string, kind DocKind,
	rawname string, info tfbridge.ResourceOrDataSourceInfo) (entityDocs, error) {

	markdownBytes, markdownFileName, found := getMarkdownDetails(g, org, provider, resourcePrefix, kind, rawname, info)
	if !found {
		cmdutil.Diag().Warningf(
			diag.Message("", "Could not find docs for resource %v; consider overriding doc source location"), rawname)
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
		// Merge Attributes from source into target
		if err := mergeDocs(g, info, org, provider, resourcePrefix, kind, doc,
			docinfo.IncludeAttributesFrom, true, true); err != nil {
			return doc, err
		}

		// Merge Arguments from source into Attributes of target
		if err := mergeDocs(g, info, org, provider, resourcePrefix, kind, doc,
			docinfo.IncludeAttributesFromArguments, true, false); err != nil {
			return doc, err
		}

		// Merge Arguments from source into target
		if err := mergeDocs(g, info, org, provider, provider, kind, doc,
			docinfo.IncludeArgumentsFrom, false, false); err != nil {
			return doc, err
		}
	}

	return doc, nil
}

// readMarkdown searches all possible locations for the markdown content
func readMarkdown(repo string, kind DocKind, possibleLocations []string) ([]byte, string, bool) {
	for _, name := range possibleLocations {
		location := path.Join(repo, "website", "docs", string(kind), name)
		markdownBytes, err := ioutil.ReadFile(location)
		if err == nil {
			return markdownBytes, name, true
		}
	}
	return nil, "", false
}

// mergeDocs adds the docs specified by extractDoc from sourceFrom into the targetDocs
func mergeDocs(g *generator, info tfbridge.ResourceOrDataSourceInfo, org string, provider string,
	resourcePrefix string,
	kind DocKind, docs entityDocs, sourceFrom string,
	useTargetAttributes bool, useSourceAttributes bool) error {

	if sourceFrom != "" {
		sourceDocs, err := getDocsForProvider(g, org, provider, resourcePrefix, kind, sourceFrom, nil)
		if err != nil {
			return err
		}

		if useTargetAttributes && useSourceAttributes {
			for k, v := range sourceDocs.Attributes {
				docs.Attributes[k] = v
			}
		} else if useTargetAttributes && !useSourceAttributes {
			for k, v := range sourceDocs.Arguments {
				docs.Attributes[k] = v.description
				for kk, vv := range v.arguments {
					docs.Attributes[kk] = vv
				}
			}
		} else if !useTargetAttributes && !useSourceAttributes {
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
	}
	return nil
}

// nolint:lll
var (
	// For example:
	// [1]: https://docs.aws.amazon.com/lambda/latest/dg/welcome.html
	linkFooterRegexp = regexp.MustCompile(`(?m)^(\[\d+\]):\s(.*)`)

	argumentBulletRegexp = regexp.MustCompile(
		"^\\s*\\*\\s+`([a-zA-z0-9_]*)`\\s*(\\([a-zA-Z]*\\)\\s*)?[–-]?\\s+(\\([^\\)]*\\)\\s*)?(.*)")

	nestedObjectRegexps = []*regexp.Regexp{
		// For example:
		// s3_bucket.html.markdown: "The `website` object supports the following:"
		// ami.html.markdown: "When `virtualization_type` is "hvm" the following additional arguments apply:"
		regexp.MustCompile("`([a-z_]+)`.*following"),

		// For example:
		// athena_workgroup.html.markdown: "#### result_configuration Argument Reference"
		regexp.MustCompile("(?i)## ([a-z_]+).* argument reference"),
	}

	attributeBulletRegexp = regexp.MustCompile("\\*\\s+`([a-zA-z0-9_]*)`\\s+[–-]?\\s+(.*)")

	docsBaseURL    = "https://github.com/%s/terraform-provider-%s/blob/master/website/docs"
	docsDetailsURL = docsBaseURL + "/%s/%s"

	standardDocReadme = `> This provider is a derived work of the [Terraform Provider](https://github.com/%[3]s/terraform-provider-%[2]s)
> distributed under [%[4]s](%[5]s). If you encounter a bug or missing feature,
> first check the [` + "`pulumi/pulumi-%[1]s`" + ` repo](https://github.com/pulumi/pulumi-%[1]s/issues); however, if that doesn't turn up anything,
> please consult the source [` + "`%[3]s/terraform-provider-%[2]s`" + ` repo](https://github.com/%[3]s/terraform-provider-%[2]s/issues).`
	attributionFormatString = "This Pulumi package is based on the [`%[1]s` Terraform Provider](https://github.com/%[2]s/terraform-provider-%[1]s)."
)

// groupLines groups a collection of strings, a, by a given separator, sep.
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

// getDocsBaseURL gets the base URL for a given provider's documentation source.
func getDocsBaseURL(org, p string) string {
	return fmt.Sprintf(docsBaseURL, org, p)
}

// getDocsDetailsURL gets the detailed resource or data source documentation source.
func getDocsDetailsURL(org, p, kind, markdownFileName string) string {
	return fmt.Sprintf(docsDetailsURL, org, p, kind, markdownFileName)
}

// getDocsIndexURL gets the given provider's documentation index page's source URL.
func getDocsIndexURL(org, p string) string {
	return getDocsBaseURL(org, p) + "/index.html.markdown"
}

// parseTFMarkdown takes a TF website markdown doc and extracts a structured representation for use in
// generating doc comments
func parseTFMarkdown(g *generator, info tfbridge.ResourceOrDataSourceInfo, kind DocKind,
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
	g                *generator
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
)

func (p *tfMarkdownParser) parse() (entityDocs, error) {
	p.ret = entityDocs{
		Arguments:  make(map[string]*argumentDocs),
		Attributes: make(map[string]string),
		URL:        getDocsDetailsURL(p.g.info.GetGitHubOrg(), p.resourcePrefix, string(p.kind), p.markdownFileName),
	}

	// Replace any Windows-style newlines.
	markdown := strings.Replace(p.markdown, "\r\n", "\n", -1)

	// Split the sections by H2 topics in the Markdown file.
	sections := splitGroupLines(markdown, "## ")

	// Reparent examples that are peers of the "Example Usage" section (if any) and fixup some example titles.
	sections = p.reformatExamples(sections)

	for _, section := range sections {
		if err := p.parseSection(section); err != nil {
			return entityDocs{}, err
		}
	}

	// Get links.
	footerLinks := getFooterLinks(markdown)

	doc, elided := cleanupDoc(p.g, p.info, p.ret, footerLinks)
	if elided {
		cmdutil.Diag().Warningf(diag.Message("",
			"Resource %v contains an <elided> doc reference that needs updated"), p.rawname)
	}

	return doc, nil
}

// fixExampleTitles transforms H4 sections that contain code snippets into H3 sections.
func (p *tfMarkdownParser) fixExampleTitles(lines []string) {
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
func (p *tfMarkdownParser) reformatExamples(sections [][]string) [][]string {
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
		} else {
			// This is a qualified example usage section. Retitle it using an H3 and its qualifier, and append it to
			// the output.
			exampleUsageSection = append(exampleUsageSection, "", "### "+strings.Title(matches[2]))
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
	p.fixExampleTitles(exampleUsageSection)
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

func (p *tfMarkdownParser) parseSection(section []string) error {
	// Extract the header name, since this will drive how we process the content.
	if len(section) == 0 {
		cmdutil.Diag().Warningf(diag.Message("",
			"Unparseable H2 doc section for %v; consider overriding doc source location"), p.rawname)
		return nil
	}

	// Skip certain headers that we don't support.
	header := section[0]
	if strings.Index(header, "## ") == 0 {
		header = header[3:]
	}

	sectionKind := sectionOther
	switch header {
	case "Import", "Imports", "Timeout", "Timeouts", "User Project Override", "User Project Overrides":
		ignoredDocSections++
		ignoredDocHeaders[header]++
		return nil
	case "Example Usage":
		sectionKind = sectionExampleUsage
	case "Arguments Reference", "Argument Reference", "Nested Blocks", "Nested blocks":
		sectionKind = sectionArgsReference
	case "Attributes Reference", "Attribute Reference":
		sectionKind = sectionAttributesReference
	case "---":
		sectionKind = sectionFrontMatter
	}

	// Now split the sections by H3 topics. This is done because we'll ignore sub-sections with code
	// snippets that are unparseable (we don't want to ignore entire H2 sections).
	var wroteHeader bool
	for _, subsection := range groupLines(section[1:], "### ") {
		if len(subsection) == 0 {
			cmdutil.Diag().Warningf(diag.Message("",
				"Unparseable H3 doc section for %v; consider overriding doc source location"), p.rawname)
			continue
		}

		// Remove the "Open in Cloud Shell" button if any and check for the presence of code snippets.
		subsection, hasExamples, isEmpty := p.reformatSubsection(subsection)
		if isEmpty {
			// Skip empty subsections (they just add unnecessary padding and headers).
			continue
		}
		if hasExamples && sectionKind != sectionExampleUsage {
			cmdutil.Diag().Warningf(diag.Message("",
				"Unexpected code snippets in section %v for resource %v"), header, p.rawname)
		}

		// Now process the content based on the H2 topic. These are mostly standard across TF's docs.
		switch sectionKind {
		case sectionArgsReference:
			p.parseArgReferenceSection(subsection)
		case sectionAttributesReference:
			p.parseAttributesReferenceSection(subsection)
		case sectionFrontMatter:
			p.parseFrontMatter(subsection)
		default:
			// Determine if this is a nested argument section.
			_, isArgument := p.ret.Arguments[header]
			if isArgument || strings.HasSuffix(header, "Configuration Block") {
				p.parseArgReferenceSection(subsection)
				continue
			}

			// For all other sections, append them to the description section.
			if !wroteHeader {
				p.ret.Description += fmt.Sprintf("## %s\n", header)
				wroteHeader = true
				if !isBlank(subsection[0]) {
					p.ret.Description += "\n"
				}
			}
			p.ret.Description += strings.Join(subsection, "\n") + "\n"
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

func (p *tfMarkdownParser) parseArgReferenceSection(subsection []string) {
	var lastMatch, nested string
	for _, line := range subsection {
		matches := argumentBulletRegexp.FindStringSubmatch(line)
		if len(matches) >= 4 {
			// found a property bullet, extract the name and description
			if nested != "" {
				// We found this line within a nested field. We should record it as such.
				if p.ret.Arguments[nested] == nil {
					p.ret.Arguments[nested] = &argumentDocs{
						arguments: make(map[string]string),
					}
				} else if p.ret.Arguments[nested].arguments == nil {
					p.ret.Arguments[nested].arguments = make(map[string]string)
				}
				p.ret.Arguments[nested].arguments[matches[1]] = matches[4]

				// Also record this as a top-level argument just in case, since sometimes the recorded nested
				// argument doesn't match the resource's argument.
				// For example, see `cors_rule` in s3_bucket.html.markdown.
				if p.ret.Arguments[matches[1]] == nil {
					p.ret.Arguments[matches[1]] = &argumentDocs{
						description: matches[4],
						isNested:    true, // Mark that this argument comes from a nested field.
					}
				}
			} else {
				if !strings.HasSuffix(line, "supports the following:") {
					p.ret.Arguments[matches[1]] = &argumentDocs{description: matches[4]}
				}
			}
			lastMatch = matches[1]
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
			for _, match := range nestedObjectRegexps {
				matches := match.FindStringSubmatch(line)
				if len(matches) >= 2 {
					nested = strings.ToLower(matches[1])
					break
				}
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
		cmdutil.Diag().Warningf(
			diag.Message("", "Expected to pair --- begin/end for resource %v's Markdown header"), p.rawname)
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
		cmdutil.Diag().Warningf(diag.Message("", "Expected an H1 in markdown for resource %v"), p.rawname)
	}
}

var (
	ignoredDocSections int
	ignoredDocHeaders  = make(map[string]int)
	hclBlocksSucceeded int
	hclBlocksFailed    int
	hclFailures        = make(map[string]bool)
)

// isBlank returns true if the line is all whitespace.
func isBlank(line string) bool {
	return strings.TrimSpace(line) == ""
}

// printDocStats outputs warnings and, if flags are set, stdout diagnostics pertaining to documentation conversion.
func printDocStats(printIgnoreDetails, printHCLFailureDetails bool) {
	// These summaries are printed on each run, to help us keep an eye on success/failure rates.
	if ignoredDocSections > 0 {
		cmdutil.Diag().Warningf(
			diag.Message("", "%d documentation sections ignored"), ignoredDocSections)
	}
	if hclBlocksFailed > 0 {
		cmdutil.Diag().Warningf(
			diag.Message("", "%d/%d documentation code blocks failed to convert"),
			hclBlocksFailed, hclBlocksFailed+hclBlocksSucceeded)
	}

	// These more detailed outputs are suppressed by default, but can be enabled to track down failures.
	if printIgnoreDetails {
		fmt.Printf("---IGNORES---\n")
		var ignores []string
		for ignore := range ignoredDocHeaders {
			ignores = append(ignores, ignore)
		}
		sort.Strings(ignores)
		for _, ignore := range ignores {
			fmt.Printf("[%d] %s\n", ignoredDocHeaders[ignore], ignore)
		}
	}
	if printHCLFailureDetails {
		fmt.Printf("---HCL FAILURES---\n")
		var failures []string
		for failure := range hclFailures {
			failures = append(failures, failure)
		}
		sort.Strings(failures)
		for i, failure := range failures {
			fmt.Printf("%d: %s\n", i, failure)
		}
	}
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

// parseExamples converts any code snippets in a subsection to Pulumi-compatible code. This conversion is done on a
// per-subsection basis; subsections with failing examples will be elided upon the caller's request.
func (g *generator) convertExamples(docs string, stripSubsectionsWithErrors bool) string {
	if docs == "" {
		return ""
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

		header, wroteHeader := section[0], false
		isFrontMatter, isExampleUsage := !strings.HasPrefix(header, "## "), header == "## Example Usage"

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
				if inCodeBlock {
					if strings.Index(line, "```") != 0 {
						continue
					}

					if g.language.shouldConvertExamples() {
						hcl := strings.Join(subsection[codeBlockStart+1:i], "\n")

						// We've got some code -- assume it's HCL and try to convert it.
						codeBlock, stderr, err := g.convertHCL(hcl)
						if err != nil {
							skippedExamples = true
							hclFailures[stderr] = true
							hclBlocksFailed++
						} else {
							fprintf(subsectionOutput, "\n%s", codeBlock)
							hclBlocksSucceeded++
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

// convertHCL converts an in-memory, simple HCL program to Pulumi, and returns it as a string. In the event
// of failure, the error returned will be non-nil, and the second string contains the stderr stream of details.
func (g *generator) convertHCL(hcl string) (string, string, error) {
	// Fixup the HCL as necessary.
	if fixed, ok := fixHcl(hcl); ok {
		hcl = fixed
	}

	input := afero.NewMemMapFs()
	f, err := input.Create("/main.tf")
	contract.AssertNoError(err)
	_, err = f.Write([]byte(hcl))
	contract.AssertNoError(err)
	contract.IgnoreClose(f)

	var result strings.Builder
	var stderr bytes.Buffer
	convertHCL := func(languageName string) (err error) {
		defer func() {
			v := recover()
			if v != nil {
				err = fmt.Errorf("panic converting HCL to %v: %v", languageName, v)
			}
		}()

		files, diags, err := convert.Convert(convert.Options{
			Root:                  input,
			TargetLanguage:        languageName,
			AllowMissingVariables: true,
			FilterResourceNames:   true,
			PackageCache:          g.packageCache,
			PluginHost:            g.pluginHost,
			ProviderInfoSource:    g.infoSource,
		})
		if err != nil {
			return fmt.Errorf("failied to convert HCL to %v: %w", languageName, err)
		}
		if diags.All.HasErrors() {
			if stderr.Len() != 0 {
				_, err := fmt.Fprintf(&stderr, "\n")
				contract.IgnoreError(err)
			}
			_, err := fmt.Fprintf(&stderr, "# %s\n", languageName)
			contract.IgnoreError(err)

			_, err = fmt.Fprintf(&stderr, "%s\n\n", hcl)
			contract.IgnoreError(err)

			err = diags.NewDiagnosticWriter(&stderr, 0, false).WriteDiagnostics(diags.All)
			contract.IgnoreError(err)

			// Note that we intentionally avoid returning an error here. The caller will check for an empty code block
			// before returning and translate that into an error.
			return nil
		}

		contract.Assert(len(files) == 1)

		// Add a fenced code-block with the resulting code snippet.
		for _, output := range files {
			if result.Len() > 0 {
				result.WriteByte('\n')
			}
			_, err := fmt.Fprintf(&result, "```%s\n%s\n```", languageName, strings.TrimSpace(string(output)))
			contract.IgnoreError(err)
		}

		return nil
	}

	switch g.language {
	case nodeJS:
		err = convertHCL("typescript")
	case python:
		err = convertHCL("python")
	case csharp:
		err = convertHCL("csharp")
	case golang:
		err = convertHCL("go")
	case pulumiSchema:
		langs := []string{"typescript", "python", "csharp", "go"}
		for _, lang := range langs {
			if langErr := convertHCL(lang); langErr != nil {
				return "", stderr.String(), langErr
			}
		}
	}
	if err != nil {
		return "", stderr.String(), err
	}
	if result.Len() == 0 {
		return "", stderr.String(), fmt.Errorf("failed to convert HCL to %v", g.language)
	}
	return result.String(), stderr.String(), nil
}

func cleanupDoc(g *generator, info tfbridge.ResourceOrDataSourceInfo, doc entityDocs,
	footerLinks map[string]string) (entityDocs, bool) {
	elidedDoc := false
	newargs := make(map[string]*argumentDocs, len(doc.Arguments))
	for k, v := range doc.Arguments {
		cleanedText, elided := cleanupText(g, info, v.description, footerLinks)
		if elided {
			elidedDoc = true
		}

		newargs[k] = &argumentDocs{
			description: cleanedText,
			arguments:   make(map[string]string, len(v.arguments)),
		}

		// Clean nested arguments (if any)
		for kk, vv := range v.arguments {
			cleanedText, elided := cleanupText(g, info, vv, footerLinks)
			if elided {
				elidedDoc = true
			}
			newargs[k].arguments[kk] = cleanedText
		}
	}
	newattrs := make(map[string]string, len(doc.Attributes))
	for k, v := range doc.Attributes {
		cleanupText, elided := cleanupText(g, info, v, footerLinks)
		if elided {
			elidedDoc = true
		}
		newattrs[k] = cleanupText
	}
	cleanupText, elided := cleanupText(g, info, doc.Description, footerLinks)
	if elided {
		elidedDoc = true
	}
	return entityDocs{
		Description: cleanupText,
		Arguments:   newargs,
		Attributes:  newattrs,
		URL:         doc.URL,
	}, elidedDoc

}

//nolint:lll
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

func fixupPropertyReferences(language language, pkg string, info tfbridge.ProviderInfo, text string) string {
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
			modname := extractModuleName(mod)
			if modname != "" {
				modname += "."
			}

			switch language {
			case golang, python:
				// Use `ec2.Instance` format
				return open + modname + resname + close
			default:
				// Use `aws.ec2.Instance` format
				return open + pkg + "." + modname + resname + close
			}
		} else if dataInfo, hasDatasourceInfo := info.DataSources[name]; hasDatasourceInfo {
			// This is a data source name
			getname, mod := dataSourceName(info.GetResourcePrefix(), name, dataInfo)
			modname := extractModuleName(mod)
			if modname != "" {
				modname += "."
			}

			switch language {
			case golang, python:
				// Use `ec2.getAmi` format
				return open + modname + getname + close
			default:
				// Use `aws.ec2.getAmi` format
				return open + pkg + "." + modname + getname + close
			}
		}
		// Else just treat as a property name
		switch language {
		case nodeJS, golang:
			// Use `camelCase` format
			pname := propertyName(name, nil, nil)
			return open + pname + close
		default:
			return match
		}
	})
}

// cleanupText processes markdown strings from TF docs and cleans them for inclusion in Pulumi docs
func cleanupText(g *generator, info tfbridge.ResourceOrDataSourceInfo, text string,
	footerLinks map[string]string) (string, bool) {

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
		text = strings.TrimPrefix(text, "-\n(Required)\n")
		text = strings.TrimPrefix(text, "-\n(Optional)\n")

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
