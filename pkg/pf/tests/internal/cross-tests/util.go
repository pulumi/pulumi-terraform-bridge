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

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	crosstestsimpl "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/internal/tests/cross-tests/impl"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tests/internal/providerbuilder"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
	tfbridge0 "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"
)

type T = crosstestsimpl.T

type testLogSink struct{ t T }

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

func skipUnlessLinux(t T) {
	if ci, ok := os.LookupEnv("CI"); ok && ci == "true" && !strings.Contains(strings.ToLower(runtime.GOOS), "linux") {
		t.Skip("Skipping on non-Linux platforms as our CI does not yet install Terraform CLI required for these tests")
	}
}

func bridgedProvider(prov *providerbuilder.Provider) info.Provider {
	shimProvider := tfbridge.ShimProvider(prov)

	provider := tfbridge0.ProviderInfo{
		P:            shimProvider,
		Name:         prov.TypeName,
		Version:      prov.Version,
		MetadataInfo: &tfbridge0.MetadataInfo{},
	}

	provider.MustComputeTokens(tokens.SingleModule(prov.TypeName, "index", tokens.MakeStandard(prov.TypeName)))

	return provider
}
