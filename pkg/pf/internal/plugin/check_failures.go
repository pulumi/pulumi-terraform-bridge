// Copyright 2016-2026, Pulumi Corporation.
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

package plugin

import (
	"fmt"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// CheckFailuresError reports CheckConfig failures in their gRPC form, which can carry details such as remediations
// that [plugin.CheckFailure] cannot. The provider server returns them as failures in the CheckConfig response, not
// as an error.
type CheckFailuresError struct {
	Failures []*pulumirpc.CheckFailure
}

func (e CheckFailuresError) Error() string {
	return fmt.Sprintf("CheckFailures %v", e.Failures)
}
