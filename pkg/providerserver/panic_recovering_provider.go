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
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/emptypb"
)

// A wrapper around a ResourceProviderServer that logs panics.
//
// While bridged providers should never panic, when this does happen it is helpful to include the provider name, version
// and resource information in the error message to assist reproducing the problem quickly.
type PanicRecoveringProviderServer struct {
	pulumirpc.UnimplementedResourceProviderServer
	innerServer     pulumirpc.ResourceProviderServer
	providerName    string
	providerVersion string
	logger          Logger

	// A slot to communicate the URN across CheckConfig and Configure calls.
	currentProviderUrn string

	// Exposed for testing.
	omitStackTraces bool
}

// The minimal interface implemented by HostClient where Error messages will be logged for each panic.
type Logger interface {
	Log(context context.Context, sev diag.Severity, urn resource.URN, msg string) error
}

var _ Logger = (*provider.HostClient)(nil)

type PanicRecoveringProviderServerOptions struct {
	Logger                 Logger
	ResourceProviderServer pulumirpc.ResourceProviderServer
	ProviderName           string
	ProviderVersion        string
}

func NewPanicRecoveringProviderServer(opts *PanicRecoveringProviderServerOptions) *PanicRecoveringProviderServer {
	contract.Assertf(opts.ResourceProviderServer != nil, "wrappedServer must not be nil")
	contract.Assertf(opts.Logger != nil, "logger must not be nil")
	contract.Assertf(opts.ProviderName != "", "providerName must not be empty")
	return &PanicRecoveringProviderServer{
		innerServer:     opts.ResourceProviderServer,
		providerName:    opts.ProviderName,
		providerVersion: opts.ProviderVersion,
		logger:          opts.Logger,
	}
}

var _ pulumirpc.ResourceProviderServer = &PanicRecoveringProviderServer{}

type logPanicOptions struct {
	// If the panic was in response to a resource operation, the URN of the resource.
	resourceURN string

	// If the panic was in response to provider configuration, the URN of the provider. This may help distinguishing
	// default from explicit providers.
	providerURN string

	// If the panic was in response to an Invoke call, the token of the invoked function.
	invokeToken string

	// Lifecycle method that caused the panic.
	method string
}

func (s *PanicRecoveringProviderServer) formatPanicMessage(
	err interface{},
	stack []byte,
	opts *logPanicOptions,
) string {
	// Format metadata key-value pairs in slog default format.
	var buf bytes.Buffer
	l := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			if a.Key == slog.LevelKey {
				return slog.Attr{}
			}
			if a.Key == slog.MessageKey {
				return slog.Attr{}
			}
			return a
		},
	}))
	var attrs []slog.Attr
	attrs = append(attrs, slog.String("provider", s.providerName))
	if s.providerVersion != "" {
		attrs = append(attrs, slog.String("v", s.providerVersion))
	}
	if opts.providerURN != "" {
		attrs = append(attrs, slog.String("providerURN", opts.providerURN))
	}
	if opts.invokeToken != "" {
		attrs = append(attrs, slog.String("invokeToken", opts.invokeToken))
	}
	if opts.resourceURN != "" {
		attrs = append(attrs, slog.String("resourceURN", opts.resourceURN))
	}
	if opts.method != "" {
		attrs = append(attrs, slog.String("method", opts.method))
	}
	l.LogAttrs(context.Background(), slog.LevelError, fmt.Sprintf("%s", err), attrs...)
	metadata := strings.TrimSpace(buf.String())
	if s.omitStackTraces {
		return fmt.Sprintf("Bridged provider panic (%s): %v", metadata, err)
	}
	return fmt.Sprintf("Bridged provider panic (%s): %v\n%s", metadata, err, stack)
}

func (s *PanicRecoveringProviderServer) determinePanicResourceURN(opts *logPanicOptions) resource.URN {
	if opts.resourceURN != "" {
		return urn.URN(opts.resourceURN)
	}
	return ""
}

func (s *PanicRecoveringProviderServer) logPanic(
	ctx context.Context,
	method string,
	err interface{},
	stack []byte,
	opts *logPanicOptions,
) {
	if opts == nil {
		opts = &logPanicOptions{}
	}
	opts.method = method
	msg := s.formatPanicMessage(err, stack, opts)
	urn := s.determinePanicResourceURN(opts)
	logErr := s.logger.Log(ctx, diag.Error, urn, msg)
	contract.IgnoreError(logErr)
}

func (s *PanicRecoveringProviderServer) Handshake(
	ctx context.Context,
	req *pulumirpc.ProviderHandshakeRequest,
) (*pulumirpc.ProviderHandshakeResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "Handshake", err, debug.Stack(), nil)
			panic(err) // rethrow
		}
	}()
	return s.innerServer.Handshake(ctx, req)
}

func (s *PanicRecoveringProviderServer) Parameterize(
	ctx context.Context,
	req *pulumirpc.ParameterizeRequest,
) (*pulumirpc.ParameterizeResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "Parameterize", err, debug.Stack(), nil)
			panic(err) // rethrow
		}
	}()
	return s.innerServer.Parameterize(ctx, req)
}

func (s *PanicRecoveringProviderServer) GetSchema(
	ctx context.Context,
	req *pulumirpc.GetSchemaRequest,
) (*pulumirpc.GetSchemaResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "GetSchema", err, debug.Stack(), nil)
			panic(err) // rethrow
		}
	}()
	return s.innerServer.GetSchema(ctx, req)
}

func (s *PanicRecoveringProviderServer) CheckConfig(
	ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	s.currentProviderUrn = req.Urn
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "CheckConfig", err, debug.Stack(), &logPanicOptions{
				providerURN: req.Urn,
			})
			panic(err) // rethrow
		}
	}()
	return s.innerServer.CheckConfig(ctx, req)
}

func (s *PanicRecoveringProviderServer) DiffConfig(
	ctx context.Context,
	req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "DiffConfig", err, debug.Stack(), &logPanicOptions{
				providerURN: req.Urn,
			})
			panic(err) // rethrow
		}
	}()
	return s.innerServer.DiffConfig(ctx, req)
}

func (s *PanicRecoveringProviderServer) Configure(
	ctx context.Context,
	req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "Configure", err, debug.Stack(), &logPanicOptions{
				providerURN: s.currentProviderUrn,
			})
			panic(err) // rethrow
		}
	}()
	return s.innerServer.Configure(ctx, req)
}

func (s *PanicRecoveringProviderServer) Invoke(
	ctx context.Context,
	req *pulumirpc.InvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "Invoke", err, debug.Stack(), &logPanicOptions{
				invokeToken: req.Tok,
			})
			panic(err) // rethrow
		}
	}()
	return s.innerServer.Invoke(ctx, req)
}

func (s *PanicRecoveringProviderServer) StreamInvoke(
	req *pulumirpc.InvokeRequest,
	srv pulumirpc.ResourceProvider_StreamInvokeServer,
) error {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(context.Background(), "StreamInvoke", err, debug.Stack(), &logPanicOptions{
				invokeToken: req.Tok,
			})
			panic(err) // rethrow
		}
	}()
	return s.innerServer.StreamInvoke(req, srv)
}

func (s *PanicRecoveringProviderServer) Call(
	ctx context.Context,
	req *pulumirpc.CallRequest,
) (*pulumirpc.CallResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			// We could possibly do better here if we inferred the URN of the __self__ argument.
			s.logPanic(ctx, "Call", err, debug.Stack(), &logPanicOptions{
				invokeToken: req.Tok,
			})
			panic(err) // rethrow
		}
	}()
	return s.innerServer.Call(ctx, req)
}

func (s *PanicRecoveringProviderServer) Check(
	ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "Check", err, debug.Stack(), &logPanicOptions{
				resourceURN: req.Urn,
			})
			panic(err) // rethrow
		}
	}()
	return s.innerServer.Check(ctx, req)
}

func (s *PanicRecoveringProviderServer) Diff(
	ctx context.Context,
	req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "Diff", err, debug.Stack(), &logPanicOptions{
				resourceURN: req.Urn,
			})
			panic(err) // rethrow
		}
	}()
	return s.innerServer.Diff(ctx, req)
}

func (s *PanicRecoveringProviderServer) Create(
	ctx context.Context,
	req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "Create", err, debug.Stack(), &logPanicOptions{
				resourceURN: req.Urn,
			})
			panic(err) // rethrow
		}
	}()
	return s.innerServer.Create(ctx, req)
}

func (s *PanicRecoveringProviderServer) Read(
	ctx context.Context,
	req *pulumirpc.ReadRequest,
) (*pulumirpc.ReadResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "Read", err, debug.Stack(), &logPanicOptions{
				resourceURN: req.Urn,
			})
			panic(err) // rethrow
		}
	}()
	return s.innerServer.Read(ctx, req)
}

func (s *PanicRecoveringProviderServer) Update(
	ctx context.Context,
	req *pulumirpc.UpdateRequest,
) (*pulumirpc.UpdateResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "Update", err, debug.Stack(), &logPanicOptions{
				resourceURN: req.Urn,
			})
			panic(err) // rethrow
		}
	}()
	return s.innerServer.Update(ctx, req)
}

func (s *PanicRecoveringProviderServer) Delete(
	ctx context.Context,
	req *pulumirpc.DeleteRequest,
) (*emptypb.Empty, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "Delete", err, debug.Stack(), &logPanicOptions{
				resourceURN: req.Urn,
			})
			panic(err) // rethrow
		}
	}()
	return s.innerServer.Delete(ctx, req)
}

func (s *PanicRecoveringProviderServer) Construct(
	ctx context.Context,
	req *pulumirpc.ConstructRequest,
) (resp *pulumirpc.ConstructResponse, finalError error) {
	defer func() {
		if err := recover(); err != nil {
			opts := &logPanicOptions{}
			opts.resourceURN = string(constructURN(req))
			s.logPanic(ctx, "Construct", err, debug.Stack(), opts)
			panic(err) // rethrow
		}
	}()
	return s.innerServer.Construct(ctx, req)
}

func (s *PanicRecoveringProviderServer) Cancel(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "Cancel", err, debug.Stack(), nil)
			panic(err) // rethrow
		}
	}()
	return s.innerServer.Cancel(ctx, empty)
}

func (s *PanicRecoveringProviderServer) GetPluginInfo(
	ctx context.Context,
	empty *emptypb.Empty,
) (*pulumirpc.PluginInfo, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "GetPluginInfo", err, debug.Stack(), nil)
			panic(err) // rethrow
		}
	}()
	return s.innerServer.GetPluginInfo(ctx, empty)
}

func (s *PanicRecoveringProviderServer) Attach(
	ctx context.Context,
	attach *pulumirpc.PluginAttach,
) (*emptypb.Empty, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "Attach", err, debug.Stack(), nil)
			panic(err) // rethrow
		}
	}()
	return s.innerServer.Attach(ctx, attach)
}

func (s *PanicRecoveringProviderServer) GetMapping(
	ctx context.Context,
	req *pulumirpc.GetMappingRequest,
) (*pulumirpc.GetMappingResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "GetMapping", err, debug.Stack(), nil)
			panic(err) // rethrow
		}
	}()
	return s.innerServer.GetMapping(ctx, req)
}

func (s *PanicRecoveringProviderServer) GetMappings(
	ctx context.Context,
	req *pulumirpc.GetMappingsRequest,
) (*pulumirpc.GetMappingsResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			s.logPanic(ctx, "GetMappings", err, debug.Stack(), nil)
			panic(err) // rethrow
		}
	}()
	return s.innerServer.GetMappings(ctx, req)
}

// Guess Pulumi URN from ConstructRequest.
func constructURN(req *pulumirpc.ConstructRequest) urn.URN {
	// Guess Pulumi URN from ConstructRequest
	stack := tokens.QName(req.Stack)
	proj := tokens.PackageName(req.Project)
	var parentType tokens.Type
	if req.Parent != "" {
		parentUrn, err := urn.Parse(req.Parent)
		if err == nil {
			parentType = parentUrn.Type()
		}
	}
	baseType := tokens.Type(req.Type)
	return urn.New(stack, proj, parentType, baseType, req.Name)
}
