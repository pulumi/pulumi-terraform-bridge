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
	"runtime/debug"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/emptypb"
	"gotest.tools/v3/assert"
)

func TestFormatPanicMessage(t *testing.T) {
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
	autogold.Expect(`Bridged provider panic (provider=myprov providerVersion=1.2.3): Something went wrong

        goroutine 5 [running]:
        runtime/debug.Stack()
         /nix/store/lb8wx7xsk7vryjxbbf9wlj820pdfvnbj-go-1.23.3/share/go/src/runtime/debug/stack.go:26 +0x64
`).Equal(t, msg)
}

func TestPanicRecoveryByMethod(t *testing.T) {
	ctx := context.Background()
	type testCase struct {
		testName      string
		send          func(pulumirpc.ResourceProviderServer)
		expectURN     autogold.Value
		expectMessage autogold.Value
	}

	testCases := []testCase{
		{
			testName: "Handshake",
			send: func(rps pulumirpc.ResourceProviderServer) {
				rps.Handshake(ctx, &pulumirpc.ProviderHandshakeRequest{EngineAddress: "localhost"})
			},
			expectURN:     autogold.Expect(urn.URN("")),
			expectMessage: autogold.Expect("Bridged provider panic (provider=myprov v=1.2.3 method=Handshake): Handshake panic"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			logger := &testLogger{}
			s := NewPanicRecoveringProviderServer(&PanicRecoveringProviderServerOptions{
				Logger:                 logger,
				ResourceProviderServer: &testRPS{},
				ProviderName:           "myprov",
				ProviderVersion:        "1.2.3",
			})
			s.includeStackTraces = false

			expectPanic(t, func() { tc.send(s) })

			assert.Equal(t, 1, logger.messageCount)
			tc.expectURN.Equal(t, logger.lastURN)
			tc.expectMessage.Equal(t, logger.lastMsg)
		})
	}
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

func expectPanic(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	f()
}

func exampleResourceURN() string {
	return "urn:pulumi:production::acmecorp-website::custom:resources:Resource$aws:s3/bucketv2:BucketV2::my-bucket"
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
