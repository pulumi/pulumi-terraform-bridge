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
	"fmt"
	"io/ioutil"
	"path"
	"regexp"
	"strings"

	"github.com/pulumi/pulumi-terraform/pkg/tfbridge"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

// parsedDoc represents the data parsed from TF markdown documentation
type parsedDoc struct {
	// Description is the description of the resource
	Description string
	// Arguments includes the names and descriptions for each argument of the resource
	Arguments map[string]string
	// Attributes includes the names and descriptions for each attribute of the resource
	Attributes map[string]string
	// URL is the source documentation URL page at www.terraform.io
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

// getDocsForProvider extracts documentation details for the given package from
// TF website documentation markdown content
func getDocsForProvider(provider string, kind DocKind, rawname string, docinfo *tfbridge.DocInfo) (parsedDoc, error) {
	repo, err := getRepoDir(provider)
	if err != nil {
		return parsedDoc{}, err
	}
	possibleMarkdownNames := []string{
		withoutPackageName(provider, rawname) + ".html.markdown",
		withoutPackageName(provider, rawname) + ".markdown",
		withoutPackageName(provider, rawname) + ".html.md",
	}
	if docinfo != nil && docinfo.Source != "" {
		possibleMarkdownNames = append(possibleMarkdownNames, docinfo.Source)
	}
	markdownByts, err := readMarkdown(repo, kind, possibleMarkdownNames)
	if err != nil {
		cmdutil.Diag().Warningf(
			diag.Message("", "Could not find docs for resource %v; consider overriding doc source location"), rawname)
		return parsedDoc{}, nil
	}
	doc := parseTFMarkdown(kind, string(markdownByts), provider, rawname)
	if docinfo != nil {
		// Merge Attributes from source into target
		if err := mergeDocs(provider, kind, doc.Attributes, docinfo.IncludeAttributesFrom,
			func(s parsedDoc) map[string]string {
				return s.Attributes
			},
		); err != nil {
			return doc, err
		}
		// Merge Arguments from source into Attributes of target
		if err := mergeDocs(provider, kind, doc.Attributes, docinfo.IncludeAttributesFromArguments,
			func(s parsedDoc) map[string]string {
				return s.Arguments
			},
		); err != nil {
			return doc, err
		}
		// Merge Arguments from source into target
		if err := mergeDocs(provider, kind, doc.Arguments, docinfo.IncludeArgumentsFrom,
			func(s parsedDoc) map[string]string {
				return s.Arguments
			},
		); err != nil {
			return doc, err
		}
	}
	return doc, nil
}

// readMarkdown searches all possible locations for the markdown content
func readMarkdown(repo string, kind DocKind, possibleLocations []string) ([]byte, error) {
	var markdownBytes []byte
	var err error
	for _, name := range possibleLocations {
		location := path.Join(repo, "website", "docs", string(kind), name)
		markdownBytes, err = ioutil.ReadFile(location)
		if err == nil {
			return markdownBytes, nil
		}
	}
	return nil, fmt.Errorf("Could not find markdown in any of: %v", possibleLocations)
}

// mergeDocs adds the docs specified by extractDoc from sourceFrom into the targetDocs
func mergeDocs(provider string, kind DocKind, targetDocs map[string]string, sourceFrom string,
	extractDocs func(d parsedDoc) map[string]string) error {

	if sourceFrom != "" {
		sourceDocs, err := getDocsForProvider(provider, kind, sourceFrom, nil)
		if err != nil {
			return err
		}
		for k, v := range extractDocs(sourceDocs) {
			targetDocs[k] = v
		}
	}
	return nil
}

var argumentBulletRegexp = regexp.MustCompile(
	"\\*\\s+`([a-zA-z0-9_]*)`\\s+(\\([a-zA-Z]*\\)\\s*)?[–-]?\\s+(\\([^\\)]*\\)\\s*)?(.*)",
)
var argumentBlockRegexp = regexp.MustCompile(
	"`([a-z_]+)`\\s+block[\\s\\w]*:",
)
var attributeBulletRegexp = regexp.MustCompile(
	"\\*\\s+`([a-zA-z0-9_]*)`\\s+[–-]?\\s+(.*)",
)
var terraformDocsTemplate = "https://www.terraform.io/docs/providers/%s/%s/%s.html"

// parseTFMarkdown takes a TF website markdown doc and extracts a structured representation for use in
// generating doc comments
func parseTFMarkdown(kind DocKind, markdown string, provider string, rawname string) parsedDoc {
	var ret parsedDoc
	ret.Arguments = map[string]string{}
	ret.Attributes = map[string]string{}
	ret.URL = fmt.Sprintf(terraformDocsTemplate, provider, kind, withoutPackageName(provider, rawname))
	sections := strings.Split(markdown, "\n## ")
	for _, section := range sections {
		lines := strings.Split(section, "\n")
		if len(lines) == 0 {
			cmdutil.Diag().Warningf(
				diag.Message("", "Unparseable doc section for  %v; consider overriding doc source location"), rawname)
		}
		switch lines[0] {
		case "Arguments Reference", "Argument Reference", "Nested Blocks", "Nested blocks":
			lastMatch := ""
			for _, line := range lines {
				matches := argumentBulletRegexp.FindStringSubmatch(line)
				blockMatches := argumentBlockRegexp.FindStringSubmatch(line)
				if len(matches) >= 4 {
					// found a property bullet, extract the name and description
					ret.Arguments[matches[1]] = matches[4]
					lastMatch = matches[1]
				} else if strings.TrimSpace(line) != "" && lastMatch != "" {
					// this is a continuation of the previous bullet
					ret.Arguments[lastMatch] += "\n" + strings.TrimSpace(line)
				} else if len(blockMatches) >= 2 {
					// found a block match, once we've found one of these the main attribute section is finished so exit the loop.
					// May require changing to get docs for named nested types as part of #163.
					break
				} else {
					// This is an empty line or there were no bullets yet - clear the lastMatch
					lastMatch = ""
				}
			}
		case "Attributes Reference", "Attribute Reference":
			lastMatch := ""
			for _, line := range lines {
				matches := attributeBulletRegexp.FindStringSubmatch(line)
				if len(matches) >= 2 {
					// found a property bullet, extract the name and description
					ret.Attributes[matches[1]] = matches[2]
					lastMatch = matches[1]
				} else if strings.TrimSpace(line) != "" && lastMatch != "" {
					// this is a continuation of the previous bullet
					ret.Attributes[lastMatch] += "\n" + strings.TrimSpace(line)
				} else {
					// This is an empty line or there were no bullets yet - clear the lastMatch
					lastMatch = ""
				}
			}
		case "---":
			// Extract the description section
			subparts := strings.Split(section, "\n# ")
			if len(subparts) != 2 {
				cmdutil.Diag().Warningf(
					diag.Message("", "Expected only a single H1 in markdown for resource %v"), rawname)
			}
			sublines := strings.Split(subparts[1], "\n")
			ret.Description += strings.Join(sublines[2:], "\n")
		case "Remarks":
			// Append the remarks to the description section
			ret.Description += strings.Join(lines[2:], "\n")
		default:
			// Ignore everything else - most commonly examples and imports with unpredictable section headers.
		}
	}
	return cleanupDoc(ret)
}

func cleanupDoc(doc parsedDoc) parsedDoc {
	newargs := make(map[string]string, len(doc.Arguments))
	for k, v := range doc.Arguments {
		newargs[k] = cleanupText(v)
	}
	newattrs := make(map[string]string, len(doc.Attributes))
	for k, v := range doc.Attributes {
		newattrs[k] = cleanupText(v)
	}
	return parsedDoc{
		Description: cleanupText(doc.Description),
		Arguments:   newargs,
		Attributes:  newattrs,
		URL:         doc.URL,
	}
}

var markdownLink = regexp.MustCompile(
	`\[([^\]]*)\]\(([^\)]*)\)`,
)

// cleanupText processes markdown strings from TF docs and cleans them for inclusion in Pulumi docs
func cleanupText(text string) string {
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
	return text
}
