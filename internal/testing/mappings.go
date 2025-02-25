// Copyright 2016-2025, Pulumi Corporation.
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

package testing

import (
	"context"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
)

type TestFileMapper struct {
	Path string
}

func (l *TestFileMapper) GetMapping(
	_ context.Context,
	provider string,
	hint *convert.MapperPackageHint,
) ([]byte, error) {
	pulumiProvider := provider
	if hint != nil {
		pulumiProvider = hint.PluginName
	}

	mappingPath := filepath.Join(l.Path, pulumiProvider) + ".json"
	mappingBytes, err := os.ReadFile(mappingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	return mappingBytes, nil
}
