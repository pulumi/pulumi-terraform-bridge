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

package parameterize

import (
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Value represents a fully qualified provider reference suitable to be embedded in
// generated SDKs.
//
// It is conceptually an either, and only Remote or Local will be non-nil.
type Value struct {
	Remote *RemoteValue `json:"remote,omitempty"`
	Local  *LocalValue  `json:"local,omitempty"`
	// Includes is the list of resource and datasource names to include in the
	// provider.  If empty, all resources are included.
	Includes []string `json:"includes,omitempty"`
}

type RemoteValue struct {
	// The fully qualified URL of the remote value.
	URL string `json:"url"`
	// The fully specified version of the remote value.
	Version string `json:"version"`
}

type LocalValue struct {
	// The local or absolute path to the Terraform provider on disk.
	Path string `json:"path"`
}

func (p Value) Marshal() []byte {
	contract.Assertf((p.Remote == nil) != (p.Local == nil),
		"cannot marshal an invalid value: p.Remote XOR p.Local must be set")
	switch {
	case p.Remote != nil:
		contract.Assertf(p.Remote.URL != "", "Url cannot be empty")
		contract.Assertf(p.Remote.Version != "", "Version cannot be empty")
	case p.Local != nil:
		contract.Assertf(p.Local.Path != "", "Path cannot be empty")
	}
	b, err := json.Marshal(p)
	contract.AssertNoErrorf(err, "p is composed of basic and non-recursive types, so it must be marshalable")
	return b
}

func (p *Value) Unmarshal(b []byte) error {
	err := json.Unmarshal(b, p)
	if err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}
	if p.Remote != nil && p.Local != nil {
		return fmt.Errorf(`cannot specify both "remote" and "local"`)
	}
	switch {
	case p.Remote != nil:
		if p.Remote.URL == "" {
			return fmt.Errorf("remote.url cannot be empty")
		}
		if p.Remote.Version == "" {
			return fmt.Errorf("remote.version cannot be empty")
		}
	case p.Local != nil:
		if p.Local.Path == "" {
			return fmt.Errorf("local.path cannot be empty")
		}
	default:
		return fmt.Errorf(`must specify either "remote" or "local"`)
	}
	return nil
}

func ParseValue(b []byte) (Value, error) {
	var value Value
	err := value.Unmarshal(b)
	return value, err
}

// IntoArgs converts a [Value] into an [Args].
//
// We can do this because [Value] is a fully resolved [Args], and so it is always possible to
// go from [Value] to [Args].
func (p *Value) IntoArgs() Args {
	if p.Local != nil {
		return Args{Local: &LocalArgs{
			Path: p.Local.Path,
		}, Includes: p.Includes}
	}
	return Args{Remote: &RemoteArgs{
		Name:    p.Remote.URL,
		Version: p.Remote.Version,
	}, Includes: p.Includes}
}
