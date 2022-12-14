// Copyright 2016-2022, Pulumi Corporation.
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
	"os"
	"path/filepath"
)

type TestFileMapper struct {
	Path string
}

func (l *TestFileMapper) GetMapping(provider string) ([]byte, error) {
	mappingPath := filepath.Join(l.Path, provider) + ".json"

	mappingBytes, err := os.ReadFile(mappingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	return mappingBytes, nil
}
