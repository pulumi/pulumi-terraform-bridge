// Copyright 2016-2025, Pulumi Corporation.
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

package providerserver

import (
	"context"
	"fmt"
	"runtime/debug"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/emptypb"
	"gotest.tools/v3/assert"
)

func TestFormatPanicMessage(t *testing.T) {
	t.Parallel()

	s := &PanicRecoveringProviderServer{
		providerName:    "myprov",
		providerVersion: "1.2.3",
	}

	panicErr, _ := examplePanic()

	exampleStack := `
        goroutine 5 [running]:
        runtime/debug.Stack()
        	/nix/store/lb8wx7xsk7vryjxbbf9wlj820pdfvnbj-go-1.23.3/share/go/src/runtime/debug/stack.go:26 +0x64
`

	msg := s.formatPanicMessage(panicErr, []byte(exampleStack), &logPanicOptions{resourceURN: exampleResourceURN()})
	//nolint:lll
	autogold.Expect(`Bridged provider panic (provider=myprov v=1.2.3 resourceURN=urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket): Something went wrong

        goroutine 5 [running]:
        runtime/debug.Stack()
         /nix/store/lb8wx7xsk7vryjxbbf9wlj820pdfvnbj-go-1.23.3/share/go/src/runtime/debug/stack.go:26 +0x64
`).Equal(t, msg)
}

func TestPanicRecoveryByMethod(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	type testCase struct {
		testName          string
		send              func(pulumirpc.ResourceProviderServer)
		expectURN         autogold.Value
		expectMessage     autogold.Value
		doNotPanicInCheck bool
	}

	testCases := []testCase{
		{
			testName: "Handshake",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.Handshake(ctx, &pulumirpc.ProviderHandshakeRequest{EngineAddress: "localhost"})
				contract.IgnoreError(err)
			},
			expectURN:     autogold.Expect(urn.URN("")),
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 method=Handshake): Handshake panic"),
		},
		{
			testName: "Parameterize",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.Parameterize(ctx, &pulumirpc.ParameterizeRequest{})
				contract.IgnoreError(err)
			},
			expectURN: autogold.Expect(urn.URN("")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 method=Parameterize): Parameterize panic"),
		},
		{
			testName: "GetSchema",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.GetSchema(ctx, &pulumirpc.GetSchemaRequest{})
				contract.IgnoreError(err)
			},
			expectURN:     autogold.Expect(urn.URN("")),
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 method=GetSchema): GetSchema panic"),
		},
		{
			testName: "CheckConfig",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.CheckConfig(ctx, &pulumirpc.CheckRequest{
					Urn: exampleProviderURN(),
				})
				contract.IgnoreError(err)
			},
			expectURN: autogold.Expect(urn.URN("")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 providerURN=urn:pulumi:dev::2024-01-27::pulumi:providers:aws::default_6_67_0::600afa97-4e03-40bd-b032-43e524727453 method=CheckConfig): CheckConfig panic"),
		},
		{
			testName: "DiffConfig",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.DiffConfig(ctx, &pulumirpc.DiffRequest{
					Urn: exampleProviderURN(),
				})
				contract.IgnoreError(err)
			},
			expectURN: autogold.Expect(urn.URN("")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 providerURN=urn:pulumi:dev::2024-01-27::pulumi:providers:aws::default_6_67_0::600afa97-4e03-40bd-b032-43e524727453 method=DiffConfig): DiffConfig panic"),
		},
		{
			testName:          "Configure",
			doNotPanicInCheck: true,
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.CheckConfig(ctx, &pulumirpc.CheckRequest{Urn: exampleProviderURN()})
				contract.IgnoreError(err)
				_, err = rps.Configure(ctx, &pulumirpc.ConfigureRequest{})
				contract.IgnoreError(err)
			},
			expectURN: autogold.Expect(urn.URN("")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 providerURN=urn:pulumi:dev::2024-01-27::pulumi:providers:aws::default_6_67_0::600afa97-4e03-40bd-b032-43e524727453 method=Configure): Configure panic"),
		},
		{
			testName: "Invoke",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.Invoke(ctx, &pulumirpc.InvokeRequest{
					Tok: exampleInvokeToken(),
				})
				contract.IgnoreError(err)
			},
			expectURN: autogold.Expect(urn.URN("")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 invokeToken=aws:acm/getCertificate:getCertificate method=Invoke): Invoke panic"),
		},
		{
			testName: "StreamInvoke",
			send: func(rps pulumirpc.ResourceProviderServer) {
				err := rps.StreamInvoke(&pulumirpc.InvokeRequest{
					Tok: exampleInvokeToken(),
				}, nil)
				contract.IgnoreError(err)
			},
			expectURN: autogold.Expect(urn.URN("")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 invokeToken=aws:acm/getCertificate:getCertificate method=StreamInvoke): StreamInvoke panic"),
		},
		{
			testName: "Call",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.Call(ctx, &pulumirpc.CallRequest{
					Tok: exampleInvokeToken(), // Not a great example, could be improved to show actual Call.
				})
				contract.IgnoreError(err)
			},
			expectURN: autogold.Expect(urn.URN("")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 invokeToken=aws:acm/getCertificate:getCertificate method=Call): Call panic"),
		},
		{
			testName: "Check",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.Check(ctx, &pulumirpc.CheckRequest{
					Urn: exampleResourceURN(),
				})
				contract.IgnoreError(err)
			},
			//nolint:lll
			expectURN: autogold.Expect(urn.URN("urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 resourceURN=urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket method=Check): Check panic"),
		},
		{
			testName: "Diff",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.Diff(ctx, &pulumirpc.DiffRequest{
					Urn: exampleResourceURN(),
				})
				contract.IgnoreError(err)
			},
			//nolint:lll
			expectURN: autogold.Expect(urn.URN("urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 resourceURN=urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket method=Diff): Diff panic"),
		},
		{
			testName: "Create",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.Create(ctx, &pulumirpc.CreateRequest{
					Urn: exampleResourceURN(),
				})
				contract.IgnoreError(err)
			},
			//nolint:lll
			expectURN: autogold.Expect(urn.URN("urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 resourceURN=urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket method=Create): Create panic"),
		},
		{
			testName: "Read",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.Read(ctx, &pulumirpc.ReadRequest{
					Urn: exampleResourceURN(),
				})
				contract.IgnoreError(err)
			},
			//nolint:lll
			expectURN: autogold.Expect(urn.URN("urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 resourceURN=urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket method=Read): Read panic"),
		},
		{
			testName: "Update",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.Update(ctx, &pulumirpc.UpdateRequest{
					Urn: exampleResourceURN(),
				})
				contract.IgnoreError(err)
			},
			//nolint:lll
			expectURN: autogold.Expect(urn.URN("urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 resourceURN=urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket method=Update): Update panic"),
		},
		{
			testName: "Delete",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.Delete(ctx, &pulumirpc.DeleteRequest{
					Urn: exampleResourceURN(),
				})
				contract.IgnoreError(err)
			},
			//nolint:lll
			expectURN: autogold.Expect(urn.URN("urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 resourceURN=urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket method=Delete): Delete panic"),
		},
		{
			testName: "Construct",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.Construct(ctx, &pulumirpc.ConstructRequest{
					Stack:   "pulumi:production",
					Project: "acmecorp-website",
					Type:    "aws:s3/bucketv2:BucketV2",
					Name:    "myBucket",
					Parent:  exampleResourceURN(),
				})
				contract.IgnoreError(err)
			},
			//nolint:lll
			expectURN: autogold.Expect(urn.URN("urn:pulumi:pulumi:production::acmecorp-website::aws:s3/bucketv2:BucketV2$aws:s3/bucketv2:BucketV2::myBucket")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 resourceURN=urn:pulumi:pulumi:production::acmecorp-website::aws:s3/bucketv2:BucketV2$aws:s3/bucketv2:BucketV2::myBucket method=Construct): Construct panic"),
		},
		{
			testName: "Cancel",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.Cancel(ctx, &emptypb.Empty{})
				contract.IgnoreError(err)
			},
			expectURN:     autogold.Expect(urn.URN("")),
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 method=Cancel): Cancel panic"),
		},
		{
			testName: "GetPluginInfo",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.GetPluginInfo(ctx, &emptypb.Empty{})
				contract.IgnoreError(err)
			},
			expectURN: autogold.Expect(urn.URN("")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 method=GetPluginInfo): GetPluginInfo panic"),
		},
		{
			testName: "Attach",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.Attach(ctx, &pulumirpc.PluginAttach{})
				contract.IgnoreError(err)
			},
			expectURN:     autogold.Expect(urn.URN("")),
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 method=Attach): Attach panic"),
		},
		{
			testName: "GetMapping",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.GetMapping(ctx, &pulumirpc.GetMappingRequest{})
				contract.IgnoreError(err)
			},
			expectURN: autogold.Expect(urn.URN("")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 method=GetMapping): GetMapping panic"),
		},
		{
			testName: "GetMappings",
			send: func(rps pulumirpc.ResourceProviderServer) {
				_, err := rps.GetMappings(ctx, &pulumirpc.GetMappingsRequest{})
				contract.IgnoreError(err)
			},
			expectURN: autogold.Expect(urn.URN("")),
			//nolint:lll
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 method=GetMappings): GetMappings panic"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			logger := &testLogger{}
			s := NewPanicRecoveringProviderServer(&PanicRecoveringProviderServerOptions{
				Logger:                 logger,
				ResourceProviderServer: &testRPS{doNotPanicInCheck: tc.doNotPanicInCheck},
				ProviderName:           "myprov",
				ProviderVersion:        "1.2.3",
			})
			s.omitStackTraces = true

			expectPanic(t, func() { tc.send(s) })

			assert.Equal(t, 1, logger.messageCount)
			tc.expectURN.Equal(t, logger.lastURN)
			tc.expectMessage.Equal(t, logger.lastMsg)
		})
	}
}

// With muxed providers, if we accidentally wrap the provider server twice with the log interceptor it should still
// annotate the panic only once.
func TestWrappingIdempotency(t *testing.T) {
	ctx := context.Background()
	logger := &testLogger{}
	provName := "myprov"
	ver := "1.2.3"
	p := &testRPS{}
	s1 := NewPanicRecoveringProviderServer(&PanicRecoveringProviderServerOptions{
		Logger:                 logger,
		ResourceProviderServer: p,
		ProviderName:           provName,
		ProviderVersion:        ver,
	})
	s2 := NewPanicRecoveringProviderServer(&PanicRecoveringProviderServerOptions{
		Logger:                 logger,
		ResourceProviderServer: s1,
		ProviderName:           provName,
		ProviderVersion:        ver,
	})
	expectPanic(t, func() { s2.Check(ctx, &pulumirpc.CheckRequest{}) })
	assert.Equal(t, 1, logger.messageCount)
}

type testLogger struct {
	messageCount int
	lastURN      resource.URN
	lastMsg      string
}

func (log *testLogger) Log(context context.Context, sev diag.Severity, urn resource.URN, msg string) error {
	log.messageCount++
	log.lastURN = urn
	log.lastMsg = msg
	return nil
}

func (log *testLogger) LogStatus(context context.Context, sev diag.Severity, urn resource.URN, msg string) error {
	return log.Log(context, sev, urn, msg)
}

func expectPanic(t *testing.T, f func()) {
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	f()
}

func exampleInvokeToken() string {
	return "aws:acm/getCertificate:getCertificate"
}

func exampleResourceURN() string {
	return "urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket"
}

// Default and explicit providers have slightly different URNs, using the default URNs to test as this package does not
// branch on explicit vs default and it should be sufficient.
func exampleProviderURN() string {
	return "urn:pulumi:dev::2024-01-27::pulumi:providers:aws::default_6_67_0::600afa97-4e03-40bd-b032-43e524727453"
}

func examplePanic() (finalError any, debugStack []byte) {
	defer func() {
		if r := recover(); r != nil {
			debugStack = debug.Stack()
			finalError = r
		}
	}()
	panic("Something went wrong")
}

type testRPS struct {
	pulumirpc.UnimplementedResourceProviderServer
	doNotPanicInCheck bool
}

var _ pulumirpc.ResourceProviderServer = (*testRPS)(nil)

func (s *testRPS) Handshake(
	ctx context.Context,
	req *pulumirpc.ProviderHandshakeRequest,
) (*pulumirpc.ProviderHandshakeResponse, error) {
	panic("Handshake panic")
}

func (s *testRPS) Parameterize(
	ctx context.Context,
	req *pulumirpc.ParameterizeRequest,
) (*pulumirpc.ParameterizeResponse, error) {
	panic("Parameterize panic")
}

func (s *testRPS) GetSchema(
	ctx context.Context,
	req *pulumirpc.GetSchemaRequest,
) (*pulumirpc.GetSchemaResponse, error) {
	panic("GetSchema panic")
}

func (s *testRPS) CheckConfig(
	ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	if s.doNotPanicInCheck {
		return &pulumirpc.CheckResponse{}, nil
	}
	panic("CheckConfig panic")
}

func (s *testRPS) DiffConfig(
	ctx context.Context,
	req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	panic("DiffConfig panic")
}

func (s *testRPS) Configure(
	ctx context.Context,
	req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	panic("Configure panic")
}

func (s *testRPS) Invoke(
	ctx context.Context,
	req *pulumirpc.InvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	panic("Invoke panic")
}

func (s *testRPS) StreamInvoke(
	req *pulumirpc.InvokeRequest,
	srv pulumirpc.ResourceProvider_StreamInvokeServer,
) error {
	panic("StreamInvoke panic")
}

func (s *testRPS) Call(
	ctx context.Context,
	req *pulumirpc.CallRequest,
) (*pulumirpc.CallResponse, error) {
	panic("Call panic")
}

func (s *testRPS) Check(
	ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	panic("Check panic")
}

func (s *testRPS) Diff(
	ctx context.Context,
	req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	panic("Diff panic")
}

func (s *testRPS) Create(
	ctx context.Context,
	req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	panic("Create panic")
}

func (s *testRPS) Read(
	ctx context.Context,
	req *pulumirpc.ReadRequest,
) (*pulumirpc.ReadResponse, error) {
	panic("Read panic")
}

func (s *testRPS) Update(
	ctx context.Context,
	req *pulumirpc.UpdateRequest,
) (*pulumirpc.UpdateResponse, error) {
	panic("Update panic")
}

func (s *testRPS) Delete(
	ctx context.Context,
	req *pulumirpc.DeleteRequest,
) (*emptypb.Empty, error) {
	panic("Delete panic")
}

func (s *testRPS) Construct(
	ctx context.Context,
	req *pulumirpc.ConstructRequest,
) (resp *pulumirpc.ConstructResponse, finalError error) {
	panic("Construct panic")
}

func (s *testRPS) Cancel(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	panic("Cancel panic")
}

func (s *testRPS) GetPluginInfo(
	ctx context.Context,
	empty *emptypb.Empty,
) (*pulumirpc.PluginInfo, error) {
	panic("GetPluginInfo panic")
}

func (s *testRPS) Attach(
	ctx context.Context,
	attach *pulumirpc.PluginAttach,
) (*emptypb.Empty, error) {
	panic("Attach panic")
}

func (s *testRPS) GetMapping(
	ctx context.Context,
	req *pulumirpc.GetMappingRequest,
) (*pulumirpc.GetMappingResponse, error) {
	panic("GetMapping panic")
}

func (s *testRPS) GetMappings(
	ctx context.Context,
	req *pulumirpc.GetMappingsRequest,
) (*pulumirpc.GetMappingsResponse, error) {
	panic("GetMappings panic")
}
