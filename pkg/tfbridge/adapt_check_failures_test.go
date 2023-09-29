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

package tfbridge

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim/schema"
)

func TestParseCheckErrorMiscFailure(t *testing.T) {
	schemaMap := &schema.SchemaMap{}
	schemaInfos := map[string]*SchemaInfo{}
	err := errors.New("random unexpected error")

	path, reason, errString := parseCheckError(schemaMap, schemaInfos, err)

	assert.Nil(t, path)
	assert.Equal(t, MiscFailure, reason)
	assert.Equal(t, "random unexpected error", errString)
}
