// Copyright 2023, Pulumi Corporation.
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

package tfgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoverageExport(t *testing.T) {
	t.Parallel()
	ce := &CoverageTracker{}
	ct := newCoverageExportUtil(ce)
	err := ct.tryExport(t.TempDir())
	assert.NoError(t, err)
}
