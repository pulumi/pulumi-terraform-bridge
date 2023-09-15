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
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

type DocsSource interface {
	getResource(rawname string, info tfbridge.ResourceOrDataSourceInfo) (*DocFile, error)
	getDatasource(rawname string, info tfbridge.ResourceOrDataSourceInfo) (*DocFile, error)
}

type DocFile struct {
	Content  []byte
	Filename string
}

func NewGitRepoDocsSource(g *Generator) DocsSource {
	return &gitRepoSource{
		docRules:              g.info.DocRules,
		upstreamRepoPath:      g.info.UpstreamRepoPath,
		org:                   g.info.GetGitHubOrg(),
		provider:              g.info.Name,
		resourcePrefix:        g.info.GetResourcePrefix(),
		providerModuleVersion: g.info.GetProviderModuleVersion(),
		githost:               g.info.GetGitHubHost(),
	}
}

func NewNullSource() DocsSource { return nullSource{} }

type nullSource struct{}

func (nullSource) getResource(rawname string, info tfbridge.ResourceOrDataSourceInfo) (*DocFile, error) {
	return nil, nil
}
func (nullSource) getDatasource(rawname string, info tfbridge.ResourceOrDataSourceInfo) (*DocFile, error) {
	return nil, nil
}

type gitRepoSource struct {
	docRules              *tfbridge.DocRuleInfo
	upstreamRepoPath      string
	org                   string
	provider              string // provider name?
	resourcePrefix        string
	providerModuleVersion string
	githost               string
}

func (gh *gitRepoSource) getResource(rawname string, info tfbridge.ResourceOrDataSourceInfo) (*DocFile, error) {
	return gh.getFile(rawname, info, ResourceDocs)
}

func (gh *gitRepoSource) getDatasource(rawname string, info tfbridge.ResourceOrDataSourceInfo) (*DocFile, error) {
	return gh.getFile(rawname, info, DataSourceDocs)
}

func (gh *gitRepoSource) getFile(
	rawname string, info tfbridge.ResourceOrDataSourceInfo, kind DocKind,
) (*DocFile, error) {
	var docinfo *tfbridge.DocInfo
	if info != nil {
		docinfo = info.GetDocs()
	}
	if docinfo != nil && len(docinfo.Markdown) != 0 {
		return &DocFile{Content: docinfo.Markdown}, nil
	}

	repoPath := gh.upstreamRepoPath
	if repoPath == "" {
		var err error
		repoPath, err = getRepoPath(gh.githost, gh.org, gh.provider, gh.providerModuleVersion)
		if err != nil {
			return nil, fmt.Errorf("get file for %q (%q): %w",
				rawname, kind, err)
		}
	}

	possibleMarkdownNames := getMarkdownNames(gh.resourcePrefix, rawname, gh.docRules)

	if docinfo != nil && docinfo.Source != "" {
		possibleMarkdownNames = append(possibleMarkdownNames, docinfo.Source)
	}

	return readMarkdown(repoPath, kind, possibleMarkdownNames)
}

// readMarkdown searches all possible locations for the markdown content
func readMarkdown(repo string, kind DocKind, possibleLocations []string) (*DocFile, error) {
	locationPrefix := getDocsPath(repo, kind)

	for _, name := range possibleLocations {
		location := filepath.Join(locationPrefix, name)
		markdownBytes, err := os.ReadFile(location)
		if err == nil {
			return &DocFile{markdownBytes, name}, nil
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return nil, nil
}
