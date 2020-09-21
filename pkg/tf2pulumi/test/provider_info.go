// Copyright 2016-2018, Pulumi Corporation.
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

package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"

	"github.com/pulumi/pulumi-terraform-bridge/v2/pkg/tfbridge"
)

type ProviderInfoSource struct {
	m sync.RWMutex

	infoDirectoryPath string
	entries           map[string]*tfbridge.ProviderInfo
}

func (s *ProviderInfoSource) getProviderInfo(tfProviderName string) (*tfbridge.ProviderInfo, bool) {
	s.m.RLock()
	defer s.m.RUnlock()

	info, ok := s.entries[tfProviderName]
	return info, ok
}

// GetProviderInfo returns the tfbridge information for the indicated Terraform provider as well as the name of the
// corresponding Pulumi resource provider.
func (s *ProviderInfoSource) GetProviderInfo(
	registryName, namespace, name, version string) (*tfbridge.ProviderInfo, error) {

	if info, ok := s.getProviderInfo(name); ok {
		return info, nil
	}

	s.m.Lock()
	defer s.m.Unlock()

	f, err := os.Open(filepath.Join(s.infoDirectoryPath, name+".json"))
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(f)

	var m tfbridge.MarshallableProviderInfo
	if err = json.NewDecoder(f).Decode(&m); err != nil {
		return nil, err
	}

	info := m.Unmarshal()
	s.entries[name] = info
	return info, nil
}

// NewProviderInfoSource creates a new ProviderInfoSource that loads serialized provider info from the fgiven directory.
func NewProviderInfoSource(path string) *ProviderInfoSource {
	return &ProviderInfoSource{
		infoDirectoryPath: path,
		entries:           map[string]*tfbridge.ProviderInfo{},
	}
}
