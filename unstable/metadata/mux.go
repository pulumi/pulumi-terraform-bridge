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

package metadata

import (
	"fmt"

	"github.com/pulumi/pulumi-terraform-bridge/x/muxer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const dispatchTableKey = "mux"

func StoreDispatchTable(data *Data, table *muxer.DispatchTable) error {
	contract.Assertf(data != nil, "data cannot be nil")
	if err := Set(data, dispatchTableKey, table); err != nil {
		return fmt.Errorf("Failed to set muxer dispatch table in MetadataInfo: %w", err)
	}
	return nil
}

func LoadDispatchTable(data *Data) (*muxer.DispatchTable, error) {
	contract.Assertf(data != nil, "data cannot be nil")
	if m, found, err := Get[*muxer.DispatchTable](data, dispatchTableKey); err != nil {
		return nil, fmt.Errorf("Failed to load muxer dispatch table from MetadataInfo: %w", err)
	} else if !found {
		return nil, nil
	} else {
		return m, nil
	}
}

func RequireDispatchTable(data *Data) (*muxer.DispatchTable, error) {
	dt, err := LoadDispatchTable(data)
	if err != nil {
		return nil, err
	}
	if dt == nil {
		return nil, fmt.Errorf("Required muxer dispatch table was not found in MetadataInfo: %w", err)
	}
	return dt, nil
}
