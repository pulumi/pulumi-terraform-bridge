// Copyright 2016-2018, Pulumi Corporation.
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
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

// Main launches the tfbridge plugin for a given package pkg and provider prov.
func Main(pkg string, version string, prov ProviderInfo, pulumiSchema []byte) {
	// Look for a request to dump the provider info to stdout.
	flags := flag.NewFlagSet("tf-provider-flags", flag.ContinueOnError)
	dumpInfo := flags.Bool("get-provider-info", false, "dump provider info as JSON to stdout")
	providerVersion := flags.Bool("version", false, "get built provider version")
	contract.IgnoreError(flags.Parse(os.Args[1:]))
	if *dumpInfo {
		if err := json.NewEncoder(os.Stdout).Encode(MarshalProviderInfo(&prov)); err != nil {
			cmdutil.ExitError(err.Error())
		}
		os.Exit(0)
	}
	if *providerVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	// Initialize Terraform logging.
	prov.P.InitLogging()

	if err := Serve(pkg, version, prov, pulumiSchema); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
