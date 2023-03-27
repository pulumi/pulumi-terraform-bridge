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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Implements main() or a bridged Pulumi plugin, complete with argument parsing.
func Main(ctx context.Context, pkg string, prov ProviderInfo, meta ProviderMetadata) {
	version := prov.Version

	// Look for a request to dump the provider info to stdout.
	flags := flag.NewFlagSet("tf-provider-flags", flag.ContinueOnError)

	// Discard print output by default; there might be flags such
	// as -tracing that are unrecognized at this phase but will be
	// parsed later by `Serve`. We do not want to print errors
	// about them. Save `defaultOutput` for help below.
	defaultOutput := flags.Output()
	flags.SetOutput(io.Discard)

	dumpInfo := flags.Bool("get-provider-info", false, "dump provider info as JSON to stdout")
	providerVersion := flags.Bool("version", false, "get built provider version")

	err := flags.Parse(os.Args[1:])
	contract.IgnoreError(err)

	// Ensure we do print help message when `--help` is requested.
	if err == flag.ErrHelp {
		flags.SetOutput(defaultOutput)
		err := flags.Parse(os.Args[1:])
		if err != nil {
			cmdutil.ExitError(err.Error())
		}
	}

	if *dumpInfo {
		pp, err := NewProvider(ctx, prov, meta)
		if err != nil {
			cmdutil.ExitError(err.Error())
		}
		info := pp.(*provider).marshalProviderInfo(ctx)
		if err := json.NewEncoder(os.Stdout).Encode(info); err != nil {
			cmdutil.ExitError(err.Error())
		}
		os.Exit(0)
	}

	if *providerVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if err := serve(ctx, pkg, prov, meta); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
