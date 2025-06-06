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

package muxer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	urn "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// The version expected to be specified by GetSchema
const SchemaVersion int32 = 0

func mux(
	host *provider.HostClient,
	dispatchTable dispatchTable,
	pulumiSchema []byte,
	getMappingHandlers getMappingHandler,
	servers ...pulumirpc.ResourceProviderServer,
) *muxer {
	contract.Assertf(len(servers) > 0, "Cannot instantiate an empty muxer")
	return &muxer{
		host:            host,
		servers:         servers,
		schema:          pulumiSchema,
		dispatchTable:   dispatchTable,
		getMappingByKey: getMappingHandlers,
	}
}

var _ pulumirpc.ResourceProviderServer = ((*muxer)(nil))

type server = pulumirpc.ResourceProviderServer

type muxer struct {
	pulumirpc.UnimplementedResourceProviderServer

	host hostClient

	dispatchTable dispatchTable

	schema []byte

	servers []server

	getMappingByKey map[string]MultiMappingHandler
}

// An interface to make *provider.HostClient test-able.
type hostClient interface {
	io.Closer
	Log(context.Context, diag.Severity, urn.URN, string) error
}

type GetMappingArgs interface {
	Fetch() []GetMappingResponse
}

type GetMappingResponse struct {
	Provider string
	Data     []byte
}

type (
	getMappingHandler   = map[string]MultiMappingHandler
	MultiMappingHandler = func(GetMappingArgs) (GetMappingResponse, error)
)

func (m *muxer) getFunction(token string) server {
	// TODO[pulumi/pulumi-terraform-bridge#3032] return a provider that we know implements the terraformConfig function
	if strings.Contains(token, "terraformConfig") {
		return m.servers[0]
	}
	i, ok := m.dispatchTable.Functions[token]
	if !ok {
		return nil
	}

	return m.servers[i]
}

func (m *muxer) getResource(token string) server {
	i, ok := m.dispatchTable.Resources[token]
	if !ok {
		return nil
	}
	return m.servers[i]
}

func (m *muxer) GetSchema(ctx context.Context, req *pulumirpc.GetSchemaRequest) (*pulumirpc.GetSchemaResponse, error) {
	if req.Version != SchemaVersion {
		return nil, fmt.Errorf("Expected schema version %d, got %d",
			SchemaVersion, req.GetVersion())
	}
	return &pulumirpc.GetSchemaResponse{Schema: string(m.schema)}, nil
}

func (m *muxer) CheckConfig(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	subs := make([]func() tuple[*pulumirpc.CheckResponse, error], len(m.servers))
	for i, s := range m.servers {
		i, s := i, s
		subs[i] = func() tuple[*pulumirpc.CheckResponse, error] {
			req := proto.Clone(req).(*pulumirpc.CheckRequest)
			return newTuple(s.CheckConfig(ctx, req))
		}
	}

	inputs := &structpb.Struct{Fields: map[string]*structpb.Value{}}
	failures := []*pulumirpc.CheckFailure{}
	uniqueFailures := map[string]struct{}{}
	var errs multierror.Error
	uniqueErrors := map[string]struct{}{}
	for i, r := range asyncJoin(subs) {
		if err := r.B; err != nil {
			errString := err.Error()
			// De-duplicate errors
			if _, has := uniqueErrors[errString]; has {
				continue
			}
			uniqueErrors[errString] = struct{}{}
			errs.Errors = append(errs.Errors, err)
			continue
		}

		// Add missing inputs, but don't override existing inputs.
		for k, v := range r.A.GetInputs().GetFields() {
			existingValue, has := inputs.Fields[k]
			if has && !proto.Equal(existingValue, v) {
				// If different servers return different values, pick arbitrarily.
				glog.V(9).Infof("[muxer] CheckConfig results do not agree on the '%s' property:"+
					"\n    server %d: %s"+
					"\n    server %d: %s"+
					"\nPicking the server %d response",
					k, i-1, showStruct(existingValue), i, showStruct(v), i-1)
			} else {
				inputs.Fields[k] = v
			}
		}

		// Here we de-duplicate rpc failures.
		for _, e := range r.A.GetFailures() {
			s := e.GetProperty() + ":" + e.GetReason()
			if _, has := uniqueFailures[s]; has {
				continue
			}
			uniqueFailures[s] = struct{}{}
			failures = append(failures, e)
		}
	}

	return &pulumirpc.CheckResponse{
		Inputs:   inputs,
		Failures: failures,
	}, m.muxedErrors(&errs)
}

// Mux multiple errors into a single error, preserving meaningful gRPC status information
// embedded into the errors.
func (m *muxer) muxedErrors(errs *multierror.Error) error {
	unimplementedCount := 0
	validErrors := multierror.Error{}

	for _, err := range errs.Errors {
		if status.Code(err) == codes.Unimplemented {
			unimplementedCount++
		} else {
			validErrors.Errors = append(validErrors.Errors, err)
		}
	}
	// If every server returned unimplemented, we need to return unimplemented
	// too. This way actually unimplemeted calls won't error when the reach the
	// engine.
	if unimplementedCount == len(m.servers) {
		return status.Error(codes.Unimplemented, errs.Error())
	}

	if len(validErrors.Errors) == 1 {
		return validErrors.Errors[0]
	}
	// Its OK for muxed calls to have some servers return unimplmeneted. We filter
	// those errors out.
	return validErrors.ErrorOrNil()
}

func (m *muxer) DiffConfig(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	subs := make([]func() tuple[*pulumirpc.DiffResponse, error], len(m.servers))
	for i, s := range m.servers {
		i, s := i, s
		subs[i] = func() tuple[*pulumirpc.DiffResponse, error] {
			req := proto.Clone(req).(*pulumirpc.DiffRequest)
			return newTuple(s.DiffConfig(ctx, req))
		}
	}

	responses := asyncJoin(subs)
	if resp := dominatingDiffResponse(responses); resp != nil {
		return resp, nil
	}

	var (
		deleteBeforeReplace bool // The OR of each server

		replaces = make(set[string]) // The AND of each server
		diffs    = make(set[string]) // The AND of each server, sans replaces
		stables  = make(set[string]) // The AND of each server, sans replaces and diffs

		changes = pulumirpc.DiffResponse_DIFF_NONE

		errs = new(multierror.Error)
	)

	var (
		detailedDiff    = map[string]*pulumirpc.PropertyDiff{}
		hasDetailedDiff = true
	)

	for _, r := range responses {
		if err := r.B; err != nil {
			errs.Errors = append(errs.Errors, err)
			continue
		}

		resp := r.A

		if resp.DeleteBeforeReplace {
			deleteBeforeReplace = true
		}

		if changes == pulumirpc.DiffResponse_DIFF_NONE {
			changes = resp.GetChanges()
		}

		stables.extend(resp.GetStables())
		replaces.extend(resp.GetReplaces())
		diffs.extend(resp.GetDiffs())

		// If any provider is lacking a detailed diff, we don't attempt to combine
		// a detailed and non-detailed diff.
		//
		// Simply checking HasDetailedDiff is not enough since marshalDiff from below
		// forgets to set it:
		//
		// https://github.com/pulumi/pulumi/blob/master/sdk/go/common/resource/plugin/provider_server.go#L70
		if (!resp.HasDetailedDiff && len(resp.DetailedDiff) == 0) || !hasDetailedDiff {
			hasDetailedDiff = false
			detailedDiff = nil
		} else {
			err := mergeDetailedDiff(detailedDiff, resp.DetailedDiff)
			errs = multierror.Append(errs, err)
		}
	}

	diffs = diffs.setMinus(replaces)
	stables = stables.setMinus(replaces).setMinus(diffs)
	return &pulumirpc.DiffResponse{
		Replaces:            replaces.elements(),
		Stables:             stables.elements(),
		DeleteBeforeReplace: deleteBeforeReplace,
		Changes:             changes,
		Diffs:               diffs.elements(),

		HasDetailedDiff: hasDetailedDiff,
		DetailedDiff:    detailedDiff,
	}, m.muxedErrors(errs)
}

func (m *muxer) Configure(ctx context.Context, req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
	// Configure determines what the values the provider understands. We take the
	// `and` of configure values.
	response := &pulumirpc.ConfigureResponse{
		AcceptSecrets:                   true,
		SupportsPreview:                 true,
		AcceptResources:                 true,
		AcceptOutputs:                   true,
		SupportsAutonamingConfiguration: true,
	}
	errs := new(multierror.Error)

	for _, s := range m.servers {
		req := proto.Clone(req).(*pulumirpc.ConfigureRequest)

		var r *pulumirpc.ConfigureResponse
		var err error

		if errs.Len() > 0 {
			var panicked bool
			r, panicked, err = panicRecoveringConfigure(ctx, s, req)
			if panicked {
				continue
			}
		} else {
			r, err = s.Configure(ctx, req)
		}

		if err != nil {
			errs.Errors = append(errs.Errors, err)
			continue
		}
		response.AcceptOutputs = response.AcceptOutputs && r.GetAcceptOutputs()
		response.AcceptResources = response.AcceptResources && r.GetAcceptResources()
		response.AcceptSecrets = response.AcceptSecrets && r.GetAcceptSecrets()
		response.SupportsPreview = response.SupportsPreview && r.GetSupportsPreview()
		response.SupportsAutonamingConfiguration = response.SupportsAutonamingConfiguration &&
			r.GetSupportsAutonamingConfiguration()
	}
	return response, m.muxedErrors(errs)
}

func panicRecoveringConfigure(
	ctx context.Context,
	s server,
	req *pulumirpc.ConfigureRequest,
) (response *pulumirpc.ConfigureResponse, panicked bool, finalError error) {
	defer func() {
		if p := recover(); p != nil {
			panicked = true
		}
	}()
	r, err := s.Configure(ctx, req)
	return r, false, err
}

type resourceRequest interface {
	GetUrn() string
}

func resourceMethod[T resourceRequest, R any](m *muxer, req T, f func(m server) (R, error)) (R, error) {
	urn := urn.URN(req.GetUrn())
	if !urn.IsValid() {
		return zero[R](), fmt.Errorf("URN '%s' is not valid", string(urn))
	}
	server := m.getResource(string(urn.Type()))
	if server == nil {
		return zero[R](), status.Errorf(codes.NotFound, "Resource type '%s' not found.", urn.Type())
	}
	return f(server)
}

func (m *muxer) Invoke(ctx context.Context, req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
	server := m.getFunction(req.GetTok())
	if server == nil {
		return nil, status.Errorf(codes.NotFound, "Invoke '%s' not found.", req.GetTok())
	}
	return server.Invoke(ctx, req)
}

func (m *muxer) Call(ctx context.Context, req *pulumirpc.CallRequest) (*pulumirpc.CallResponse, error) {
	server := m.getFunction(req.GetTok())
	if server == nil {
		return nil, status.Errorf(codes.NotFound, "Resource Method '%s' not found.", req.GetTok())
	}
	return server.Call(ctx, req)
}

func (m *muxer) Check(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return resourceMethod(m, req, func(m server) (*pulumirpc.CheckResponse, error) {
		return m.Check(ctx, req)
	})
}

func (m *muxer) Diff(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return resourceMethod(m, req, func(m server) (*pulumirpc.DiffResponse, error) {
		return m.Diff(ctx, req)
	})
}

func (m *muxer) Create(ctx context.Context, req *pulumirpc.CreateRequest) (*pulumirpc.CreateResponse, error) {
	return resourceMethod(m, req, func(m server) (*pulumirpc.CreateResponse, error) {
		return m.Create(ctx, req)
	})
}

func (m *muxer) Read(ctx context.Context, req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
	return resourceMethod(m, req, func(m server) (*pulumirpc.ReadResponse, error) {
		return m.Read(ctx, req)
	})
}

func (m *muxer) Update(ctx context.Context, req *pulumirpc.UpdateRequest) (*pulumirpc.UpdateResponse, error) {
	return resourceMethod(m, req, func(m server) (*pulumirpc.UpdateResponse, error) {
		return m.Update(ctx, req)
	})
}

func (m *muxer) Delete(ctx context.Context, req *pulumirpc.DeleteRequest) (*emptypb.Empty, error) {
	return resourceMethod(m, req, func(m server) (*emptypb.Empty, error) {
		return m.Delete(ctx, req)
	})
}

func (m *muxer) Construct(ctx context.Context, req *pulumirpc.ConstructRequest) (*pulumirpc.ConstructResponse, error) {
	server := m.getResource(req.GetType())
	if server == nil {
		return nil, status.Errorf(codes.NotFound, "Component Resource type '%s' does not exist.", req.GetType())
	}
	return server.Construct(ctx, req)
}

func (m *muxer) Cancel(ctx context.Context, e *emptypb.Empty) (*emptypb.Empty, error) {
	subs := make([]func() error, len(m.servers))
	for i, s := range m.servers {
		s := s
		subs[i] = func() error { _, err := s.Cancel(ctx, e); return err }
	}
	errs := new(multierror.Error)
	for _, err := range asyncJoin(subs) {
		if err != nil {
			errs.Errors = append(errs.Errors, err)
		}
	}
	return e, m.muxedErrors(errs)
}

func (m *muxer) GetPluginInfo(ctx context.Context, e *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	// rpc.PluginInfo only returns the version. We just return the version
	// of the most prominent plugin.
	return m.servers[0].GetPluginInfo(ctx, e)
}

func (m *muxer) Attach(ctx context.Context, req *pulumirpc.PluginAttach) (*emptypb.Empty, error) {
	attach := make([]func() error, len(m.servers))
	for i, s := range m.servers {
		s := s
		attach[i] = func() error {
			_, err := s.Attach(ctx, req)
			return err
		}
	}

	var closeErr error
	// Because in Go, an interface type is not nil even when its underlying value is nil, the nil check here
	// must test the underlying type.
	if !reflect.ValueOf(m.host).IsNil() {
		closeErr = m.host.Close()
	}

	var err error
	m.host, err = provider.NewHostClient(req.GetAddress())
	return &emptypb.Empty{}, errors.Join(append(asyncJoin(attach), err, closeErr)...)
}

type getMappingArgs struct {
	m   *muxer
	req *pulumirpc.GetMappingRequest
	ctx context.Context

	err error
}

func (a *getMappingArgs) Fetch() []GetMappingResponse {
	resp, err := a.m.getMappingRaw(a.ctx, a.req, false)
	a.err = err
	return resp
}

func (m *muxer) GetMapping(
	ctx context.Context, req *pulumirpc.GetMappingRequest,
) (*pulumirpc.GetMappingResponse, error) {
	// We need to merge multiple mappings
	combineMapping, found := m.getMappingByKey[req.Key]
	if !found {
		results, err := m.getMappingRaw(ctx, req, true)
		if err != nil {
			return nil, err
		}

		switch len(results) {
		case 0:
			// There are no results and some sub-providers implemented the
			// method. This means that no provider responded to this key. We return an
			// empty response.
			return &pulumirpc.GetMappingResponse{}, nil
		case 1:
			// We don't need to worry about merging GetMapping data if there is only one
			// server with valid data.
			return &pulumirpc.GetMappingResponse{
				Provider: results[0].Provider,
				Data:     results[0].Data,
			}, nil
		}
		return nil, fmt.Errorf("No multi-mapping handler for GetMapping key '%s'", req.Key)
	}

	args := getMappingArgs{m, req, ctx, nil}
	result, err := combineMapping(&args)
	if err != nil {
		if args.err != nil {
			return nil, fmt.Errorf("%w (sub-provider GetMapping call failed: %w)", err, args.err)
		}
		return nil, err
	}
	return &pulumirpc.GetMappingResponse{
		Provider: result.Provider,
		Data:     result.Data,
	}, nil
}

func (m *muxer) getMappingRaw(
	ctx context.Context, req *pulumirpc.GetMappingRequest, strict bool,
) ([]GetMappingResponse, error) {
	subs := make([]func() tuple[*pulumirpc.GetMappingResponse, error], len(m.servers))
	for i, s := range m.servers {
		i, s := i, s
		subs[i] = func() tuple[*pulumirpc.GetMappingResponse, error] {
			return newTuple(s.GetMapping(ctx, proto.Clone(req).(*pulumirpc.GetMappingRequest)))
		}
	}
	errs := new(multierror.Error)
	results := []GetMappingResponse{}
	var providerName string
	for i, v := range asyncJoin(subs) {
		if err := v.B; err != nil {
			errs.Errors = append(errs.Errors, err)
			continue
		}
		response := v.A
		if len(response.Data) == 0 {
			continue
		}
		if response.Provider == "" && strict {
			errs.Errors = append(errs.Errors,
				fmt.Errorf("Missing provider name for subprovider %d", i))
			continue
		} else if providerName == "" {
			providerName = response.Provider
		} else if providerName != response.Provider && strict {
			errs = multierror.Append(errs,
				m.Warnf(ctx, "GetMapping",
					"Ignoring Mapping data due to provider name mismatch: %s != %s",
					providerName, response.Provider))
			continue
		}
		results = append(results, GetMappingResponse{
			Provider: response.Provider,
			Data:     response.Data,
		})
	}

	if err := m.muxedErrors(errs); err != nil {
		return nil, err
	}

	return results, nil
}

func (m *muxer) Warnf(ctx context.Context, method, msg string, a ...any) error {
	return m.host.Log(ctx, diag.Warning, "", fmt.Sprintf("[muxer/"+method+"] "+msg, a...))
}

// `mergeDetailedDiff` copies values from `src` to  `dst`.
//
// A returned err indicates a conflict between src and dst.
func mergeDetailedDiff(dst map[string]*pulumirpc.PropertyDiff, src map[string]*pulumirpc.PropertyDiff) error {
	var errs []error
	for k, v := range src {
		existing, ok := dst[k]
		if !ok {
			// This diff does not exist in `dst`, so just copy it over.
			dst[k] = v
			continue
		}

		// Both diffs are equal, so no need to error
		if proto.Equal(existing, v) {
			continue
		}
		errs = append(errs, fmt.Errorf(`["%s"]: provider mismatch (%v != %v)`, k, existing, v))
	}
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return (&multierror.Error{Errors: errs}).ErrorOrNil()
	}
}

// Call n similar functions, returning a slice of their results.
//
// It is safe to assume that each function lines up with it's result.
func asyncJoin[T any](f []func() T) []T {
	var wg sync.WaitGroup
	wg.Add(len(f))
	// Safety: The array is concurrently accessed, but each go-thread only accesses
	// it's own pre-allocated cell. This means there will be no contention between threads.
	out := make([]T, len(f))
	for i, f := range f {
		i := i
		f := f
		go func() {
			out[i] = f()
			wg.Done()
		}()
	}

	// The wait group ensures that concurrency is fully encapsulated with `asyncJoin`.
	wg.Wait()
	return out
}

type tuple[A, B any] struct {
	A A
	B B
}

func newTuple[A, B any](a A, b B) tuple[A, B] {
	return tuple[A, B]{a, b}
}

func zero[T any]() T {
	var t T
	return t
}

type set[T comparable] map[T]struct{}

func (s set[T]) extend(elements []T) {
	for _, v := range elements {
		s[v] = struct{}{}
	}
}

func (s set[T]) elements() []T {
	elements := make([]T, 0, len(s))
	for e := range s {
		elements = append(elements, e)
	}
	return elements
}

func (s set[T]) setMinus(other set[T]) set[T] {
	new := set[T]{}
	for k := range s {
		if _, has := other[k]; has {
			continue
		}
		new[k] = struct{}{}
	}
	return new
}

func showStruct(value *structpb.Value) string {
	j, err := protojson.Marshal(value)
	if err != nil {
		return err.Error()
	}
	return string(j)
}

func dominatingDiffResponse(responses []tuple[*pulumirpc.DiffResponse, error]) *pulumirpc.DiffResponse {
	unimplemented := 0
	errors := 0
	var resp *pulumirpc.DiffResponse
	for _, r := range responses {
		if r.B != nil {
			errors++
			if s, ok := status.FromError(r.B); ok {
				if s.Code() == codes.Unimplemented {
					unimplemented++
					continue
				}
			}
		} else {
			resp = r.A
		}
	}
	if errors == len(responses)-1 && errors == unimplemented {
		return resp
	}
	return nil
}
