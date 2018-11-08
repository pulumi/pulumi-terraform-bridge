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
	"go/build"
	"os"
	"path"
)

// GitInfo contains Git information about a provider.
type GitInfo struct {
	Repo   string // the Git repo for this provider.
	Tag    string // the Git tag info for this provider.
	Commit string // the Git commit info for this provider.
}

const (
	tfGitHub         = "github.com"
	tfProvidersOrg   = "terraform-providers"
	tfProviderPrefix = "terraform-provider"
)

// getRepoDir gets the source repository for a given provider
func getRepoDir(prov string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	repo := path.Join(tfGitHub, tfProvidersOrg, tfProviderPrefix+"-"+prov)
	pkg, err := build.Import(repo, wd, build.FindOnly)
	if err != nil {
		return "", err
	}
	return pkg.Dir, nil
}
