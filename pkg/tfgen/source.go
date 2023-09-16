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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	provider              string
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

type GetRepoPathErr struct {
	Expected   string
	Underlying error
}

func (e GetRepoPathErr) Error() string {
	return fmt.Sprintf("Unable to access repository at %s: %s", e.Expected, e.Underlying.Error())
}

func (e GetRepoPathErr) Unwrap() error {
	return e.Underlying
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
	} else if _, err := os.Stat(repoPath); err != nil {
		return nil, GetRepoPathErr{Expected: repoPath, Underlying: err}
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

func getRepoPath(gitHost string, org string, provider string, version string) (_ string, err error) {
	moduleCoordinates := fmt.Sprintf("%s/%s/terraform-provider-%s", gitHost, org, provider)
	if version != "" {
		moduleCoordinates = fmt.Sprintf("%s/%s", moduleCoordinates, version)
	}

	if path, ok := repoPaths.Load(moduleCoordinates); ok {
		return path.(string), nil
	}

	defer func() {
		if err == nil {
			return
		}
		err = GetRepoPathErr{Expected: moduleCoordinates, Underlying: err}
	}()

	curWd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("error finding current working directory: %w", err)
	}
	if filepath.Base(curWd) != "provider" {
		provDir := filepath.Join(curWd, "provider")
		info, err := os.Stat(provDir)
		if err == nil && info.IsDir() {
			curWd = provDir
		} else if err != nil && !os.IsNotExist(err) {
			return "", err
		}

	}

	command := exec.Command("go", "mod", "download", "-json", moduleCoordinates)
	command.Dir = curWd
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error running '%s' in %q dir for module: %w\n\nOutput: %s",
			strings.Join(command.Args, " "), curWd, err, output)
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
		return "", fmt.Errorf("error from '%s' for module: %s",
			strings.Join(command.Args, " "), target.Error)
	}

	repoPaths.Store(moduleCoordinates, target.Dir)

	return target.Dir, nil
}
