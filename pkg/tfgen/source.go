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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
)

// A source of documentation bytes.
type DocsSource interface {
	// Get the bytes for a resource with TF token rawname.
	getResource(rawname string, info *tfbridge.DocInfo) (*DocFile, error)

	// Get the bytes for a datasource with TF token rawname.
	getDatasource(rawname string, info *tfbridge.DocInfo) (*DocFile, error)
}

type DocFile struct {
	Content  []byte
	FileName string
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

type gitRepoSource struct {
	docRules              *tfbridge.DocRuleInfo
	upstreamRepoPath      string
	org                   string
	provider              string
	resourcePrefix        string
	providerModuleVersion string
	githost               string
}

func (gh *gitRepoSource) getResource(rawname string, info *tfbridge.DocInfo) (*DocFile, error) {
	return gh.getFile(rawname, info, ResourceDocs)
}

func (gh *gitRepoSource) getDatasource(rawname string, info *tfbridge.DocInfo) (*DocFile, error) {
	return gh.getFile(rawname, info, DataSourceDocs)
}

// getFile implements the private logic necessary to get a file from a TF Git repo's website section.
func (gh *gitRepoSource) getFile(
	rawname string, info *tfbridge.DocInfo, kind DocKind,
) (*DocFile, error) {
	if info != nil && len(info.Markdown) != 0 {
		return &DocFile{Content: info.Markdown}, nil
	}

	repoPath := gh.upstreamRepoPath
	if repoPath == "" {
		var err error
		repoPath, err = getRepoPath(gh.githost, gh.org, gh.provider, gh.providerModuleVersion)
		if err != nil {
			return nil, fmt.Errorf("repo for token %q: %w", rawname, err)
		}
	}

	possibleMarkdownNames := getMarkdownNames(gh.resourcePrefix, rawname, gh.docRules)

	if info != nil && info.Source != "" {
		possibleMarkdownNames = append(possibleMarkdownNames, info.Source)
	}

	return readMarkdown(repoPath, kind, possibleMarkdownNames)
}

// An error that represents a missing repo path directory.
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

var repoPaths sync.Map

func getRepoPath(gitHost string, org string, provider string, version string) (_ string, err error) {
	moduleCoordinates := fmt.Sprintf("%s/%s/terraform-provider-%s", gitHost, org, provider)
	if version != "" {
		moduleCoordinates = fmt.Sprintf("%s/%s", moduleCoordinates, version)
	}

	defer func() {
		if err == nil {
			return
		}
		err = GetRepoPathErr{
			Expected:   moduleCoordinates,
			Underlying: err,
		}
	}()

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

func getMarkdownNames(packagePrefix, rawName string, globalInfo *tfbridge.DocRuleInfo) []string {
	possibleMarkdownNames := []string{
		// Most frequently, docs leave off the provider prefix
		withoutPackageName(packagePrefix, rawName) + ".html.markdown",
		withoutPackageName(packagePrefix, rawName) + ".markdown",
		withoutPackageName(packagePrefix, rawName) + ".html.md",
		withoutPackageName(packagePrefix, rawName) + ".md",
		// But for some providers, the prefix is included in the name of the doc file
		rawName + ".html.markdown",
		rawName + ".markdown",
		rawName + ".html.md",
		rawName + ".md",
	}

	if globalInfo != nil && globalInfo.AlternativeNames != nil {
		// We look at user generated names before we look at default names
		possibleMarkdownNames = append(globalInfo.AlternativeNames(tfbridge.DocsPathInfo{
			TfToken: rawName,
		}), possibleMarkdownNames...)
	}

	return possibleMarkdownNames
}

// readMarkdown searches all possible locations for the markdown content
func readMarkdown(repo string, kind DocKind, possibleLocations []string) (*DocFile, error) {
	locationPrefix := getDocsPath(repo, kind)

	for _, name := range possibleLocations {
		location := filepath.Join(locationPrefix, name)
		markdownBytes, err := os.ReadFile(location)
		if err == nil {
			return &DocFile{markdownBytes, name}, nil
		} else if !os.IsNotExist(err) && !errors.Is(err, &os.PathError{}) {
			// Missing doc files are expected and OK.
			//
			// If the file we expect is actually a directory (PathError), that
			// is also OK.
			//
			// Other errors (such as permission errors) indicate a problem
			// with the host system, and should be reported.
			return nil, fmt.Errorf("%s: %w", location, err)
		}
	}
	return nil, nil
}
