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
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

// TestingT describers what crosstests needs to run a test with.
//
// TestingT should be compatible with [pgregory.net/rapid.T].
type TestingT interface {
	Skip(args ...any)
	Failed() bool
	Errorf(format string, args ...any)
	Name() string
	Log(...any)
	Logf(string, ...any)
	Fail()
	FailNow()
	Helper()
}

type testLogSink struct{ t TestingT }

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

func convertResourceValue(t TestingT, properties resource.PropertyMap) map[string]any {
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

func withAugmentedT(t TestingT, f func(t *augmentedT)) {
	c := augmentedT{TestingT: t}
	defer c.cleanup()
	f(&c)
}

// augmentedT augments
type augmentedT struct {
	TestingT
	tasks []func()
}

// TempDir returns a temporary directory for the test to use.
// The directory is automatically removed when the test and
// all its subtests complete.
// Each subsequent call to t.TempDir returns a unique directory;
// if the directory creation fails, TempDir terminates the test by calling Fatal.
func (t *augmentedT) TempDir() string {
	// If the underlying TestingT actually implements TempDir, then just call that.
	if t, ok := t.TestingT.(interface{ TempDir() string }); ok {
		return t.TempDir()
	}

	// Re-implement TempDir:

	name := t.Name()
	name = strings.ReplaceAll(name, "#", "")
	name = strings.ReplaceAll(name, string(os.PathSeparator), "")
	dir, err := os.MkdirTemp("", name)
	require.NoError(t, err)
	return dir
}

func (t *augmentedT) Cleanup(f func()) {
	// If the underlying TestingT actually implements Cleanup, then just call that.
	if t, ok := t.TestingT.(interface{ Cleanup(f func()) }); ok {
		t.Cleanup(f)
		return
	}

	// Add f to the set of tasks to be cleaned up later. Cleanup is only valid when
	// called in a context where t.cleanup() will be called, such as [withAugmentedT].
	t.tasks = append(t.tasks, f)
}

func (t *augmentedT) Deadline() (time.Time, bool) {
	// If the underlying TestingT actually implements Deadline, then just call that.
	if t, ok := t.TestingT.(interface{ Deadline() (time.Time, bool) }); ok {
		return t.Deadline()
	}

	// Otherwise the test has no deadline.

	return time.Time{}, false
}

func (t *augmentedT) cleanup() {
	for i := len(t.tasks) - 1; i >= 0; i-- {
		v := t.tasks[i]
		if v != nil {
			v()
		}
	}
}
