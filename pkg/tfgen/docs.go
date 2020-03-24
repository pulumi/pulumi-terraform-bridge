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
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"

	"github.com/pulumi/pulumi-terraform-bridge/pkg/tfbridge"
)

// Either doc or arguments
type argument struct {
	// The description for this argument
	description string
	// The arguments for this argument
	arguments map[string]string
	// Whether this argument was derived from a nested object. Used to determine
	// whether to append descriptions that have continued to the following line
	isNested bool
}

// parsedDoc represents the data parsed from TF markdown documentation
type parsedDoc struct {
	// Description is the description of the resource
	Description string
	// Arguments includes the names and descriptions for each argument of the resource
	Arguments map[string]*argument
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
	rawname string, info tfbridge.ResourceOrDataSourceInfo) (parsedDoc, error) {

	markdownBytes, markdownFileName, found := getMarkdownDetails(g, org, provider, resourcePrefix, kind, rawname, info)
	if !found {
		cmdutil.Diag().Warningf(
			diag.Message("", "Could not find docs for resource %v; consider overriding doc source location"), rawname)
		return parsedDoc{}, nil
	}

	doc, err := parseTFMarkdown(g, info, kind, string(markdownBytes), markdownFileName, resourcePrefix, rawname)
	if err != nil {
		return parsedDoc{}, nil
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
	resourcePrefix string, kind DocKind, docs parsedDoc, sourceFrom string, useTargetAttributes bool, useSourceAttributes bool) error {

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
				docs.Arguments[k] = &argument{
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
	argumentBulletRegexp = regexp.MustCompile(
		"\\*\\s+`([a-zA-z0-9_]*)`\\s+(\\([a-zA-Z]*\\)\\s*)?[–-]?\\s+(\\([^\\)]*\\)\\s*)?(.*)")

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
	markdown, markdownFileName, resourcePrefix, rawname string) (parsedDoc, error) {

	ret := parsedDoc{
		Arguments:  make(map[string]*argument),
		Attributes: make(map[string]string),
		URL:        getDocsDetailsURL(g.info.GetGitHubOrg(), resourcePrefix, string(kind), markdownFileName),
	}

	// Replace any Windows-style newlines.
	markdown = strings.Replace(markdown, "\r\n", "\n", -1)

	// Split the sections by H2 topics in the MarkDown file.
	for _, section := range splitGroupLines(markdown, "## ") {
		// Extract the header name, since this will drive how we process the content.
		if len(section) == 0 {
			cmdutil.Diag().Warningf(diag.Message("",
				"Unparseable H2 doc section for %v; consider overriding doc source location"), rawname)
			continue
		}

		// Skip certain headers that we don't support.
		header := section[0]
		if strings.Index(header, "## ") == 0 {
			header = header[3:]
		}
		if header == "Import" || header == "Imports" || header == "Timeout" || header == "Timeouts" {
			ignoredDocSections++
			ignoredDocHeaders[header]++
			continue
		}

		// Now split the sections by H3 topics. This is done because we'll ignore sub-sections with code
		// snippets that are unparseable (we don't want to ignore entire H2 sections).
		var wroteHeader bool
		for _, subsection := range groupLines(section[1:], "### ") {
			if len(subsection) == 0 {
				cmdutil.Diag().Warningf(diag.Message("",
					"Unparseable H3 doc section for %v; consider overriding doc source location"), rawname)
				continue
			}

			// Skip empty sections (they just add unnecessary padding and headers).
			var allEmpty bool
			for i, sub := range subsection {
				if !isBlank(sub) {
					break
				}
				if i == len(subsection)-1 {
					allEmpty = true
					break
				}
			}
			if allEmpty {
				continue
			}

			// Detect important section kinds.
			var headerIsArgsReference bool
			var headerIsAttributesReference bool
			var headerIsFrontMatter bool
			switch header {
			case "Arguments Reference", "Argument Reference", "Nested Blocks", "Nested blocks":
				headerIsArgsReference = true
			case "Attributes Reference", "Attribute Reference":
				headerIsAttributesReference = true
			case "---":
				headerIsFrontMatter = true
			}

			// Convert any code snippets, if there are any. If this yields a fatal error, we
			// bail out, but most errors are ignorable and just lead to us skipping one section.
			var skippableExamples bool
			var err error
			subsection, skippableExamples, err = parseExamples(g.language, subsection)
			if err != nil {
				return parsedDoc{}, err
			} else if skippableExamples && !headerIsArgsReference &&
				!headerIsAttributesReference && !headerIsFrontMatter {
				// Skip sections with failed examples, so long as they aren't "essential" blocks.
				continue
			}

			// Now process the content based on the H2 topic. These are mostly standard across TF's docs.
			switch {
			case headerIsArgsReference:
				var lastMatch string
				nested := ""
				for _, line := range subsection {
					matches := argumentBulletRegexp.FindStringSubmatch(line)
					if len(matches) >= 4 {
						// found a property bullet, extract the name and description
						if nested != "" {
							// We found this line within a nested field. We should record it as such.
							if ret.Arguments[nested] == nil {
								ret.Arguments[nested] = &argument{
									arguments: make(map[string]string),
								}
							} else if ret.Arguments[nested].arguments == nil {
								ret.Arguments[nested].arguments = make(map[string]string)
							}
							ret.Arguments[nested].arguments[matches[1]] = matches[4]

							// Also record this as a top-level argument just in case, since sometimes the recorded nested
							// argument doesn't match the resource's argument.
							// For example, see `cors_rule` in s3_bucket.html.markdown.
							if ret.Arguments[matches[1]] == nil {
								ret.Arguments[matches[1]] = &argument{
									description: matches[4],
									isNested:    true, // Mark that this argument comes from a nested field.
								}
							}
						} else {
							ret.Arguments[matches[1]] = &argument{description: matches[4]}
						}
						lastMatch = matches[1]
					} else if !isBlank(line) && lastMatch != "" {
						// this is a continuation of the previous bullet
						if nested != "" {
							ret.Arguments[nested].arguments[lastMatch] += "\n" + strings.TrimSpace(line)

							// Also update the top-level argument if we took it from a nested field.
							if ret.Arguments[lastMatch].isNested {
								ret.Arguments[lastMatch].description += "\n" + strings.TrimSpace(line)
							}
						} else {
							ret.Arguments[lastMatch].description += "\n" + strings.TrimSpace(line)
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
			case headerIsAttributesReference:
				var lastMatch string
				for _, line := range subsection {
					matches := attributeBulletRegexp.FindStringSubmatch(line)
					if len(matches) >= 2 {
						// found a property bullet, extract the name and description
						ret.Attributes[matches[1]] = matches[2]
						lastMatch = matches[1]
					} else if !isBlank(line) && lastMatch != "" {
						// this is a continuation of the previous bullet
						ret.Attributes[lastMatch] += "\n" + strings.TrimSpace(line)
					} else {
						// This is an empty line or there were no bullets yet - clear the lastMatch
						lastMatch = ""
					}
				}
			case headerIsFrontMatter:
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
						diag.Message("", "Expected to pair --- begin/end for resource %v's Markdown header"), rawname)
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
						ret.Description += line + "\n"
						lastBlank = false
					} else if isBlank(line) {
						lastBlank = true
					}
				}
				if !foundH1Resource {
					cmdutil.Diag().Warningf(diag.Message("", "Expected an H1 in markdown for resource %v"), rawname)
				}
			default:
				// For all other sections, append them to the description section.
				if !wroteHeader {
					ret.Description += fmt.Sprintf("## %s\n", header)
					wroteHeader = true
					if !isBlank(subsection[0]) {
						ret.Description += "\n"
					}
				}
				ret.Description += strings.Join(subsection, "\n") + "\n"
			}
		}
	}

	doc, elided := cleanupDoc(g, info, ret)
	if elided {
		cmdutil.Diag().Warningf(diag.Message("",
			"Resource %v contains an <elided> doc reference that needs updated"), rawname)
	}

	return doc, nil
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

// trimTrailingBlanks removes any blank lines from the end of an array.
func trimTrailingBlanks(lines []string) []string {
	for len(lines) > 0 && isBlank(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	return lines
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

// parseExamples converts an examples section into code comments, including converting any code snippets.
// If an error converting a code example occurs, the bool (skip) will be true. If a fatal error occurs, the
// error returned will be non-nil.
func parseExamples(language language, lines []string) ([]string, bool, error) {
	// Each `Example ...` section contains one or more examples written in HCL, optionally separated by
	// comments about the examples. We will attempt to convert them using our `tf2pulumi` tool, and append
	// them to the description. If we can't, we'll simply log a warning and keep moving along.
	var result []string
	var skippableExamples bool
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.Index(line, "```") == 0 {
			// If we found a fenced block, parse out the code from it.
			if language == nodeJS || language == pulumiSchema {
				var hcl string
				for i = i + 1; i < len(lines); i++ {
					cline := lines[i]
					if strings.Index(cline, "```") == 0 {
						// We've got some code -- assume it's HCL and try to convert it.
						code, stderr, err := convertHCL(hcl)
						if err != nil {
							// If the conversion failed, there are two cases to consider. First, if tf2pulumi was
							// missing from the path, we want to error eagerly, as that means the user probably
							// forgot to add tf2pulumi to their path (and we don't want to simply silently delete
							// all docs). Second, for all other errors, record them and proceed silently.
							if err == errTF2PulumiMissing {
								return result, false, err
							}
							skippableExamples = true
							hclFailures[stderr] = true
							hclBlocksFailed++
						} else {
							// Add a fenced code-block with the resulting TypeScript code snippet.
							result = append(result, "```typescript")
							codeLines := strings.Split(code, "\n")
							codeLines = trimTrailingBlanks(codeLines)
							result = append(result, codeLines...)
							result = append(result, "```")
							hclBlocksSucceeded++
						}

						// Now set the index and break out of the inner loop, to consume the code string.
						hcl = ""
						break
					} else {
						hcl += cline + "\n"
					}
				}
				if hcl != "" {
					// If the HCL wasn't consumed, we had an unbalanced pair of ```s, this example is skippable.
					hcl = ""
					skippableExamples = true
				}
			} else {
				// TODO: support other languages.
				for i = i + 1; i < len(lines); i++ {
					if strings.Index(lines[i], "```") == 0 {
						break
					}
				}
				skippableExamples = true
			}
		} else if strings.Index(line, "<div") == 0 && strings.Contains(line, "oics-button") {
			// Strip "Open in Cloud Shell" buttons.
			for i = i + 1; i < len(lines); i++ {
				if strings.Index(lines[i], "</div>") == 0 {
					break
				}
			}
		} else {
			// Otherwise, record any text found before, in between, or after the code snippets, as-is.
			result = append(result, line)
		}
	}

	return result, skippableExamples, nil
}

// errTF2PulumiMissing is a singleton error used to convey and identify situations in which
// tf2pulumi isn't on the PATH and/or hasn't been installed.
var errTF2PulumiMissing = errors.New("tf2pulumi is missing, please install it and re-run")

// convertHCL converts an in-memory, simple HCL program to Pulumi, and returns it as a string. In the event
// of failure, the error returned will be non-nil, and the second string contains the stderr stream of details.
func convertHCL(hcl string) (string, string, error) {
	// First, see if tf2pulumi is on the PATH, or not.
	path, err := exec.LookPath("tf2pulumi")
	if err != nil {
		return "", "", errTF2PulumiMissing
	}

	// Now create a temp dir and spill the HCL into it. Terraform's module loader assumes code is in a file.
	dir, err := ioutil.TempDir("", "pt-hcl-")
	if err != nil {
		return "", "", errors.Wrap(err, "creating temp HCL dir")
	}
	defer os.RemoveAll(dir)

	// fixup the HCL as necessary.
	if fixed, ok := fixHcl(hcl); ok {
		hcl = fixed
	}

	file := filepath.Join(dir, "main.tf")
	if err = ioutil.WriteFile(file, []byte(hcl), 0644); err != nil {
		return "", "", errors.Wrap(err, "writing temp HCL file")
	}

	// Now run the tf2pulumi command, streaming the results into a string. This explicitly does not use
	// tf2pulumi in library form because it greatly complicates our modules/vendoring story.
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	args := []string{"--allow-missing-variables", "--filter-auto-names"}

	tf2pulumi := exec.Command(path, args...)
	tf2pulumi.Dir = dir
	tf2pulumi.Stdout = stdout
	tf2pulumi.Stderr = stderr
	if err = tf2pulumi.Run(); err != nil {
		return "", stderr.String(), errors.Wrap(err, "converting HCL to Pulumi code")
	}

	result := stdout.String()
	if strings.Contains(result, "tf2pulumi error") {
		return "", "", errors.New("tf2pulumi error in generated code")
	}
	return result, "", nil
}

func cleanupDoc(g *generator, info tfbridge.ResourceOrDataSourceInfo, doc parsedDoc) (parsedDoc, bool) {
	elidedDoc := false
	newargs := make(map[string]*argument, len(doc.Arguments))
	for k, v := range doc.Arguments {
		cleanedText, elided := cleanupText(g, info, v.description)
		if elided {
			elidedDoc = true
		}

		newargs[k] = &argument{
			description: cleanedText,
			arguments:   make(map[string]string, len(v.arguments)),
		}

		// Clean nested arguments (if any)
		for kk, vv := range v.arguments {
			cleanedText, elided := cleanupText(g, info, vv)
			if elided {
				elidedDoc = true
			}
			newargs[k].arguments[kk] = cleanedText
		}
	}
	newattrs := make(map[string]string, len(doc.Attributes))
	for k, v := range doc.Attributes {
		cleanupText, elided := cleanupText(g, info, v)
		if elided {
			elidedDoc = true
		}
		newattrs[k] = cleanupText
	}
	cleanupText, elided := cleanupText(g, info, doc.Description)
	if elided {
		elidedDoc = true
	}
	return parsedDoc{
		Description: cleanupText,
		Arguments:   newargs,
		Attributes:  newattrs,
		URL:         doc.URL,
	}, elidedDoc

}

var markdownLink = regexp.MustCompile(`\[([^\]]*)\]\(([^\)]*)\)`)
var codeLikeSingleWord = regexp.MustCompile("([\\s`\"\\[])(([0-9a-z]+_)+[0-9a-z]+)([\\s`\"\\]])")

// Regex for catching reference links, e.g. [1]: /docs/providers/aws/d/networ_interface.html
var markdownPageReferenceLink = regexp.MustCompile(`\[[1-9]+\]: /docs/providers(?:/[a-z1-9_]+)+\.[a-z]+`)

const elidedDocComment = "<elided>"

// cleanupText processes markdown strings from TF docs and cleans them for inclusion in Pulumi docs
func cleanupText(g *generator, info tfbridge.ResourceOrDataSourceInfo, text string) (string, bool) {
	// Remove incorrect documentation that should have been cleaned up in our forks.
	// TODO: fail the build in the face of such text, once we have a processes in place.
	if strings.Contains(text, "Terraform") || strings.Contains(text, "terraform") {
		return "", true
	}

	// Replace occurrences of "->" or "~>" with just ">", to get a proper MarkDown note.
	text = strings.Replace(text, "-> ", "> ", -1)
	text = strings.Replace(text, "~> ", "> ", -1)

	// Find markdown Terraform docs site reference links.
	text = markdownPageReferenceLink.ReplaceAllStringFunc(text, func(referenceLink string) string {
		parts := strings.Split(referenceLink, " ")
		// Add Terraform domain to avoid broken links.
		return fmt.Sprintf("%s https://www.terraform.io%s", parts[0], parts[1])
	})

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
	text = codeLikeSingleWord.ReplaceAllStringFunc(text, func(match string) string {
		parts := codeLikeSingleWord.FindStringSubmatch(match)
		name := parts[2]
		if resInfo, hasResourceInfo := g.info.Resources[name]; hasResourceInfo {
			// This is a resource name
			resname, mod := resourceName(g.info.GetResourcePrefix(), name, resInfo, false)
			modname := extractModuleName(mod)
			switch g.language {
			case golang, python:
				// Use `ec2.Instance` format
				return parts[1] + modname + "." + resname + parts[4]
			default:
				// Use `aws.ec2.Instance` format
				return parts[1] + g.pkg + "." + modname + "." + resname + parts[4]
			}
		} else if dataInfo, hasDatasourceInfo := g.info.DataSources[name]; hasDatasourceInfo {
			// This is a data source name
			getname, mod := dataSourceName(g.info.GetResourcePrefix(), name, dataInfo)
			modname := extractModuleName(mod)
			switch g.language {
			case golang, python:
				// Use `ec2.getAmi` format
				return parts[1] + modname + "." + getname + parts[4]
			default:
				// Use `aws.ec2.getAmi` format
				return parts[1] + g.pkg + "." + modname + "." + getname + parts[4]
			}
		}
		// Else just treat as a property name
		switch g.language {
		case nodeJS, golang:
			// Use `camelCase` format
			pname := propertyName(name, nil, nil)
			return parts[1] + pname + parts[4]
		default:
			return match
		}
	})

	// Finally, trim any trailing blank lines and return the result.
	lines := strings.Split(text, "\n")
	lines = trimTrailingBlanks(lines)
	return strings.Join(lines, "\n"), false
}
