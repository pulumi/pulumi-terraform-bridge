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

	rprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	pfmuxer "github.com/pulumi/pulumi-terraform-bridge/pf/internal/muxer"
	"github.com/pulumi/pulumi-terraform-bridge/pf/internal/schemashim"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
	"github.com/pulumi/pulumi-terraform-bridge/v3/unstable/metadata"
	"github.com/pulumi/pulumi-terraform-bridge/x/muxer"
)

// Implements main() or a bridged Pulumi plugin, complete with argument parsing.
//
// info.P must be constructed with ShimProvider or ShimProviderWithContext.
func Main(ctx context.Context, pkg string, prov tfbridge.ProviderInfo, meta ProviderMetadata) {
	handleFlags(ctx, prov.Version,
		func() (*tfbridge.MarshallableProviderInfo, error) {
			pp, err := newProviderWithContext(ctx, prov, meta)
			if err != nil {
				return nil, err
			}
			return pp.(*provider).marshalProviderInfo(ctx), nil
		})
	// TODO[pulumi/pulumi-terraform-bridge#820]
	// prov.P.InitLogging()

	if err := serve(ctx, pkg, prov, meta); err != nil {
		cmdutil.ExitError(err.Error())
	}
}

func handleFlags(
	ctx context.Context, version string,
	getProviderInfo func() (*tfbridge.MarshallableProviderInfo, error),
) {
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
		info, err := getProviderInfo()
		if err != nil {
			cmdutil.ExitError(err.Error())
		}
		if err := json.NewEncoder(os.Stdout).Encode(info); err != nil {
			cmdutil.ExitError(err.Error())
		}
		os.Exit(0)
	}

	if *providerVersion {
		fmt.Println(version)
		os.Exit(0)
	}
}

// Implements main() or a bridged Pulumi plugin, complete with argument parsing.
//
// This is an experimental API.
func MainWithMuxer(ctx context.Context, pkg string, info tfbridge.ProviderInfo, schema []byte) {
	if len(info.MuxWith) > 0 {
		panic("mixin providers via tfbridge.ProviderInfo.MuxWith is currently not supported")
	}
	handleFlags(ctx, info.Version, func() (*tfbridge.MarshallableProviderInfo, error) {
		info := info
		return tfbridge.MarshalProviderInfo(&info), nil
	})

	f := MakeMuxedServer(ctx, pkg, info, schema)

	err := rprovider.Main(pkg, f)
	if err != nil {
		cmdutil.ExitError(err.Error())
	}
}

// Create a function to produce a Muxed provider.
//
// This function exposes implementation details for testing. It should not be used outside
// of pulumi-terraform-bridge.  This is an experimental API.
func MakeMuxedServer(
	ctx context.Context, pkg string, info tfbridge.ProviderInfo, schema []byte,
) func(host *rprovider.HostClient) (pulumirpc.ResourceProviderServer, error) {

	shim, ok := info.P.(*pfmuxer.ProviderShim)
	contract.Assertf(ok, "MainWithMuxer must have a ProviderInfo.P created with AugmentShimWithPF")
	_, err := shim.ResolveDispatch(&info)
	contract.AssertNoErrorf(err, "Failed to re-apply alias mappings")
	version := info.Version
	dispatchTable, found, err := metadata.Get[muxer.DispatchTable](info.GetMetadata(), "mux")
	if err != nil {
		cmdutil.ExitError(err.Error())
	}
	if !found {
		fmt.Printf("Missing precomputed mapping. Did you run `make tfgen`?")
		os.Exit(1)
	}

	getTFMapping := func(muxer.GetMappingArgs) (muxer.GetMappingResponse, error) {
		info := info
		marshalled := tfbridge.MarshalProviderInfo(&info)
		data, err := json.Marshal(marshalled)
		mapped := info.ResourcePrefix
		if mapped == "" {
			mapped = info.Name
		}
		return muxer.GetMappingResponse{
			Provider: mapped,
			Data:     data,
		}, err
	}
	m := muxer.Main{
		DispatchTable: dispatchTable,
		Schema:        schema,
		GetMappingHandler: map[string]muxer.MultiMappingHandler{
			"tf":        getTFMapping,
			"terraform": getTFMapping,
		},
	}
	return func(host *rprovider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		for _, prov := range shim.MuxedProviders {
			prov := prov // https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables

			switch prov := prov.(type) {
			case *schemashim.SchemaOnlyProvider:
				m.Servers = append(m.Servers, muxer.Endpoint{
					Server: func(host *rprovider.HostClient) (pulumirpc.ResourceProviderServer, error) {
						infoCopy := info
						infoCopy.P = prov
						return NewProviderServer(ctx, host,
							infoCopy, ProviderMetadata{PackageSchema: schema})
					}})
			default:
				m.Servers = append(m.Servers, muxer.Endpoint{
					Server: func(host *rprovider.HostClient) (pulumirpc.ResourceProviderServer, error) {
						return tfbridge.NewProvider(ctx, host, pkg, version, prov, info, schema), nil
					}})
			}
		}
		return m.Server(host, pkg, version)
	}
}
