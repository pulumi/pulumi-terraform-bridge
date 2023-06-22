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

package defaults

import (
	"fmt"
	"strings"
)

// Indicate that a token cannot be mapped.
type UnmappableError struct {
	TfToken string
	Reason  error
}

func (err UnmappableError) Error() string {
	return fmt.Sprintf("'%s' unmappable: %s", err.TfToken, err.Reason)
}

func (err UnmappableError) Unwrap() error {
	return err.Reason
}

func (ts Strategy) Unmappable(substring, reason string) Strategy {
	ts.DataSource = ts.DataSource.Unmappable(substring, reason)
	ts.Resource = ts.Resource.Unmappable(substring, reason)
	return ts
}

// Mark that a strategy cannot handle a sub-string.
func (ts ElementStrategy[T]) Unmappable(substring, reason string) ElementStrategy[T] {
	msg := fmt.Sprintf("cannot map tokens that contains '%s'", substring)
	if reason != "" {
		msg += ": " + reason
	}
	return func(tfToken string) (*T, error) {
		if strings.Contains(tfToken, substring) {
			return nil, UnmappableError{
				TfToken: tfToken,
				Reason:  fmt.Errorf(msg),
			}
		}
		return ts(tfToken)
	}
}
