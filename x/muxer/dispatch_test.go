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

package muxer

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDispatch(t *testing.T) {
	dt := DispatchTable{
		Resources: map[string]int{
			"aws:auditmanager/assessmentReport:AssessmentReport": 1,
		},
		Functions: map[string]int{
			"aws:auditmanager/getControl:getControl": 1,
		},
		Config: map[string][]int{
			"both": {0, 1},
			"zero": {0},
			"one":  {1},
		},
	}

	i, found := dt.DispatchResource("aws:auditmanager/assessmentReport:AssessmentReport")
	require.True(t, found)
	require.Equal(t, 1, i)

	_, found = dt.DispatchResource("aws:s3/bucket:Bucket")
	require.False(t, found)

	i, found = dt.DispatchFunction("aws:auditmanager/getControl:getControl")
	require.True(t, found)
	require.Equal(t, 1, i)

	_, found = dt.DispatchFunction("aws:unknown/getUnknown:getUnknown")
	require.False(t, found)

	xs, found := dt.DispatchConfig("both")
	require.True(t, found)
	require.Equal(t, []int{0, 1}, xs)

	xs, found = dt.DispatchConfig("zero")
	require.True(t, found)
	require.Equal(t, []int{0}, xs)

	xs, found = dt.DispatchConfig("one")
	require.True(t, found)
	require.Equal(t, []int{1}, xs)

	_, found = dt.DispatchConfig("two")
	require.False(t, found)

	defaultIndex := 2
	dt.ResourcesDefault = &defaultIndex
	dt.FunctionsDefault = &defaultIndex
	dt.ConfigDefault = []int{defaultIndex}

	i, found = dt.DispatchResource("aws:s3/bucket:Bucket")
	require.True(t, found)
	require.Equal(t, defaultIndex, i)

	i, found = dt.DispatchFunction("aws:unknown/getUnknown:getUnknown")
	require.True(t, found)
	require.Equal(t, defaultIndex, i)

	xs, found = dt.DispatchConfig("two")
	require.True(t, found)
	require.Equal(t, []int{defaultIndex}, xs)
}
