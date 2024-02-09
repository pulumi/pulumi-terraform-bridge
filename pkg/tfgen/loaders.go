// Copyright 2016-2021, Pulumi Corporation.
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
	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

type loader struct {
	innerLoader     schema.Loader
	emptyPackages   map[string]bool
	aliasedPackages map[string]string
}

var _ schema.Loader = &loader{}

func (l *loader) aliasPackage(alias string, canonicalName string) {
	if l.aliasedPackages == nil {
		l.aliasedPackages = map[string]string{}
	}
	l.aliasedPackages[alias] = canonicalName
}

func (l *loader) LoadPackage(name string, ver *semver.Version) (*schema.Package, error) {
	if renamed, ok := l.aliasedPackages[name]; ok {
		name = renamed
	}

	if l.emptyPackages[name] {
		return &schema.Package{
			Name:    name,
			Version: ver,
		}, nil
	}
	return l.innerLoader.LoadPackage(name, ver)
}

// Overrides `schema.NewPluginLoader` to load an empty
// `*schema.Package{}` when `name=”` is requested. In the doc
// generation context a dummy package seems better than failing in the
// case.
func newLoader(host plugin.Host) *loader {
	return &loader{
		innerLoader: schema.NewPluginLoader(host),
		emptyPackages: map[string]bool{
			"": true,
		},
	}
}
