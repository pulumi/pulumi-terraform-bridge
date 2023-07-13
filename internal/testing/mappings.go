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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type TestFileMapper struct {
	Path string
}

func (l *TestFileMapper) GetMapping(_ context.Context, provider string, pulumiProvider string) ([]byte, error) {
	if pulumiProvider == "" {
		pulumiProvider = provider
	}
	if pulumiProvider == "" {
		panic("provider and pulumiProvider cannot both be empty")
	}

	if pulumiProvider == "unknown" {
		// 'unknown' is used as a known provider name that will return nothing, so return early here so we
		// don't hit the standard unknown error below.
		return nil, nil
	}
	if pulumiProvider == "error" {
		// 'error' is used as a known provider name that will cause GetMapping to error, so return early here
		// so we don't hit the standard unknown error below.
		return nil, errors.New("test error")
	}

	mappingPath := filepath.Join(l.Path, pulumiProvider) + ".json"
	mappingBytes, err := os.ReadFile(mappingPath)
	if err != nil {
		if os.IsNotExist(err) {
			panic(fmt.Sprintf("provider %s (%s) is not known to the test system", provider, pulumiProvider))
		}
		panic(err)
	}

	return mappingBytes, nil
}
