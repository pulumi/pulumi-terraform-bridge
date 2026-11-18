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
	"encoding/json"
	"fmt"
	"strings"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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

type typeRefs map[tokens.Type]struct{}

func newTypeRefs(tok ...tokens.Type) typeRefs {
	tr := make(typeRefs)
	tr.Add(tok...)
	return tr
}

func (refs typeRefs) Contains(t tokens.Type) bool {
	_, ok := refs[t]
	return ok
}

func (refs typeRefs) Add(tok ...tokens.Type) {
	for _, t := range tok {
		refs[t] = struct{}{}
	}
}

func (refs typeRefs) Slice() []tokens.Type {
	x := []tokens.Type{}
	for k := range refs {
		x = append(x, k)
	}
	return x
}

func (refs typeRefs) Best() tokens.Type {
	var best tokens.Type
	for k := range refs {
		if best == "" || len(k) < len(best) {
			best = k
		}
	}
	contract.Assertf(best != "", "expected to find at least one element")
	return best
}

func findGenericTypeReferences(x any) (typeRefs, error) {
	bytes, err := json.Marshal(x)
	if err != nil {
		return nil, err
	}

	var s any
	if err := json.Unmarshal(bytes, &s); err != nil {
		return nil, err
	}
	tr := make(typeRefs, 0)
	transformMaps(func(m map[string]any) {
		ref, ok := detectRef(m)
		if ok {
			tr[ref] = struct{}{}
		}
	}, s)
	return tr, nil
}

func detectRef(m map[string]any) (tokens.Type, bool) {
	ref, gotRef := m["$ref"]
	if !gotRef {
		return "", false
	}
	refs, ok := ref.(string)
	if !ok {
		return "", false
	}
	return parseLocalRef(refs)
}

func transformMaps(transform func(map[string]any), value any) any {
	switch value := value.(type) {
	case map[string]any:
		transform(value)
		for k := range value {
			value[k] = transformMaps(transform, value[k])
		}
		return value
	case []any:
		for i := range value {
			value[i] = transformMaps(transform, value[i])
		}
		return value
	default:
		return value
	}
}

func rewriteTypeRefs(rewrites map[tokens.Type]tokens.Type, schema *pschema.PackageSpec) error {
	bytes, err := json.Marshal(schema)
	if err != nil {
		return err
	}

	var s any
	if err := json.Unmarshal(bytes, &s); err != nil {
		return err
	}

	rewrite := func(m map[string]any) {
		cleanRef, ok := detectRef(m)
		if !ok {
			return
		}
		if modifiedRef, ok := rewrites[cleanRef]; ok {
			m["$ref"] = fmt.Sprintf("#/types/%s", modifiedRef)
		}
	}

	modifiedS := transformMaps(rewrite, s)

	modifiedBytes, err := json.Marshal(modifiedS)
	if err != nil {
		return err
	}

	var modifiedSchema pschema.PackageSpec
	if err := json.Unmarshal(modifiedBytes, &modifiedSchema); err != nil {
		return err
	}

	for deletedType := range rewrites {
		delete(modifiedSchema.Types, string(deletedType))
	}

	*schema = modifiedSchema

	return nil
}

func findResourceTypeReferences(r pschema.ResourceSpec) (typeRefs, error) {
	return findGenericTypeReferences(r)
}

func findFunctionTypeReferences(f pschema.FunctionSpec) (typeRefs, error) {
	return findGenericTypeReferences(f)
}

func findTypeReferenceTransitiveClosure(spec *pschema.PackageSpec, refs typeRefs) (typeRefs, error) {
	queue := []tokens.Type{}
	for r := range refs {
		queue = append(queue, r)
	}
	seen := typeRefs{}
	for len(queue) > 0 {
		r := queue[0]
		queue = queue[1:]
		if _, ok := seen[r]; ok {
			continue
		}
		t, ok := spec.Types[string(r)]
		if ok {
			moreRefs, err := findGenericTypeReferences(t)
			if err != nil {
				return nil, err
			}
			for m := range moreRefs {
				queue = append(queue, m)
			}
		}
		seen[r] = struct{}{}
	}
	return seen, nil
}
