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
	innerLoader schema.Loader
}

var _ schema.Loader = &loader{}

func (l *loader) LoadPackage(name string, ver *semver.Version) (*schema.Package, error) {
	// In the doc generation context a dummy package seems better
	// than failing in this case.
	if name == "" {
		return &schema.Package{}, nil
	}
	return l.innerLoader.LoadPackage(name, ver)
}

func newLoader(host plugin.Host) schema.Loader {
	return &loader{schema.NewPluginLoader(host)}
}
