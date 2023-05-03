// Copyright 2016-2023, Pulumi Corporation.
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

package x

import (
	"context"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/walk"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// Ensures resource.MakeSecret is used to wrap any nested values that correspond to secret properties in the schema. A
// property is considered secret if it is declared Sensitive in the SchemaMap in the upstream provider. Users may also
// override a matching SchemaInfo.Secret setting to force a property to be considered secret or non-secret.
func MarkSchemaSecrets(ctx context.Context, schemaMap shim.SchemaMap, configInfos map[string]*tfbridge.SchemaInfo,
	pv resource.PropertyValue) resource.PropertyValue {
	ss := &schemaSecrets{schemaMap, configInfos}
	return ss.markSchemaSecretsTransform(make(resource.PropertyPath, 0), pv)
}

type schemaSecrets struct {
	schemaMap   shim.SchemaMap
	schemaInfos map[string]*tfbridge.SchemaInfo
}

func (ss *schemaSecrets) shouldBeSecret(path resource.PropertyPath) bool {
	schemaPath := walk.PropertyPathToSchemaPath(path, ss.schemaMap, ss.schemaInfos)
	s, info, err := walk.LookupSchemas(schemaPath, ss.schemaMap, ss.schemaInfos)
	if err != nil {
		return false
	}
	secret := false
	if info != nil && info.Secret != nil {
		secret = *info.Secret
	} else if s != nil {
		secret = s.Sensitive()
	}
	return secret
}

func (ss *schemaSecrets) markSchemaSecretsTransform(
	path resource.PropertyPath,
	value resource.PropertyValue,
) resource.PropertyValue {
	switch {
	case value.IsArray():
		av := value.ArrayValue()
		tvs := make([]resource.PropertyValue, 0, len(av))
		for i, v := range av {
			subPath := append(path, i)
			tv := ss.markSchemaSecretsTransform(subPath, v)
			tvs = append(tvs, tv)
		}
		value = resource.NewArrayProperty(tvs)
	case value.IsObject():
		pm := make(resource.PropertyMap)
		for k, v := range value.ObjectValue() {
			subPath := append(path, string(k))
			tv := ss.markSchemaSecretsTransform(subPath, v)
			pm[k] = tv
		}
		value = resource.NewObjectProperty(pm)
	case value.IsOutput():
		o := value.OutputValue()
		if o.Secret {
			// short-circuit instead of marking nested secrets
			return value
		}

		tv := ss.markSchemaSecretsTransform(path, o.Element)
		value = resource.NewOutputProperty(resource.Output{
			Element:      tv,
			Known:        o.Known,
			Secret:       o.Secret,
			Dependencies: o.Dependencies,
		})
	case value.IsSecret():
		// short-circuit instead of marking nested secrets
		return value
	}

	if !value.IsSecret() && ss.shouldBeSecret(path) {
		value = resource.MakeSecret(value)
	}

	return value
}
