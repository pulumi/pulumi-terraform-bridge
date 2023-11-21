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

package logging

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecoverKeys(t *testing.T) {
	keys := recoverKeys()
	t.Logf("provider: %v", keys.provider)
	require.NotNil(t, keys.provider)
	t.Logf("providerOptions: %v", keys.providerOptions)
	require.NotNil(t, keys.providerOptions)
	t.Logf("sdk: %v", keys.sdk)
	require.NotNil(t, keys.sdk)
	t.Logf("sdkOptions: %v", keys.sdkOptions)
	require.NotNil(t, keys.sdkOptions)
}
