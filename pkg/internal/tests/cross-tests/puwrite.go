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

package crosstests

import "github.com/pulumi/pulumi/sdk/v3/go/common/resource"

func generateYaml(resourceToken string, puConfig resource.PropertyMap) (map[string]any, error) {
	data := map[string]any{
		"name":    "project",
		"runtime": "yaml",
		"backend": map[string]any{
			"url": "file://./data",
		},
	}
	if puConfig == nil {
		return data, nil
	}

	data["resources"] = map[string]any{
		"example": map[string]any{
			"type": resourceToken,
			// This is a bit of a leap of faith that serializing PropertyMap
			// to YAML in this way will yield valid Pulumi YAML. This probably
			// needs refinement.
			"properties": puConfig.Mappable(),
		},
	}
	return data, nil
}
