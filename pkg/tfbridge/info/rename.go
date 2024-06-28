// Copyright 2016-2024, Pulumi Corporation.
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

package info

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

const RenamedEntitySuffix string = "_legacy"

func (p *Provider) RenameResourceWithAlias(resourceName string, legacyTok tokens.Type, newTok tokens.Type,
	legacyModule string, newModule string, info *Resource,
) {
	resourcePrefix := p.Name + "_"
	legacyResourceName := resourceName + RenamedEntitySuffix
	if info == nil {
		info = &Resource{}
	}
	legacyInfo := *info
	currentInfo := *info

	legacyInfo.Tok = legacyTok
	legacyType := legacyInfo.Tok.String()

	if newTok != "" {
		legacyTok = newTok
	}

	currentInfo.Tok = legacyTok
	currentInfo.Aliases = []Alias{
		{Type: &legacyType},
	}

	if legacyInfo.Docs == nil {
		legacyInfo.Docs = &Doc{
			Source: resourceName[len(resourcePrefix):] + ".html.markdown",
		}
	}

	legacyInfo.DeprecationMessage = fmt.Sprintf("%s has been deprecated in favor of %s",
		generateResourceName(legacyInfo.Tok.Module().Package(), strings.ToLower(legacyModule),
			legacyInfo.Tok.Name().String()),
		generateResourceName(currentInfo.Tok.Module().Package(), strings.ToLower(newModule),
			currentInfo.Tok.Name().String()))
	p.Resources[resourceName] = &currentInfo
	p.Resources[legacyResourceName] = &legacyInfo
	err := shim.CloneResource(p.P.ResourcesMap(), resourceName, legacyResourceName)
	contract.AssertNoErrorf(err, "Failed to rename the resource")
}

func (p *Provider) RenameDataSource(resourceName string, legacyTok tokens.ModuleMember, newTok tokens.ModuleMember,
	legacyModule string, newModule string, info *DataSource,
) {
	resourcePrefix := p.Name + "_"
	legacyResourceName := resourceName + RenamedEntitySuffix
	if info == nil {
		info = &DataSource{}
	}
	legacyInfo := *info
	currentInfo := *info

	legacyInfo.Tok = legacyTok

	if newTok != "" {
		legacyTok = newTok
	}

	currentInfo.Tok = legacyTok

	if legacyInfo.Docs == nil {
		legacyInfo.Docs = &Doc{
			Source: resourceName[len(resourcePrefix):] + ".html.markdown",
		}
	}

	legacyInfo.DeprecationMessage = fmt.Sprintf("%s has been deprecated in favor of %s",
		generateResourceName(legacyInfo.Tok.Module().Package(), strings.ToLower(legacyModule),
			legacyInfo.Tok.Name().String()),
		generateResourceName(currentInfo.Tok.Module().Package(), strings.ToLower(newModule),
			currentInfo.Tok.Name().String()))
	p.DataSources[resourceName] = &currentInfo
	p.DataSources[legacyResourceName] = &legacyInfo
	p.P.DataSourcesMap().Set(legacyResourceName, p.P.DataSourcesMap().Get(resourceName))
}

func generateResourceName(packageName tokens.Package, moduleName string, moduleMemberName string) string {
	// We don't want DeprecationMessages that read
	// `postgresql.index.DefaultPrivileg` has been deprecated in favour of `postgresql.index.DefaultPrivileges`
	// we would never use `index` in a reference to the Class. So we should remove this where needed
	if moduleName == "" || moduleName == "index" {
		return fmt.Sprintf("%s.%s", packageName, moduleMemberName)
	}

	return fmt.Sprintf("%s.%s.%s", packageName, moduleName, moduleMemberName)
}
