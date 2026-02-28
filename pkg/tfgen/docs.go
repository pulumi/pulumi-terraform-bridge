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
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"unicode"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/afero"
	"github.com/yuin/goldmark"
	gmast "github.com/yuin/goldmark/ast"
	gmtext "github.com/yuin/goldmark/text"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/parse"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfgen/parse/section"
)

const (
	startPulumiCodeChooser = "<!--Start PulumiCodeChooser -->"
	endPulumiCodeChooser   = "<!--End PulumiCodeChooser -->"

	// The Hugo front matter delimiter
	delimiter = "---\n"
)

// argumentDocs contains the documentation metadata for an argument of the resource.
type argumentDocs struct {
	// The description for this argument.
	description string
}

// Included for testing convenience.
func (ad argumentDocs) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Description string `json:"description"`
	}{
		Description: ad.description,
	})
}

// docsPath represents the path to a (possibly nested) TF property.
//
// These are all valid `docsPath`s:
//
// "foo"
// "foo.bar"
// "foo.a.b.c.bar"
//
// Because document parsing is heuristic, it is possible that a docsPath will end up in an
// unexpected state, such as "foo...". Methods should not panic in this scenario, but they
// may return unexpected results.
type docsPath string

func (d docsPath) nested() bool                 { return strings.Contains(string(d), ".") }
func (d docsPath) leaf() string                 { s := d.parts(); return s[len(s)-1] }
func (d docsPath) join(segment string) docsPath { return docsPath(string(d) + "." + segment) }
func (d docsPath) parts() []string              { return strings.Split(string(d), ".") }
func (d docsPath) withOutRoot() docsPath        { s := d.parts(); return docsPath(strings.Join(s[1:], ".")) }

type docsPathArr []docsPath

var _ = (sort.Interface)((docsPathArr)(nil))

func (a docsPathArr) Len() int           { return len(a) }
func (a docsPathArr) Less(i, j int) bool { return a[i] < a[j] }
func (a docsPathArr) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a docsPathArr) Sort()              { sort.Sort(a) }

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
	Arguments map[docsPath]*argumentDocs

	// Attributes includes the names and descriptions for each attribute of the resource
	Attributes map[string]string

	// Import is the import details for the resource
	Import string
}

func (ed *entityDocs) ensure() {
	if ed.Arguments == nil {
		ed.Arguments = map[docsPath]*argumentDocs{}
	}
	if ed.Attributes == nil {
		ed.Attributes = map[string]string{}
	}
}

// DocKind indicates what kind of entity's documentation is being requested.
type DocKind string

const (
	// ResourceDocs indicates documentation pertaining to resource entities.
	ResourceDocs DocKind = "resources"
	// DataSourceDocs indicates documentation pertaining to data source entities.
	DataSourceDocs DocKind = "data-sources"
	// ActionDocs indicates documentation pertaining to action entities.
	ActionDocs DocKind = "actions"
	// InstallationDocs indicates documentation pertaining to provider configuration and installation.
	InstallationDocs DocKind = "installation"
)

func (k DocKind) String() string {
	switch k {
	case DataSourceDocs:
		return "data source"
	case ActionDocs:
		return "action"
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
		return fmt.Sprintf("'%s' (aliased or renamed)", strings.ReplaceAll(rawname, tfbridge.RenamedEntitySuffix, ""))
	}
	return fmt.Sprintf("'%s'", rawname)
}

// getDocsForResource extracts documentation details for the given package from
// TF website documentation markdown content
func getDocsForResource(g *Generator, source DocsSource, kind DocKind,
	rawname string, info tfbridge.ResourceOrDataSourceInfo,
) (entityDocs, error) {
	if g.skipDocs {
		return entityDocs{}, nil
	}
	var docInfo *tfbridge.DocInfo
	if info != nil {
		docInfo = info.GetDocs()
	}

	var docFile *DocFile
	var err error
	switch kind {
	case ResourceDocs:
		docFile, err = source.getResource(rawname, docInfo)
	case DataSourceDocs:
		docFile, err = source.getDatasource(rawname, docInfo)
	case ActionDocs:
		docFile, err = source.getAction(rawname, docInfo)
	default:
		panic("unknown docs kind")
	}

	// If requested, speed up debugging schema generation by targeting only one resource or datasource.
	if t, ok := os.LookupEnv("PULUMI_CONVERT_ONLY"); ok {
		if t != rawname {
			docFile = nil
		}
	}

	if err != nil {
		return entityDocs{}, fmt.Errorf("get docs for token %s: %w", rawname, err)
	}

	if docFile == nil {
		entitiesMissingDocs++
		msg := fmt.Sprintf("could not find docs for %v %v. Override the Docs property in the %v mapping. See "+
			"type tfbridge.DocInfo for details.", kind, formatEntityName(rawname), kind)

		if cmdutil.IsTruthy(os.Getenv("PULUMI_MISSING_DOCS_ERROR")) {
			if docInfo == nil || !docInfo.AllowMissing {
				g.error(msg)
				return entityDocs{}, errors.New(msg)
			}
		}

		// Ideally, we would still want to still return an error here and let upstream callers handle it, but at the
		// time the option to fail the build for missing docs was added (see just above), there are multiple callers of
		// this function who do not expect docs not being found to return an error, and the cost of doing the idiomatic
		// thing (returning an error) was too high.
		g.warn(msg)
		return entityDocs{}, nil
	}

	markdownBytes, markdownFileName := docFile.Content, docFile.FileName

	doc, err := parseTFMarkdown(g, info, kind, markdownBytes, markdownFileName, rawname)
	if err != nil {
		return entityDocs{}, err
	}

	if docInfo != nil {
		// Helper func for readability due to large number of params
		getSourceDocs := func(sourceFrom string) (entityDocs, error) {
			return getDocsForResource(g, source, kind, sourceFrom, nil)
		}

		if docInfo.IncludeAttributesFrom != "" {
			sourceDocs, err := getSourceDocs(docInfo.IncludeAttributesFrom)
			if err != nil {
				return doc, err
			}

			overlayAttributesToAttributes(sourceDocs, doc)
		}

		if docInfo.IncludeAttributesFromArguments != "" {
			sourceDocs, err := getSourceDocs(docInfo.IncludeAttributesFromArguments)
			if err != nil {
				return doc, err
			}

			overlayArgsToAttributes(sourceDocs, doc)
		}

		if docInfo.IncludeArgumentsFrom != "" {
			sourceDocs, err := getSourceDocs(docInfo.IncludeArgumentsFrom)
			if err != nil {
				return doc, err
			}

			overlayArgsToArgs(sourceDocs, &doc)
		}
	}

	return doc, nil
}

type (
	expandedArguments map[string]*argumentNode
	argumentNode      struct {
		docs     *argumentDocs
		children expandedArguments
	}
)

func (a *argumentNode) get(seg string) *argumentNode {
	if a.children == nil {
		a.children = expandedArguments{}
	}
	v, ok := a.children[seg]
	if !ok {
		v = &argumentNode{}
		a.children[seg] = v
	}
	return v
}

func argumentTree(m map[docsPath]*argumentDocs) expandedArguments {
	topLevel := &argumentNode{}
	for k, v := range m {
		a := topLevel
		for _, segment := range k.parts() {
			a = a.get(segment)
		}
		a.docs = v
	}
	return topLevel.children
}

func (t expandedArguments) collapse() map[docsPath]*argumentDocs {
	m := make(map[docsPath]*argumentDocs, len(t))

	var set func(docsPath, expandedArguments)
	set = func(path docsPath, args expandedArguments) {
		for k, v := range args {
			p := path.join(k)
			if v.docs != nil {
				m[p] = v.docs
			}
			set(p, v.children)
		}
	}

	for k, v := range t {
		if v.docs != nil {
			m[docsPath(k)] = v.docs
		}
		set(docsPath(k), v.children)
	}
	return m
}

func overlayAttributesToAttributes(sourceDocs entityDocs, targetDocs entityDocs) {
	for k, v := range sourceDocs.Attributes {
		targetDocs.Attributes[k] = v
	}
}

func overlayArgsToAttributes(sourceDocs entityDocs, targetDocs entityDocs) {
	for k, v := range sourceDocs.Arguments {
		targetDocs.Attributes[k.leaf()] = v.description
	}
}

func overlayArgsToArgs(sourceDocs entityDocs, docs *entityDocs) {
	docsArgs := argumentTree(docs.Arguments)

	for k, v := range argumentTree(sourceDocs.Arguments) { // string -> argument
		docsArgs[k] = v
	}
	docs.Arguments = docsArgs.collapse()
}

//nolint:lll
var (
	// For example:
	// [1]: https://docs.aws.amazon.com/lambda/latest/dg/welcome.html
	linkFooterRegexp = regexp.MustCompile(`(?m)^(\[\d+\]):\s(.*)`)

	argumentBulletRegexp = regexp.MustCompile(
		"^\\s*[*+-]\\s*`([a-z0-9_]*)`\\s*(\\([a-zA-Z]*\\)\\s*)?\\s*[:–-]?\\s*(\\([^\\)]*\\)[-\\s]*)?(.*)",
	)

	bulletPointRegexStr       = "^\\s*[*+-]"          // matches any bullet point-like character
	attributePathNameRegexStr = "\\s*`([a-z0-9._]*)`" // matches any TF attribute path name

	// matches any line starting with a bullet point followed by a TF path or resource name)
	attributeBulletRegexp = regexp.MustCompile(
		bulletPointRegexStr + attributePathNameRegexStr + "\\s*[:–-]?\\s*(.*)",
	)

	attributionFormatString = "This Pulumi package is based on the [`%[1]s` Terraform Provider](https://%[3]s/%[2]s/terraform-provider-%[1]s)."
	listMarkerRegex         = regexp.MustCompile("[-*+]")
)

func trimFrontMatter(text []byte) []byte {
	body, ok := bytes.CutPrefix(text, []byte("---"))
	if !ok {
		return text
	}
	idx := bytes.Index(body, []byte("\n---"))

	// Unable to find closing, so just return.
	if idx == -1 {
		return text
	}
	return body[idx+4:]
}

func splitByMarkdownHeaders(text string, level int) [][]string {
	// splitByMarkdownHeaders parses text, then walks the resulting AST to find
	// appropriate header nodes. It uses the location of these header nodes to split
	// text into sections, which are then split into lines.

	bytes := trimFrontMatter([]byte(text))

	offset := len(text) - len(bytes)
	contract.Assertf(offset >= 0, "The offset generated by chopping of the front-matter cannot be negative")

	gm := goldmark.New(goldmark.WithExtensions(parse.TFRegistryExtension))

	headers := []int{}
	parse.WalkNode(gm.Parser().Parse(gmtext.NewReader(bytes)), func(heading *gmast.Heading) {
		if heading.Level != level {
			return
		}
		if heading.Lines().Len() == 0 {
			return
		}
		for i := heading.Lines().At(0).Start; i > 0; i-- {
			if bytes[i] == '\n' {
				headers = append(headers,
					// +1 to move past the \n
					//
					// +offset to move past the front-matter (if present)
					i+1+offset)
				return
			}
		}
	})

	// headers now contains the index into `bytes` that represent the start of each
	// section.
	//
	// We now use that information to extract sections from `text`.

	sections := make([][]string, 0, len(headers)+1)
	for _, section := range splitStringsAtIndexes(text, headers) {
		sections = append(sections, strings.Split(section, "\n"))
	}

	return sections
}

func splitStringsAtIndexes(s string, splits []int) []string {
	if len(splits) == 0 {
		return []string{s}
	}

	parts := make([]string, 0, len(splits)+1)

	// Account for the first section
	parts = append(parts, s[:splits[0]-1])

	// Now handle the middle section
	for from := 0; from+1 < len(splits); from++ {
		parts = append(parts, s[splits[from]:splits[from+1]-1])
	}

	// Account for the end section
	parts = append(parts, s[splits[len(splits)-1]:])

	return parts
}

// parseTFMarkdown takes a TF website markdown doc and extracts a structured representation for use in
// generating doc comments
func parseTFMarkdown(g *Generator, info tfbridge.ResourceOrDataSourceInfo, kind DocKind,
	markdown []byte, markdownFileName, rawname string,
) (entityDocs, error) {
	p := &tfMarkdownParser{
		sink:             g,
		info:             info,
		kind:             kind,
		markdownFileName: markdownFileName,
		rawname:          rawname,

		infoCtx: infoContext{
			language: g.language,
			pkg:      g.pkg,
			info:     g.info,
		},
		editRules: g.editRules,
	}
	return p.parse(markdown)
}

type diagsSink interface {
	debug(string, ...interface{})
	warn(string, ...interface{})
	error(string, ...interface{})
}

type tfMarkdownParser struct {
	sink             diagsSink
	info             tfbridge.ResourceOrDataSourceInfo
	kind             DocKind
	markdownFileName string
	rawname          string

	infoCtx   infoContext
	editRules editRules

	ret entityDocs

	// readFile allows tests to mock out external files. It should not be set outside
	// of test cases.
	readFileFunc func(string) ([]byte, error)
}

const (
	sectionOther               = 0
	sectionExampleUsage        = 1
	sectionArgsReference       = 2
	sectionAttributesReference = 3
	sectionFrontMatter         = 4
	sectionImports             = 5
)

func (p *tfMarkdownParser) readFile(name string) ([]byte, error) {
	if p.readFileFunc != nil {
		// p.readFileFunc is a testing hard-point. Outside of tests,
		// p.readFileFunc should be nil.
		return p.readFileFunc(name)
	}
	return os.ReadFile(name)
}

func (p *tfMarkdownParser) readSupplementaryExamples() (string, error) {
	examplesFileName := fmt.Sprintf("docs/%s/%s.examples.md", p.kind, p.rawname)
	absPath, err := filepath.Abs(examplesFileName)
	if err != nil {
		return "", err
	}
	fileBytes, err := p.readFile(absPath)
	if err != nil {
		p.sink.error("explicitly marked resource documentation for replacement, but found no file at %q", examplesFileName)
		return "", err
	}

	return string(fileBytes), nil
}

func (p *tfMarkdownParser) parse(tfMarkdown []byte) (entityDocs, error) {
	p.ret = entityDocs{
		Arguments:  make(map[docsPath]*argumentDocs),
		Attributes: make(map[string]string),
	}
	var err error
	tfMarkdown, err = p.editRules.apply(p.markdownFileName, tfMarkdown, info.PreCodeTranslation)
	if err != nil {
		return entityDocs{}, fmt.Errorf("file %s: %w", p.markdownFileName, err)
	}
	markdown := string(tfMarkdown)

	// Replace any Windows-style newlines.
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")

	// Replace redundant comment.
	markdown = strings.ReplaceAll(markdown, "<!-- schema generated by tfplugindocs -->", "")

	// Split the sections by H2 topics in the Markdown file.
	sections := splitByMarkdownHeaders(markdown, 2)

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
		newExamples, err := p.readSupplementaryExamples()
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

	doc, _ := cleanupDoc(p.rawname, p.sink, p.infoCtx, p.ret, footerLinks)
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
		if len(s) == 0 {
			// Skip empty sections.
			continue
		}
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
				for s = s[1:]; len(s) > 0 && isBlank(s[0]); {
					s = s[1:]
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
		p.sink.warn("Unparseable H2 doc section for %v; consider overriding doc source location", p.rawname)
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
		p.sink.debug("Ignoring doc section [%v] for [%v]", header, p.rawname)
		ignoredDocHeaders[header]++
		return nil
	case "Example Usage", "Example":
		sectionKind = sectionExampleUsage
	case "Arguments Reference", "Argument Reference", "Argument reference", "Nested Blocks", "Nested blocks", "Arguments":
		sectionKind = sectionArgsReference
	case "Attributes Reference", "Attribute Reference", "Attribute reference", "Attributes":
		sectionKind = sectionAttributesReference
	case "Import", "Imports":
		sectionKind = sectionImports
	case "---":
		sectionKind = sectionFrontMatter
	case "Schema":
		p.parseSchemaWithNestedSections(h2Section)
		return nil
	}

	if sectionKind == sectionImports {
		reformattedH2Section, isEmpty := p.reformatSubsection(h2Section[1:])
		if isEmpty {
			return nil
		}
		p.parseImports(strings.Join(reformattedH2Section, "\n"))
		return nil
	}

	// Now split the sections by H3 topics. This is done because we'll ignore sub-sections with code
	// snippets that are unparseable (we don't want to ignore entire H2 sections).
	var wroteHeader bool
	for _, h3Section := range splitByMarkdownHeaders(strings.Join(h2Section[1:], "\n"), 3) {
		if len(h3Section) == 0 {
			// An unparseable H3 appears (as observed by building a few tier 1 providers) to typically be due to an
			// empty section resulting from how we parse sections earlier in the docs generation process. Therefore, we
			// log it as debug output:
			p.sink.debug("empty or unparseable H3 doc section for %v; consider overriding doc source location",
				p.rawname, p.kind)
			continue
		}

		// Remove the "Open in Cloud Shell" button if any and check for the presence of code snippets.
		reformattedH3Section, isEmpty := p.reformatSubsection(h3Section)
		if isEmpty {
			// Skip empty subsections (they just add unnecessary padding and headers).
			continue
		}

		// Now process the content based on the H2 topic. These are mostly standard across TF's docs.
		switch sectionKind {
		case sectionArgsReference:
			parseArgReferenceSection(reformattedH3Section, &p.ret)
		case sectionAttributesReference:
			parseAttributesReferenceSection(reformattedH3Section, &p.ret)
		case sectionFrontMatter:
			p.parseFrontMatter(reformattedH3Section)
		case sectionImports:
			p.parseImports(strings.Join(reformattedH3Section, "\n"))
		default:
			// Determine if this is a nested argument section.
			_, isArgument := p.ret.Arguments[docsPath(header)]
			if isArgument || strings.HasSuffix(header, "Configuration Block") {
				parseArgReferenceSection(reformattedH3Section, &p.ret)
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
		p.sink.warn(fmt.Sprintf("error: Failure in parsing resource name: %s, subsection: %s", p.rawname, subsection[0]))
		return
	}
	if topLevelSchema == nil {
		p.sink.warn("Failed to parse top-level Schema section")
		return
	}
	parseTopLevelSchemaIntoDocs(&p.ret, topLevelSchema, p.sink.warn)
}

type markdownLineInfo struct {
	name, desc string
	isFound    bool
}

type bulletListEntry struct {
	name  string
	index int
}

// trackBulletListIndentation looks at the index of the bullet list marker ( `*`, `-` or `+`) in a docs line and
// compares it to a collection that tracks the level of list nesting by comparing to the previous list entry's nested
// level (if any).
// Note that this function only looks at the placement of the bullet list marker, and assumes same-level list markers
// to be in the same location in each line. This is not necessarily the case for Markdown, which considers a range of
// locations within 1-4 whitespace characters, as well as considers the start index of the text following the bullet
// point. If and when this becomes an issue during docs parsing, we may consider adding some of those rules here.
// Read more about nested lists in GitHub-flavored Markdown:
// https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax#nested-lists
//
//nolint:lll
func trackBulletListIndentation(line, name string, tracker []bulletListEntry) []bulletListEntry {
	listMarkerLocation := listMarkerRegex.FindStringIndex(line)
	contract.Assertf(len(listMarkerLocation) == 2,
		"Expected to find bullet list marker in line %s", line)
	listMarkerIndex := listMarkerLocation[0]

	// If our tracker is empty, we are at list nested level 0.
	if len(tracker) == 0 {
		newEntry := bulletListEntry{
			name:  name,
			index: listMarkerIndex,
		}
		return append(tracker, newEntry)
	}
	// Always compare to last entry in tracker
	lastListEntry := tracker[len(tracker)-1]

	// if current line's listMarkerIndex is greater than the tracker's last entry's listMarkerIndex,
	// make a new tracker entry and push it on there with all the info.
	if listMarkerIndex > lastListEntry.index {
		name = lastListEntry.name + "." + name
		newEntry := bulletListEntry{
			name:  name,
			index: listMarkerIndex,
		}
		return append(tracker, newEntry)
	}
	// if current line's listMarkerIndex is the same as the last entry's, we're at the same level.
	if listMarkerIndex == lastListEntry.index {
		// Replace the last entry in our tracker
		replaceEntry := bulletListEntry{
			index: listMarkerIndex,
		}
		if len(tracker) == 1 {
			replaceEntry.name = name
		} else {
			// use the penultimate entry name to build current name
			replaceName := tracker[(len(tracker)-2)].name + "." + name
			replaceEntry.name = replaceName
		}
		return append(tracker[:len(tracker)-1], replaceEntry)
	}

	// The current line's listMarkerIndex is smaller that the previous entry's.
	// Pop off the latest entry, and retry to see if the next previous entry is a match.
	return trackBulletListIndentation(line, name, tracker[:len(tracker)-1])
}

// parseArgFromMarkdownLine takes a line of Markdown and attempts to parse it for a Terraform argument and its
// description. It returns a struct containing the name and description of the arg, and whether an arg was found.
func parseArgFromMarkdownLine(line string) markdownLineInfo {
	matches := argumentBulletRegexp.FindStringSubmatch(line)
	var parsed markdownLineInfo
	if len(matches) > 4 {
		parsed.name = matches[1]
		parsed.desc = matches[4]
		parsed.isFound = true
	}
	return parsed
}

var genericNestedRegexp = regexp.MustCompile("supports? the following:")

var nestedObjectRegexps = []*regexp.Regexp{
	// For example:
	// s3_bucket.html.markdown: "The `website` object supports the following:"
	// appmesh_gateway_route.html.markdown:
	//		"The `grpc_route`, `http_route` and `http2_route` objects supports the following:"
	// ami.html.markdown: "When `virtualization_type` is "hvm" the following additional arguments apply:"
	regexp.MustCompile("`([a-z_0-9]+)`.*following"),

	// For example:
	// athena_workgroup.html.markdown: "#### result_configuration Argument Reference"
	regexp.MustCompile("(?i)## ([a-z_0-9]+).* argument reference"),

	// For example:
	// codebuild_project.html.markdown: "#### build_batch_config: restrictions"
	// codebuild_project.html.markdown: "#### logs_config: s3_logs"
	regexp.MustCompile("###+ ([a-zA-Z_0-9]+: [a-zA-Z_0-9]+).*"),

	// For example:
	// elasticsearch_domain.html.markdown: "### advanced_security_options"
	regexp.MustCompile("###+ ([a-z_0-9]+).*"),

	// For example:
	// dynamodb_table.html.markdown: "### `server_side_encryption`"
	regexp.MustCompile("###+ `([a-z_0-9]+).*`"),

	// For example:
	// route53_record.html.markdown: "### Failover Routing Policy"
	// appflow_flow.html.markdown: "###### S3 Input Format Config"
	regexp.MustCompile("###+ ([a-zA-Z_ 0-9]+).*"),

	// For example:
	// sql_database_instance.html.markdown:
	// 		"The optional `settings.ip_configuration.authorized_networks[]`` sublist supports:"
	regexp.MustCompile("`([a-zA-Z_.\\[\\]]+)`.*supports:"),

	// For example when blocks/subblocks/sublists are defined across more than one line
	// sql_database_instance.html.markdown:
	//		"The optional `settings.maintenance_window` subblock for instances declares a one-hour"
	regexp.MustCompile("The .* `([a-zA-Z_.\\[\\]]+)` sublist .*"),
	regexp.MustCompile("The .* `([a-zA-Z_.\\[\\]]+)` subblock .*"),
	regexp.MustCompile("The .* `([a-zA-Z_.\\[\\]]+)` block .*"),
}

// getMultipleNestedBlockNames is called when we detect that a resource matches the "`([a-z_0-9]+)`.*following" regex.
// We check if more than one nested block name is listed on this line, and we additionally check if there's an
// indication of an extra nested object, denoted by "'s", as in:
// "The `grpc_route`, `http_route` and `http2_route` 's `action` object supports the following:".
func getMultipleNestedBlockNames(match string) []string {
	subNest := ""
	var nestedBlockNames []string
	// First we check for the presence of a possible nested property via 's
	if strings.Contains(match, "'s ") {
		// split the match along the 's
		part1, part2, _ := strings.Cut(match, "'s")
		match = part1
		// Extract our subheading. It should be the second item in part2 of the match.
		part2Slice := strings.Split(part2, "`")
		if len(part2Slice) >= 2 {
			subNest = part2Slice[1]
		}
	}
	// As per previous regex match, the resource names will be surrounded by backticks, so we extract them
	tokenInBackticks := regexp.MustCompile("`[^`]+`")
	blockNames := tokenInBackticks.FindAllString(match, -1)
	for _, blockName := range blockNames {
		if blockName != "" {
			blockName = strings.Trim(blockName, "`")
			blockName = strings.ToLower(blockName)
			blockName = strings.ReplaceAll(blockName, " ", "_")
			blockName = strings.TrimSuffix(blockName, "[]")
			if subNest != "" {
				// For the format:
				//
				//		The `grpc_route`, `http_route` and `http2_route` 's `action` object supports the following:
				//
				// the result should be grpc_route.action
				blockName = blockName + "." + subNest
			}
			nestedBlockNames = append(nestedBlockNames, blockName)
		}
	}
	return nestedBlockNames
}

// getNestedNameWithColon handles cases where a property is shown as a subpoperty by means of a colon in the docs.
// For example:
//
//	#### logs_config: s3_logs
//
// should return []string{"logs_config.s3_logs"}.
func getNestedNameWithColon(match string) []string {
	var nestedBlockNames []string
	parts := strings.Split(match, ":")

	blockName := strings.ToLower(parts[0])
	blockName = strings.ReplaceAll(blockName, " ", "_")

	subNest := strings.ToLower(parts[1])
	subNest = strings.TrimSpace(subNest)
	subNest = strings.ReplaceAll(subNest, " ", "_")

	blockName = blockName + "." + subNest
	nestedBlockNames = append(nestedBlockNames, blockName)
	return nestedBlockNames
}

// getNestedBlockNames takes line of a Terraform docs Markdown page and returns the name(s) of the nested block it
// describes. If the line does not describe a nested block, an empty string is returned.
//
// Examples of nested blocks include (but are not limited to):
//
// - "The `private_cluster_config` block supports:" -> "private_cluster_config"
// - "The optional settings.backup_configuration subblock supports:" -> "settings.backup_configuration"
func getNestedBlockNames(line string) []string {
	var nestedBlockNames []string

	for i, match := range nestedObjectRegexps {
		matches := match.FindStringSubmatch(line)

		// If we match with the first regex, we have to see if we've got many to many matching for resources going on.
		if len(matches) >= 2 && i == 0 {
			nestedBlockNames = getMultipleNestedBlockNames(matches[0])
			break
		} else if len(matches) >= 2 && i == 2 {
			// there's a colon in the subheader; split the line
			nestedBlockNames = getNestedNameWithColon(matches[1])
			break
		} else if len(matches) >= 2 {
			nested := strings.ToLower(matches[1])
			nested = strings.ReplaceAll(nested, " ", "_")
			nested = strings.TrimSuffix(nested, "[]")
			nestedBlockNames = append(nestedBlockNames, nested)
			break
		}
	}
	return nestedBlockNames
}

func parseArgReferenceSection(subsection []string, ret *entityDocs) {
	// Variable to remember the last argument we found.
	var lastMatch string
	// Collection to hold all arguments that headline a nested description.
	var nesteds []docsPath

	addNewHeading := func(name, desc, line string) {
		// found a property bullet, extract the name and description
		if len(nesteds) > 0 {
			for _, nested := range nesteds {
				// We found this line within a nested field. We should record it as such.
				if ret.Arguments[nested] == nil {
					totalArgumentsFromDocs++
				}
				ret.Arguments[nested.join(name)] = &argumentDocs{desc}
			}
		} else {
			if genericNestedRegexp.MatchString(line) {
				return
			}
			ret.Arguments[docsPath(name)] = &argumentDocs{description: desc}
			totalArgumentsFromDocs++
		}
	}
	// This function adds the current line as a description to the last matched resource,
	// in cases where there's no resource match found on this line.
	// It represents a multi-line description for a field.
	extendExistingHeading := func(line string) {
		if len(nesteds) > 0 {
			for _, nested := range nesteds {
				line = "\n" + strings.TrimSpace(line)
				ret.Arguments[nested.join(lastMatch)].description += line
			}
		} else {
			if genericNestedRegexp.MatchString(line) {
				lastMatch = ""
				nesteds = []docsPath{}
				return
			}
			line = "\n" + strings.TrimSpace(line)
			arg := ret.Arguments[docsPath(lastMatch)]
			if arg != nil {
				arg.description += line
			}
		}
	}

	// hadSpace tells us if the previous line was blank.
	var hadSpace bool

	// bulletListTracker is a stack-like collection that tracks the level of nesting for a bulleted list with
	// nested lists. The name of the topmost entry represents the nested docs path for the current line.
	var bulletListTracker []bulletListEntry

	for _, line := range subsection {
		parsedArg := parseArgFromMarkdownLine(line)
		matchFound := parsedArg.isFound
		if matchFound { // We have found a new property bullet point.
			desc := parsedArg.desc
			bulletListTracker = trackBulletListIndentation(line, parsedArg.name, bulletListTracker)
			name := bulletListTracker[len(bulletListTracker)-1].name
			lastMatch = name
			addNewHeading(name, desc, line)
		} else if strings.TrimSpace(line) == "---" {
			// --- is a markdown section break. This probably indicates the
			// section is over, but we take it to mean that the current
			// heading is over.
			lastMatch = ""
			bulletListTracker = nil
		} else if nestedBlockCurrentLine := getNestedBlockNames(line); hadSpace && len(nestedBlockCurrentLine) > 0 {
			// This tells us if there's a resource that is about to have subfields (nesteds)
			// in subsequent lines.
			// empty nesteds
			nesteds = []docsPath{}
			for _, item := range nestedBlockCurrentLine {
				nesteds = append(nesteds, docsPath(item))
			}
			lastMatch = ""
			bulletListTracker = nil
		} else if !isBlank(line) && lastMatch != "" {
			// This appends the current line to the previous match's description.
			extendExistingHeading(line)
		} else if nestedBlockCurrentLine := getNestedBlockNames(line); len(nestedBlockCurrentLine) > 0 {
			// This tells us if there's a resource that is about to have subfields (nesteds)
			// in subsequent lines.
			// empty nesteds
			nesteds = []docsPath{}
			for _, item := range nestedBlockCurrentLine {
				nesteds = append(nesteds, docsPath(item))
			}
			lastMatch = ""
			bulletListTracker = nil
		} else if lastMatch != "" {
			extendExistingHeading(line)
		}
		hadSpace = isBlank(line)
	}

	for _, v := range ret.Arguments {
		v.description = strings.TrimRightFunc(v.description, unicode.IsSpace)
	}
}

func parseAttributesReferenceSection(subsection []string, ret *entityDocs) {
	var lastMatch string
	for _, line := range subsection {
		matches := attributeBulletRegexp.FindStringSubmatch(line)
		if len(matches) >= 2 {
			// found a property bullet, extract the name and description
			attribute := flattenListAttributeKey(matches[1])
			description := matches[2]
			ret.Attributes[attribute] = description
			lastMatch = attribute
		} else if !isBlank(line) && lastMatch != "" {
			// this is a continuation of the previous bullet
			ret.Attributes[lastMatch] += "\n" + strings.TrimSpace(line)
		} else {
			// This is an empty line or there were no bullets yet - clear the lastMatch
			lastMatch = ""
		}
	}
}

// flattenListAttributeKey removes a TF index from a docs string.
// In a TF schema.TypeList the `.0` index is used to access the list itself, but it does not translate into our docs
// path lookup, so we remove it here.
// A search through the pulumi-aws schema as well as upstream/website do not show any descriptions != `.0`; it appears
// that indices > 0 are not currently used in the TF schema definitions.
func flattenListAttributeKey(attribute string) string {
	return strings.ReplaceAll(attribute, ".0", "")
}

func (p *tfMarkdownParser) parseImports(body string) {
	var token string
	if p.info != nil && p.info.GetTok() != "" {
		token = p.info.GetTok().String()
	}

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

	if token == "" {
		token = "MISSING_TOK"
	}
	if rewritten, ok := rewriteImportMarkdown(body, token); ok {
		p.ret.Import = rewritten
	}
}

var (
	fenceStart = regexp.MustCompile("^([\\t ]*)```(.*)$")
	fenceEnd   = regexp.MustCompile("^([\\t ]*)```\\s*$")
)

// rewriteImportMarkdown scans an import section, preserving raw markdown except for
// Terraform import examples which are rewritten to pulumi import syntax.
//
// Example (input):
//
//	Import is supported using the following syntax:
//	```shell
//	terraform import snowflake_api_integration.example name
//	```
//
// Example (output):
//
//	## Import
//
//	Import is supported using the following syntax:
//	```sh
//	$ pulumi import snowflake:index/apiIntegration:ApiIntegration example name
//	```
func rewriteImportMarkdown(body, typeToken string) (string, bool) {
	if body == "" {
		return "", false
	}

	var out []string
	var fenceLines []string
	var fenceIndent string
	var fenceInfo string
	var fenceStartLine string
	var inFence bool

	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if !inFence {
			if matches := fenceStart.FindStringSubmatch(line); matches != nil {
				// Example:
				//   line: "  ```shell"
				//   matches[0]: "  ```shell", matches[1]: "  ", matches[2]: "shell"
				inFence = true
				// Example: matches[1] == "  " (leading indentation before the fence).
				fenceIndent = matches[1]
				// Example: matches[2] == "shell" (raw fence info string).
				fenceInfo = strings.TrimSpace(matches[2])
				fenceStartLine = line
				fenceLines = fenceLines[:0]
				continue
			}
			out = append(out, line)
			continue
		}

		// End of a fenced block: rewrite or drop the fence based on its contents.
		// This acts as a post-processor for the collected fence lines.
		if fenceEnd.MatchString(line) {
			rewritten, keep, dropSyntaxLine := rewriteImportFence(
				fenceStartLine, line, fenceIndent, fenceInfo, fenceLines, typeToken)
			if keep {
				if dropSyntaxLine {
					out = dropImportSyntaxLine(out)
				}
				out = append(out, rewritten...)
			}
			inFence = false
			continue
		}
		fenceLines = append(fenceLines, line)
	}

	if inFence {
		out = append(out, fenceStartLine)
		out = append(out, fenceLines...)
	}

	// Normalize the collected lines:
	// - Drop boilerplate "Import is supported..." line (always)
	// - Trim leading/trailing blank lines from the assembled block
	// - Drop an existing "## Import" heading from the source to avoid duplicates
	// - If there's no remaining content, signal "no import section"
	//
	// Example:
	//   out == []string{"", "## Import", "", "Import is supported using the following syntax:",
	//                   "", "```sh", "$ pulumi import ...", "```", ""}
	//   content becomes: "Import is supported using the following syntax:\n\n```sh\n$ pulumi import ...\n```"
	out = dropImportSyntaxLine(out)
	content := strings.Trim(strings.Join(out, "\n"), "\n")
	if content != "" {
		lines := strings.Split(content, "\n")
		i := 0
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}
		if i < len(lines) {
			heading := strings.TrimSpace(lines[i])
			if heading == "## Import" || heading == "# Import" {
				i++
				for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
					i++
				}
				lines = lines[i:]
			}
		}
		content = strings.Trim(strings.Join(lines, "\n"), "\n")
	}
	if strings.TrimSpace(content) == "" {
		return "", false
	}
	return fmt.Sprintf("## Import\n\n%s\n\n", content), true
}

// rewriteImportFence rewrites a fenced import block, preserving raw content except for
// Terraform import examples which are converted to pulumi import syntax.
//
// Example (input fence content):
//
//	# Example:
//	terraform import auth0_pages.my_pages "22f4f21b-017a-319d-92e7-2291c1ca36c4"
//
// Example (output fence content):
//
//	# Example:
//	$ pulumi import auth0/index/pages:Pages my_pages "22f4f21b-017a-319d-92e7-2291c1ca36c4"
func rewriteImportFence(
	startLine, endLine, indent, info string, lines []string, typeToken string,
) ([]string, bool, bool) {
	if fields := strings.Fields(info); len(fields) > 0 {
		info = fields[0]
	} else {
		info = ""
	}
	code := strings.Join(lines, "\n")
	if info == "terraform" && looksLikeTerraformImportBlock(code) {
		return nil, false, false
	}
	if info == "" || info == "console" || info == "shell" || info == "sh" || info == "bash" {
		forcedStart := startLine
		forcedEnd := endLine
		if info == "" || info == "console" || info == "shell" || info == "bash" {
			forcedStart = fmt.Sprintf("%s```sh", indent)
			forcedEnd = fmt.Sprintf("%s```", indent)
		}
		hoistMode := hasHoistableImportComments(lines)
		if hoistMode {
			if rewritten, updated := rewriteImportFenceWithHoistedComments(lines, indent, typeToken); updated {
				return rewritten, true, true
			}
		}
		// Rewrite `terraform import` examples inside shell/console fences.
		//
		// Example (input fence lines):
		//   # Example:
		//   terraform import auth0_pages.my_pages "..."
		// Example (output fence lines):
		//   # Example:
		//   $ pulumi import auth0/index/pages:Pages my_pages "..."
		comments, remaining, hoist := extractImportFenceComments(lines, indent)
		rewritten, updated := rewriteImportLines(remaining, typeToken)
		if updated {
			out := make([]string, 0, len(rewritten)+2)
			if hoist && len(comments) > 0 {
				out = append(out, comments...)
				out = append(out, fmt.Sprintf("%s```sh", indent))
				out = append(out, rewritten...)
				out = append(out, fmt.Sprintf("%s```", indent))
				return out, true, true
			}
			fenceStart := forcedStart
			fenceEnd := forcedEnd
			out = append(out, fenceStart)
			out = append(out, rewritten...)
			out = append(out, fenceEnd)
			return out, true, false
		}
	}

	rewritten := make([]string, 0, len(lines)+2)
	if info == "" || info == "console" || info == "shell" || info == "bash" {
		rewritten = append(rewritten, fmt.Sprintf("%s```sh", indent))
	} else {
		rewritten = append(rewritten, startLine)
	}
	rewritten = append(rewritten, lines...)
	rewritten = append(rewritten, fmt.Sprintf("%s```", indent))
	return rewritten, true, false
}

func hasHoistableImportComments(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if isHoistablePreambleLine(trimmed) {
			return true
		}
	}
	return false
}

func rewriteImportFenceWithHoistedComments(lines []string, indent, typeToken string) ([]string, bool) {
	var out []string
	updated := false
	remaining := lines
	for len(remaining) > 0 {
		trimmed := strings.TrimLeft(remaining[0], " \t")
		if isHoistablePreambleLine(trimmed) {
			comments := []string{}
			sawComment := false
			for len(remaining) > 0 {
				trimmed = strings.TrimLeft(remaining[0], " \t")
				if strings.HasPrefix(trimmed, "#") {
					sawComment = true
					comments = append(comments, strings.TrimSpace(strings.TrimPrefix(trimmed, "#")))
					remaining = remaining[1:]
					continue
				}
				if trimmed == "" && sawComment {
					comments = append(comments, "")
					remaining = remaining[1:]
					continue
				}
				if trimmed != "" && isHoistablePreambleLine(trimmed) {
					sawComment = true
					comments = append(comments, trimmed)
					remaining = remaining[1:]
					continue
				}
				break
			}
			if len(comments) > 0 {
				out = append(out, formatHoistedImportComments(comments, indent)...)
			}
			continue
		}

		chunk := []string{}
		for len(remaining) > 0 {
			trimmed = strings.TrimLeft(remaining[0], " \t")
			if strings.HasPrefix(trimmed, "#") {
				break
			}
			chunk = append(chunk, remaining[0])
			remaining = remaining[1:]
		}
		if len(chunk) == 0 {
			continue
		}
		if allBlank(chunk) {
			continue
		}
		chunk, hadTrailingBlank := trimTrailingBlankLines(chunk)
		rewritten, chunkUpdated := rewriteImportLines(chunk, typeToken)
		if chunkUpdated {
			updated = true
		}
		out = trimTrailingBlankLinesTo(out, 1)
		out = append(out, fmt.Sprintf("%s```sh", indent))
		out = append(out, rewritten...)
		out = append(out, fmt.Sprintf("%s```", indent))
		if hadTrailingBlank {
			out = append(out, "")
			continue
		}
		if nextNonBlankIsComment(remaining) {
			out = append(out, "")
		}
	}
	return out, updated
}

func rewriteImportLines(lines []string, typeToken string) ([]string, bool) {
	updated := false
	rewritten := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		line := lines[i]
		leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		trimmed := strings.TrimLeft(line, " \t")

		if strings.Contains(trimmed, "terraform import") || strings.Contains(trimmed, "pulumi import") {
			j := i
			cmdLines := []string{trimmed}
			for strings.HasSuffix(strings.TrimRight(lines[j], " \t"), "\\") && j+1 < len(lines) {
				j++
				cmdLines = append(cmdLines, strings.TrimLeft(lines[j], " \t"))
			}
			cmd := strings.Join(cmdLines, "\n")
			if parsed, ok := parseImportCode(cmd); ok {
				rewritten = append(rewritten,
					fmt.Sprintf("%s$ pulumi import %s %s %s", leading, typeToken, parsed.Name, parsed.ID))
				updated = true
				i = j + 1
				continue
			}

			for k := i; k <= j; k++ {
				rewritten = append(rewritten, lines[k])
			}
			i = j + 1
			continue
		}

		rewritten = append(rewritten, line)
		i++
	}
	return rewritten, updated
}

// extractImportFenceComments hoists leading comment lines (starting with '#') from a fence.
//
// Example (input):
//
//	# As this is not a resource identifiable by an ID within the Auth0 Management API,
//	# pages can be imported using a random string.
//	terraform import auth0_pages.my_pages "..."
//
// Example (output):
//
//	comments: []string{"As this is not a resource identifiable by an ID within the Auth0 Management API,",
//	                   "", "pages can be imported using a random string.", ""}
//	remaining: []string{"terraform import auth0_pages.my_pages \"...\""}
func extractImportFenceComments(lines []string, indent string) ([]string, []string, bool) {
	var comments []string
	remaining := lines
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "#") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			comments = append(comments, trimmed)
			remaining = lines[i+1:]
			continue
		}
		if trimmed == "" {
			if len(comments) > 0 {
				comments = append(comments, "")
				remaining = lines[i+1:]
				continue
			}
		}
		if trimmed != "" && isHoistablePreambleLine(trimmed) {
			comments = append(comments, trimmed)
			remaining = lines[i+1:]
			continue
		}
		break
	}
	hoist := len(comments) > 0
	if len(comments) == 0 || !hoist {
		return nil, lines, false
	}
	return formatHoistedImportComments(comments, indent), remaining, true
}

func formatHoistedImportComments(comments []string, indent string) []string {
	out := make([]string, 0, len(comments)+1)
	for _, line := range comments {
		if line == "" {
			out = append(out, "")
			continue
		}
		out = append(out, indent+line)
	}
	out = append(out, "")
	return out
}

func trimTrailingBlankLines(lines []string) ([]string, bool) {
	i := len(lines)
	for i > 0 && strings.TrimSpace(lines[i-1]) == "" {
		i--
	}
	return lines[:i], i != len(lines)
}

func trimTrailingBlankLinesTo(lines []string, max int) []string {
	if max < 0 {
		max = 0
	}
	i := len(lines)
	for i > 0 && strings.TrimSpace(lines[i-1]) == "" {
		i--
	}
	blankCount := len(lines) - i
	if blankCount <= max {
		return lines
	}
	out := make([]string, 0, i+max)
	out = append(out, lines[:i]...)
	for j := 0; j < max; j++ {
		out = append(out, "")
	}
	return out
}

func allBlank(lines []string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return false
		}
	}
	return true
}

func nextNonBlankIsComment(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		return strings.HasPrefix(trimmed, "#")
	}
	return false
}

func isHoistablePreambleLine(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "$") {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "terraform import") || strings.Contains(lower, "pulumi import") {
		return false
	}
	return true
}

// dropImportSyntaxLine removes boilerplate import-syntax lines anywhere in the section.
//
// Example:
//
//	before: ["Import is supported using the following syntax:", ""]
//	after:  [""]
func dropImportSyntaxLine(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, "Import is supported using the following syntax") ||
			strings.Contains(line, "`terraform import` command") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// looksLikeTerraformImportBlock detects the HCL `import { ... }` syntax so it can be dropped.
//
// Example:
// ```terraform
//
//	import {
//	  to = aws_iam_role.example
//	  id = "developer_name"
//	}
//
// ```
func looksLikeTerraformImportBlock(code string) bool {
	for _, line := range strings.Split(code, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		return strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "import{") ||
			strings.HasPrefix(trimmed, "import {")
	}
	return false
}

// Parses import example codeblocks.
//
// Example matching strings:
//
//	% pulumi import some_resource.name someID
//	$ terraform import some_resource.name <some-ID>
//	$ terraform import \
//	      some_resource.name \
//	      <some-ID>
var importCodePattern = regexp.MustCompile(
	`^\s*(?:[%$]\s+)?(?:pulumi|terraform) import[\\\s]+([^.]+)[.]([^\s]+)[\\\s]+([^\s]+)\s*$`)

// Recognize import example codeblocks.
//
// Example:
//
//	s := "% pulumi import aws_accessanalyzer_analyzer.example exampleID"
//	v, ok := parseImportCode(s)
//	v.Name == "example"
//	v.ID == "exampleID"
func parseImportCode(code string) (struct {
	Name string
	ID   string
}, bool,
) {
	type ret struct {
		Name string
		ID   string
	}
	if importCodePattern.MatchString(code) {
		matches := importCodePattern.FindStringSubmatch(code)
		return ret{Name: matches[2], ID: matches[3]}, true
	}
	return ret{}, false
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
		p.sink.warn("", "Expected to pair --- begin/end for resource %v's Markdown header", p.rawname)
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
		p.sink.warn("Expected an H1 in markdown for resource %v", p.rawname)
	}
}

// isBlank returns true if the line is all whitespace.
func isBlank(line string) bool {
	return strings.TrimSpace(line) == ""
}

// reformatSubsection strips any "Open in Cloud Shell" buttons from the subsection and detects the presence of example
// code snippets.
func (p *tfMarkdownParser) reformatSubsection(lines []string) ([]string, bool) {
	var result []string
	isEmpty := true

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
				if (strings.Index(line, "```") != 0) && !isBlank(line) {
					isEmpty = false
				}
				result = append(result, line)
			}
		}
	}

	return result, isEmpty
}

// convertExamples converts any code snippets in a subsection to Pulumi-compatible code. This conversion is done on a
// per-subsection basis; subsections with failing examples will be elided upon the caller's request.
func (g *Generator) convertExamples(docs string, path examplePath) string {
	if docs == "" {
		return ""
	}

	if strings.Contains(docs, "{{% examples %}}") {
		// The provider author has explicitly written an entire markdown document including examples.
		// We'll just return it as is.
		return docs
	}

	if strings.Contains(docs, "```typescript") || strings.Contains(docs, "```python") ||
		strings.Contains(docs, "```go") || strings.Contains(docs, "```yaml") ||
		strings.Contains(docs, "```csharp") || strings.Contains(docs, "```java") {
		// we have explicitly rewritten these examples and need to just return them directly rather than trying
		// to reconvert them.
		//
		//TODO: This only works if the incoming docs already have an {{% example }} shortcode, and if they are
		// in an "Example Usage" section.
		// The shortcode should be replaced with the new HTML comment, either in the incoming docs, or here to avoid
		// breaking users.

		// We need to surround the examples in the examples shortcode for rendering on the registry

		// Find the index of "## Example Usage"
		exampleIndex := strings.Index(docs, "## Example Usage")

		// if not found surround all content
		if exampleIndex == -1 {
			return fmt.Sprintf("{{%% examples %%}}\n%s\n{{%% /examples %%}}", docs)
		}

		// Separate resource description and surround the examples
		return fmt.Sprintf("%s\n\n{{%% examples %%}}\n%s\n{{%% /examples %%}}",
			strings.TrimRightFunc(docs[:exampleIndex], unicode.IsSpace),
			docs[exampleIndex:])
	}
	if cliConverterEnabled() {
		return g.cliConverter().StartConvertingExamples(docs, path)
	}

	// Use coverage tracker: on by default.
	cov := true
	return g.convertExamplesInner(docs, path, g.convertHCL, cov)
}

// codeBlock represents a code block found in the upstream docs, delineated by code fences (```).
// It also tracks which header it is part of.
type codeBlock struct {
	start       int    // The index of the first backtick of an opening code fence
	end         int    // The index of the first backtick of a closing code fence
	headerStart int    // The index of the first "#" in a Markdown header. A value of -1 indicates there's no header.
	language    string // The language of the code block.
}

// A string representing the code inside a code block.
//
// Given the code block:
//
//	```sh
//	$ cmd \
//	  --flag
//
//	```
//
// This method would return "$ cmd \\\n  --flag\n".
//
// The returned string represents a view into the passed in byte slice, and does not
// remove any padding found in the original document.
func (cb codeBlock) code(document []byte) string {
	nextNewLine := bytes.IndexRune(document[cb.start:cb.end], '\n')
	return string(document[cb.start+nextNewLine+1 : cb.end])
}

func findCodeBlocks(docs []byte) []codeBlock {
	rootNode := goldmark.New(goldmark.WithExtensions(parse.TFRegistryExtension)).
		Parser().Parse(gmtext.NewReader(docs))

	var codeBlocks []codeBlock
	parse.WalkNode(rootNode, func(cb *gmast.FencedCodeBlock) {
		lines := cb.Lines()

		// Skip empty code blocks to avoid panic
		if lines.Len() == 0 {
			return
		}

		headerStart := -1
		for p := cb.Parent(); p != nil; p = p.Parent() {
			if s, ok := p.(*section.Section); ok {
				l := s.FirstChild().Lines()
				if l.Len() == 0 {
					// A header doesn't have any lines if there is no text associated with the
					// header, then we can't find its location due to limitations of goldmark.
					//
					// Just give up on finding a header here.
					break
				}
				headerStart = bytes.LastIndexByte(docs[:l.At(0).Start], '\n') + 1
				break
			}
		}

		firstNewlineOfCodeBlock := bytes.LastIndexByte(docs[:lines.At(0).Start], '\n')
		firstNewlineOfCodeFence := bytes.LastIndexByte(docs[:firstNewlineOfCodeBlock], '\n')
		if firstNewlineOfCodeFence == -1 {
			// This means that docs starts with a code block
			firstNewlineOfCodeFence = 0
		}
		firstBacktickOfCodeFence := bytes.IndexByte(docs[firstNewlineOfCodeFence:], '`') + firstNewlineOfCodeFence

		codeBlocks = append(codeBlocks, codeBlock{
			start:       firstBacktickOfCodeFence,
			end:         lines.At(lines.Len() - 1).Stop,
			headerStart: headerStart,
			language:    string(cb.Language(docs)),
		})
	})
	return codeBlocks
}

// The inner implementation of examples conversion is parameterized by convertHCL so that it can be
// executed either normally or in symbolic mode.
func (g *Generator) convertExamplesInner(
	docs string,
	path examplePath,
	convertHCL func(
		e *Example, hcl, path string, languages []string,
	) (string, error),
	useCoverageTracker bool,
) string {
	output := &bytes.Buffer{}
	fprintf := func(f string, args ...interface{}) {
		_, err := fmt.Fprintf(output, f, args...)
		contract.AssertNoErrorf(err, "Cannot fail to write out output buffer")
	}
	const codeFence = "```"

	// Traverse the code blocks and take appropriate action before appending to output
	textStart := 0
	stripSection := false
	stripSectionHeader := 0 // The index of the header that we might want to strip.
	for _, tfBlock := range findCodeBlocks([]byte(docs)) {
		// if the section has a header we append the header after trying to convert the code.
		hasHeader := tfBlock.headerStart >= 0 && textStart < tfBlock.headerStart

		// append non-code text to output
		if !stripSection {
			end := tfBlock.start
			if hasHeader {
				end = tfBlock.headerStart
			}
			fprintf("%s", docs[textStart:end])
		} else {
			// if we are stripping this section and still have the same header, we append nothing and skip to the next
			// code block.
			if stripSectionHeader == tfBlock.headerStart {
				if eol := strings.IndexRune(docs[tfBlock.end:], '\n'); eol > -1 {
					textStart = tfBlock.end + eol
				} else {
					// If no newline character is found, we are at the end of the doc.
					textStart = len(docs)
				}
				continue
			}
			if stripSectionHeader < tfBlock.headerStart {
				stripSection = false
			}
		}
		// Only attempt to convert code blocks that are either explicitly marked as Terraform, or
		// unmarked. For unmarked snippets further gate by a regex guess if it is actually Terraform.
		if hcl := tfBlock.code([]byte(docs)); isHCL(tfBlock.language, hcl) {
			// generate the code block and append
			if g.language.shouldConvertExamples() {
				// Most of our results should be HCL, so we try to convert it.
				var e *Example
				if useCoverageTracker {
					e = g.coverageTracker.getOrCreateExample(
						path.String(), hcl)
				}
				langs := genLanguageToSlice(g.language)
				convertedBlock, err := convertHCL(e, hcl, path.String(), langs)
				if err != nil {
					// We do not write this section, ever.
					//
					// We have to strip the entire section: any header, the code
					// block, and any surrounding text.
					stripSection = true
					stripSectionHeader = tfBlock.headerStart
				} else {
					// append any headers and following text first
					if hasHeader {
						fprintf("%s", docs[tfBlock.headerStart:tfBlock.start])
					}

					switch g.language {
					// If we are targeting the schema, then print code switcher
					// fences for the registry.
					case Schema:
						fprintf("%s\n%s\n%s",
							startPulumiCodeChooser,
							convertedBlock,
							endPulumiCodeChooser)
					// Otherwise skip code switcher fences so they don't show up
					// in generated SDKs.
					default:
						fprintf("%s", convertedBlock)
					}
				}
			}
		} else {
			// Take already-valid code blocks as-is.
			if hasHeader {
				fprintf("%s", docs[tfBlock.headerStart:tfBlock.start])
			}
			fprintf("%s"+codeFence, docs[tfBlock.start:tfBlock.end])
		}

		// We want to start including non-code text after the end of the code block.
		//
		// The codeblock "ends" with the newline character at the end of the
		// closing fence.
		if eol := strings.IndexRune(docs[tfBlock.end:], '\n'); eol > -1 {
			textStart = tfBlock.end + eol
		} else {
			// If no newline character is found, we are at the end of the doc.
			textStart = len(docs)
		}
	}
	// Append any remainder of the docs string to the output
	if !stripSection {
		fprintf("%s", docs[textStart:])
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
	err := fmt.Errorf("panic converting HCL: %s", panicArg)
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
func (g *Generator) convert(
	e *Example, input afero.Fs, languageName string,
) (files map[string][]byte, diags convert.Diagnostics, err error) {
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		files = map[string][]byte{}
		diags = convert.Diagnostics{}
		trace := string(debug.Stack())
		err = newConversionError(v, trace)
		g.coverageTracker.languageConversionPanic(e, languageName, fmt.Sprintf("%v", v))
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
	})

	return files, diags, err
}

func (g *Generator) legacyConvert(
	e *Example, hclCode, fileName, languageName string,
) (string, hcl.Diagnostics, error) {
	input := afero.NewMemMapFs()
	f, err := input.Create(fileName)
	contract.AssertNoErrorf(err, "err != nil")
	_, err = f.Write([]byte(hclCode))
	contract.AssertNoErrorf(err, "err != nil")
	contract.IgnoreClose(f)

	files, diags, err := g.convert(e, input, languageName)
	if diags.All.HasErrors() || err != nil {
		return "", diags.All, err
	}

	contract.Assertf(len(files) == 1, `len(files) == 1`)

	convertedHcl := ""
	for _, output := range files {
		convertedHcl = strings.TrimSpace(string(output))
	}
	return convertedHcl, diags.All, nil
}

// convertHCLToString hides the implementation details of the upstream implementation for HCL conversion and provides
// simplified parameters and return values
func (g *Generator) convertHCLToString(e *Example, hclCode, path, languageName string) (string, error) {
	fileName := fmt.Sprintf("/%s.tf", strings.ReplaceAll(path, "/", "-"))

	failure := func(diags hcl.Diagnostics) error {
		// Remove the temp filename from the error, since it will be confusing to users of the bridge who do not know
		// we write an example to a temp file internally in order to pass to convert.Convert().
		//
		// fileName starts with a "/" which is not present in the resulting error, so we need to skip the first rune.
		errMsg := strings.ReplaceAll(diags.Error(), fileName[1:], "")

		g.coverageTracker.languageConversionFailure(e, languageName, diags)
		return errors.New(errMsg)
	}

	cache := g.getOrCreateExamplesCache()
	if convertedHcl, ok := cache.Lookup(hclCode, languageName); ok {
		g.coverageTracker.languageConversionSuccess(e, languageName, convertedHcl)
		return convertedHcl, nil
	}

	var convertedHcl string
	var diags hcl.Diagnostics
	var err error

	if cliConverterEnabled() {
		// The cliConverter has a slightly different error behavior as it can return both
		// err and diags but does not panic. Handle this by re-coding err as a diag and
		// proceeding to handle diags normally.
		convertedHcl, diags, err = g.cliConverter().Convert(hclCode, languageName)
		if err != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  err.Error(),
			})
		}
		if diags.HasErrors() {
			return "", failure(diags)
		}
	} else {
		convertedHcl, diags, err = g.legacyConvert(e, hclCode, fileName, languageName)
	}

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
	if diags.HasErrors() {
		return "", failure(diags)
	}

	cache.Store(hclCode, languageName, convertedHcl)
	g.coverageTracker.languageConversionSuccess(e, languageName, convertedHcl)
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
	}
	// Fall back to alphabetical if neither are found:
	return s[i] < s[j]
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
func (g *Generator) convertHCL(e *Example, hcl, path string, languages []string) (string, error) {
	g.debug("converting HCL for %s", path)

	// Fixup the HCL as necessary.
	if fixed, ok := fixHcl(hcl); ok {
		hcl = fixed
	}

	hclConversions := map[string]string{}
	var result strings.Builder

	failedLangs := map[string]error{}

	for _, lang := range languages {
		var convertErr error
		hclConversions[lang], convertErr = g.convertHCLToString(e, hcl, path, lang)
		if convertErr != nil {
			failedLangs[lang] = convertErr
		}
	}

	result.WriteString(hclConversionsToString(hclConversions))

	switch {
	// Success
	case len(failedLangs) == 0:
		return result.String(), nil
	// Complete failure - every language conversion failed; error
	case len(failedLangs) == len(languages):
		err := g.warnUnableToConvertHCLExample(path, failedLangs)
		return "", err
	// Partial failure - not returning an error but still emit the warning
	default:
		err := g.warnUnableToConvertHCLExample(path, failedLangs)
		contract.IgnoreError(err)
		return result.String(), nil
	}
}

func (g *Generator) warnUnableToConvertHCLExample(path string, failedLangs map[string]error) error {
	// Index sets of languages by error message to avoid emitting similar errors for each language.
	languagesByErrMsg := map[string]map[string]struct{}{}
	for lang, convertErr := range failedLangs {
		errMsg := convertErr.Error()
		if _, ok := languagesByErrMsg[errMsg]; !ok {
			languagesByErrMsg[errMsg] = map[string]struct{}{}
		}
		languagesByErrMsg[errMsg][lang] = struct{}{}
	}

	var err error

	seen := map[string]struct{}{}
	for _, convertErr := range failedLangs {
		if _, dup := seen[convertErr.Error()]; dup {
			continue
		}
		errMsg := convertErr.Error()
		seen[errMsg] = struct{}{}

		langs := []string{}
		for l := range languagesByErrMsg[errMsg] {
			langs = append(langs, l)
		}
		sort.Strings(langs)
		ls := strings.Join(langs, ", ") // all languages that have this error
		err = multierror.Append(err, fmt.Errorf("[%s] %w", ls, convertErr))
	}

	g.warn("unable to convert HCL example for Pulumi entity '%s'. The example will be dropped "+
		"from any generated docs or SDKs: %v", path, err)

	return err
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
	case Java:
		return []string{convert.LanguageJava}
	case PCL:
		return []string{convert.LanguagePulumi}
	case Schema, RegistryDocs:
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

func cleanupDoc(
	name string, g diagsSink, infoCtx infoContext, doc entityDocs,
	footerLinks map[string]string,
) (entityDocs, bool) {
	newargs := make(map[docsPath]*argumentDocs, len(doc.Arguments))

	for k, v := range doc.Arguments {
		if k.nested() {
			g.debug("Cleaning up text for nested argument [%v] in [%v]", k, name)
		} else {
			g.debug("Cleaning up text for argument [%v] in [%v]", k, name)
		}
		cleanedText := reformatText(infoCtx, v.description, footerLinks)

		newargs[k] = &argumentDocs{description: cleanedText}
	}

	newattrs := make(map[string]string, len(doc.Attributes))
	for k, v := range doc.Attributes {
		g.debug("Cleaning up text for attribute [%v] in [%v]", k, name)
		cleanedText := reformatText(infoCtx, v, footerLinks)
		newattrs[k] = cleanedText
	}

	g.debug("Cleaning up description text for [%v]", name)
	cleanupText := reformatText(infoCtx, doc.Description, footerLinks)

	importText := doc.Import
	if importText != "" {
		importText = reformatImportText(infoCtx, importText, footerLinks)
	}

	return entityDocs{
		Description: cleanupText,
		Arguments:   newargs,
		Attributes:  newattrs,
		Import:      importText,
	}, false
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

type infoContext struct {
	language Language
	pkg      tokens.Package
	info     tfbridge.ProviderInfo
}

type spanValues struct {
	node, dotnet, golang, python, yaml, java, defaultDisplay string
}

func buildSpan(values spanValues) string {
	//nolint:lll
	spanFormat := `<span pulumi-lang-nodejs="%s" pulumi-lang-dotnet="%s" pulumi-lang-go="%s" pulumi-lang-python="%s" pulumi-lang-yaml="%s" pulumi-lang-java="%s">%s</span>`
	return fmt.Sprintf(
		spanFormat,
		values.node,
		values.dotnet,
		values.golang,
		values.python,
		values.yaml,
		values.java,
		values.defaultDisplay)
}

func (c infoContext) fixupPropertyReference(text string) string {
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

		if resInfo, hasResourceInfo := c.info.Resources[name]; hasResourceInfo {
			// This is a resource name
			resname, mod := resourceName(c.info.GetResourcePrefix(), name, resInfo, false)
			modname := formatModulePrefix(parentModuleName(mod))

			// Use `ec2.Instance` format for Go and Python
			goAndPyFormat := open + modname + resname.String() + close
			// Use `aws.ec2.Instance` format for all other languages
			allOtherLangs := open + c.pkg.String() + "." + modname + resname.String() + close

			// We use the NodeJS default for registry docs.
			if c.language == RegistryDocs {
				return allOtherLangs
			}
			return buildSpan(spanValues{
				node:           allOtherLangs,
				dotnet:         allOtherLangs,
				golang:         goAndPyFormat,
				python:         goAndPyFormat,
				yaml:           allOtherLangs,
				java:           allOtherLangs,
				defaultDisplay: allOtherLangs,
			})
		} else if dataInfo, hasDatasourceInfo := c.info.DataSources[name]; hasDatasourceInfo {
			// This is a data source name
			getname, mod := dataSourceName(c.info.GetResourcePrefix(), name, dataInfo)
			modname := formatModulePrefix(parentModuleName(mod))

			goFormat := open + modname + getname.String() + close
			pyFormat := open + python.PyName(modname+getname.String()) + close
			// Use `aws.ec2.Instance` format
			allOtherLangs := open + c.pkg.String() + "." + modname + getname.String() + close
			if c.language == RegistryDocs {
				return allOtherLangs
			}
			return buildSpan(spanValues{
				node:           allOtherLangs,
				dotnet:         allOtherLangs,
				golang:         goFormat,
				python:         pyFormat,
				yaml:           allOtherLangs,
				java:           allOtherLangs,
				defaultDisplay: allOtherLangs,
			})
		}
		// Else just treat as a property name
		pname := propertyName(name, nil, nil)
		camelCaseFormat := open + pname + close

		// Capitalize dotnet properties
		firstLetter := string(pname[0])
		dotnetFormat := open + strings.ToUpper(firstLetter) + pname[1:] + close

		if c.language == RegistryDocs {
			return camelCaseFormat
		}
		return buildSpan(spanValues{
			node:           camelCaseFormat,
			dotnet:         dotnetFormat,
			golang:         camelCaseFormat,
			python:         match,
			yaml:           camelCaseFormat,
			java:           camelCaseFormat,
			defaultDisplay: match,
		})
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

	return strings.ReplaceAll(description, parts[0], "")
}

// reformatText processes markdown strings from TF docs and cleans them for inclusion in Pulumi docs
func reformatText(g infoContext, text string, footerLinks map[string]string) string {
	return reformatTextWithOptions(g, text, footerLinks, true)
}

func reformatImportText(g infoContext, text string, footerLinks map[string]string) string {
	clean := reformatTextWithOptions(g, text, footerLinks, false)
	if clean != "" {
		if strings.HasSuffix(clean, "\n\n") {
			return clean
		}
		if strings.HasSuffix(clean, "\n") {
			return clean + "\n"
		}
		return clean + "\n\n"
	}
	return clean
}

func reformatTextWithOptions(
	g infoContext, text string, footerLinks map[string]string, trimSpace bool,
) string {
	cleanupText := func(text string) string {
		// Replace occurrences of "->" or "~>" with just ">", to get a proper Markdown note.
		text = strings.ReplaceAll(text, "-> ", "> ")
		text = strings.ReplaceAll(text, "~> ", "> ")

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

		// Fixup resource and property name references.
		text = g.fixupPropertyReference(text)

		return text
	}

	// Detect all code blocks in the text so we can avoid processing them.
	codeBlocks := codeBlocks.FindAllStringIndex(text, -1)

	var parts []string
	var end int
	start := 0
	for _, codeBlock := range codeBlocks {
		end = codeBlock[0]

		parts = append(parts, cleanupText(text[start:end]))

		start = codeBlock[1]
		parts = append(parts, text[end:start])
	}
	if start != len(text) {
		parts = append(parts, cleanupText(text[start:]))
	}

	result := strings.Join(parts, "")
	if trimSpace {
		result = strings.TrimSpace(result)
	} else {
		result = strings.TrimLeftFunc(result, unicode.IsSpace)
	}
	return result
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

var guessIsHCLPattern = regexp.MustCompile(`(resource|data)\s+["][^"]+["]\s+["][^"]+["]\s+[{]`)

func guessIsHCL(code string) bool {
	return guessIsHCLPattern.MatchString(code)
}

func isHCL(fenceLanguage, code string) bool {
	switch fenceLanguage {
	case "terraform", "hcl", "tf":
		return true
	case "":
		return guessIsHCL(code)
	default:
		return false
	}
}
