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

package propertyvalue

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func Transform(
	transformer func(resource.PropertyValue) resource.PropertyValue,
	value resource.PropertyValue,
) resource.PropertyValue {
	tvalue, _ := TransformPropertyValue(make(resource.PropertyPath, 0),
		func(_ resource.PropertyPath, pv resource.PropertyValue) (resource.PropertyValue, error) {
			return transformer(pv), nil
		}, value)
	return tvalue
}

func TransformErr(
	transformer func(resource.PropertyValue) (resource.PropertyValue, error),
	value resource.PropertyValue,
) (resource.PropertyValue, error) {
	return TransformPropertyValue(make(resource.PropertyPath, 0),
		func(_ resource.PropertyPath, pv resource.PropertyValue) (resource.PropertyValue, error) {
			return transformer(pv)
		}, value)
}

func TransformPropertyValue(
	path resource.PropertyPath,
	transformer func(resource.PropertyPath, resource.PropertyValue) (resource.PropertyValue, error),
	value resource.PropertyValue,
) (resource.PropertyValue, error) {
	switch {
	case value.IsArray():
		tvs := []resource.PropertyValue{}
		for i, v := range value.ArrayValue() {
			tv, err := TransformPropertyValue(extendPath(path, i), transformer, v)
			if err != nil {
				return resource.NewNullProperty(), err
			}
			tvs = append(tvs, tv)
		}
		value = resource.NewArrayProperty(tvs)
	case value.IsObject():
		pm := make(resource.PropertyMap)
		for k, v := range value.ObjectValue() {
			tv, err := TransformPropertyValue(extendPath(path, string(k)), transformer, v)
			if err != nil {
				return resource.NewNullProperty(), err
			}
			pm[k] = tv
		}
		value = resource.NewObjectProperty(pm)
	case value.IsOutput():
		o := value.OutputValue()
		te, err := TransformPropertyValue(path, transformer, o.Element)
		if err != nil {
			return resource.NewNullProperty(), err
		}
		value = resource.NewOutputProperty(resource.Output{
			Element:      te,
			Known:        o.Known,
			Secret:       o.Secret,
			Dependencies: o.Dependencies,
		})
	case value.IsSecret():
		s := value.SecretValue()
		te, err := TransformPropertyValue(path, transformer, s.Element)
		if err != nil {
			return resource.NewNullProperty(), err
		}
		value = resource.NewSecretProperty(&resource.Secret{
			Element: te,
		})
	}
	return transformer(path, value)
}

// Removes any resource.NewSecretProperty wrappers. Removes Secret: true flags from any first-class outputs.
func RemoveSecrets(pv resource.PropertyValue) resource.PropertyValue {
	unsecret := func(pv resource.PropertyValue) resource.PropertyValue {
		if pv.IsSecret() {
			return pv.SecretValue().Element
		}
		if pv.IsOutput() {
			o := pv.OutputValue()
			return resource.NewOutputProperty(resource.Output{
				Element:      o.Element,
				Known:        o.Known,
				Secret:       false,
				Dependencies: o.Dependencies,
			})
		}
		return pv
	}
	return Transform(unsecret, pv)
}

func extendPath(p resource.PropertyPath, segment any) resource.PropertyPath {
	rp := make(resource.PropertyPath, len(p)+1)
	copy(rp, p)
	rp[len(p)] = segment
	return rp
}
