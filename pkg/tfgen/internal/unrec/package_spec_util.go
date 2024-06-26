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

package unrec

import (
	"strings"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func parseLocalRef(rawRef string) (tokens.Type, bool) {
	if !strings.HasPrefix(rawRef, "#/types/") {
		return "", false
	}
	cleanRef := strings.TrimPrefix(rawRef, "#/types/")
	return tokens.Type(cleanRef), true
}

type xProperty struct {
	pschema.PropertySpec
	IsRequired bool
	IsPlain    bool
}

type xPropertyMap map[string]xProperty

func newXPropertyMap(s pschema.ObjectTypeSpec) xPropertyMap {
	m := make(xPropertyMap)
	for _, prop := range s.Plain {
		p := m[prop]
		p.IsPlain = true
		m[prop] = p
	}
	for _, prop := range s.Required {
		p := m[prop]
		p.IsRequired = true
		m[prop] = p
	}
	for prop, spec := range s.Properties {
		p := m[prop]
		p.PropertySpec = spec
		m[prop] = p
	}
	return m
}
