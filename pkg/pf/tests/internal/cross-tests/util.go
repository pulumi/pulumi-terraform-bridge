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

package crosstests

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func propageteSkip(parent, child *testing.T) {
	if child.Skipped() {
		parent.Skipf("skipping due to skipped child test")
	}
}

type testLogSink struct{ t *testing.T }

func (s testLogSink) Log(_ context.Context, sev diag.Severity, urn resource.URN, msg string) error {
	return s.log("LOG", sev, urn, msg)
}
func (s testLogSink) LogStatus(_ context.Context, sev diag.Severity, urn resource.URN, msg string) error {
	return s.log("STATUS", sev, urn, msg)
}

func (s testLogSink) log(kind string, sev diag.Severity, urn resource.URN, msg string) error {
	var urnMsg string
	if urn != "" {
		urnMsg = " (" + string(urn) + ")"
	}
	s.t.Logf("Provider[%s]: %s%s: %s", kind, sev, urnMsg, msg)
	return nil
}

func convertResourceValue(t *testing.T, properties resource.PropertyMap) map[string]any {
	var convertValue func(resource.PropertyValue) (any, bool)
	convertValue = func(v resource.PropertyValue) (any, bool) {
		if v.IsComputed() {
			require.Fail(t, "cannot convert computed value to YAML")
		}
		var isSecret bool
		if v.IsOutput() {
			o := v.OutputValue()
			if !o.Known {
				require.Fail(t, "cannot convert unknown output value to YAML")
			}
			v = o.Element
			isSecret = o.Secret
		}
		if v.IsSecret() {
			isSecret = true
			v = v.SecretValue().Element
		}

		if isSecret {
			return map[string]any{
				"fn::secret": v.MapRepl(nil, convertValue),
			}, true
		}
		return nil, false

	}
	return properties.MapRepl(nil, convertValue)
}

func skipUnlessLinux(t *testing.T) {
	if ci, ok := os.LookupEnv("CI"); ok && ci == "true" && !strings.Contains(strings.ToLower(runtime.GOOS), "linux") {
		t.Skip("Skipping on non-Linux platforms as our CI does not yet install Terraform CLI required for these tests")
	}
}
