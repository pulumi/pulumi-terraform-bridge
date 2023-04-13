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
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	urn "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// The version expected to be specified by GetSchema
const SchemaVersion int32 = 0

func mux(
	host *provider.HostClient, mapping *DispatchTable, pulumiSchema string,
	getMappingHandlers getMappingHandler,
	servers ...rpc.ResourceProviderServer,
) *muxer {
	contract.Assertf(len(servers) > 0, "Cannot instantiate an empty muxer")
	return &muxer{
		host:            host,
		servers:         servers,
		schema:          pulumiSchema,
		mapping:         mapping,
		getMappingByKey: getMappingHandlers,
	}
}

var _ rpc.ResourceProviderServer = ((*muxer)(nil))

type server = rpc.ResourceProviderServer

type muxer struct {
	rpc.UnimplementedResourceProviderServer

	host *provider.HostClient

	mapping *DispatchTable

	schema string

	servers []server

	getMappingByKey map[string]MultiMappingHandler
}

type getMappingHandler = map[string]MultiMappingHandler
type MultiMappingHandler = func(provider string, data [][]byte) ([]byte, error)

func (m *muxer) getFunction(token string) (s server) {
	if i, ok := m.mapping.DispatchFunction(token); ok {
		s = m.servers[i]
	}
	return
}

func (m *muxer) getResource(token string) (s server) {
	if i, ok := m.mapping.DispatchResource(token); ok {
		s = m.servers[i]
	}
	return
}

func (m *muxer) GetSchema(ctx context.Context, req *rpc.GetSchemaRequest) (*rpc.GetSchemaResponse, error) {
	if req.Version != SchemaVersion {
		return nil, fmt.Errorf("Expected schema version %d, got %d",
			SchemaVersion, req.GetVersion())
	}
	return &rpc.GetSchemaResponse{Schema: m.schema}, nil
}

func filterConfig[T any](m *muxer, i int, vars map[string]T) {
	for v := range vars {
		var has bool
		js, _ := m.mapping.DispatchConfig(v)
		for _, j := range js {
			if j == i {
				has = true
				break
			}
		}
		if !has {
			delete(vars, v)
		}
	}
}

// Destructively filter `vars`, removing variables that are not known to apply to server
// `i`.
func (m *muxer) filterConfigVariables(i int, vars map[string]string) {
	filterConfig(m, i, vars)
}

// Destructively filter `args`, removing values that are not known to apply to server `i`.
func (m *muxer) filterConfigArgs(i int, args *structpb.Struct) {
	filterConfig(m, i, args.Fields)
}

func (m *muxer) CheckConfig(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	subs := make([]func() tuple[*rpc.CheckResponse, error], len(m.servers))
	for i, s := range m.servers {
		i, s := i, s
		subs[i] = func() tuple[*rpc.CheckResponse, error] {
			req := proto.Clone(req).(*rpc.CheckRequest)
			m.filterConfigArgs(i, req.Olds)
			m.filterConfigArgs(i, req.News)
			return newTuple(s.CheckConfig(ctx, req))
		}
	}

	inputs := &structpb.Struct{Fields: map[string]*structpb.Value{}}
	failures := []*rpc.CheckFailure{}
	uniqueFailures := map[string]struct{}{}
	var errs multierror.Error
	for i, r := range asyncJoin(subs) {
		if err := r.B; err != nil {
			errs.Errors = append(errs.Errors, err)
			continue
		}

		// Add missing inputs, but don't override existing inputs.
		for k, v := range r.A.GetInputs().GetFields() {
			existingValue, has := inputs.Fields[k]
			if has {
				// If different servers return different check values,
				// signal an error. We don't know how to deal with that.
				if proto.Equal(existingValue, v) {
					continue
				}
				return nil, fmt.Errorf("config value '%s' mismatch between servers %d and a previous server",
					k, i)
			}
			inputs.Fields[k] = v
		}

		// Here we de-duplicate errors.
		for _, e := range r.A.GetFailures() {
			s := e.GetProperty() + ":" + e.GetReason()
			if _, has := uniqueFailures[s]; has {
				continue
			}
			uniqueFailures[s] = struct{}{}
			failures = append(failures, e)
		}
	}

	return &rpc.CheckResponse{
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

func (m *muxer) DiffConfig(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	subs := make([]func() tuple[*rpc.DiffResponse, error], len(m.servers))
	for i, s := range m.servers {
		i, s := i, s
		subs[i] = func() tuple[*rpc.DiffResponse, error] {
			req := proto.Clone(req).(*rpc.DiffRequest)
			m.filterConfigArgs(i, req.Olds)
			m.filterConfigArgs(i, req.News)
			r, err := s.DiffConfig(ctx, req)
			return newTuple(r, err)
		}
	}

	var (
		deleteBeforeReplace bool                         // The OR of each server
		replaces            set[string]                  // The AND of each server
		diffs               set[string]                  // The AND of each server, sans replaces
		stables             set[string]                  // The AND of each server, sans replaces and diffs
		changes             rpc.DiffResponse_DiffChanges = rpc.DiffResponse_DIFF_NONE

		errs = new(multierror.Error)
	)

	var (
		detailedDiff    = map[string]*rpc.PropertyDiff{}
		hasDetailedDiff = true
	)

	for _, r := range asyncJoin(subs) {
		if err := r.B; err != nil {
			errs.Errors = append(errs.Errors, err)
			continue
		}

		resp := r.A

		if resp.DeleteBeforeReplace {
			deleteBeforeReplace = true
		}

		if changes == rpc.DiffResponse_DIFF_NONE {
			changes = resp.GetChanges()
		}

		stables.extend(resp.GetStables())
		replaces.extend(resp.GetReplaces())
		diffs.extend(resp.GetDiffs())

		// If any provider is lacking a detailed diff, we don't attempt to combine
		// a detailed and non-detailed diff.
		if !resp.HasDetailedDiff || !hasDetailedDiff {
			hasDetailedDiff = false
			detailedDiff = nil
		} else {
			err := mergeDetailedDiff(detailedDiff, resp.DetailedDiff)
			errs = multierror.Append(errs, err)
		}
	}

	diffs = diffs.setMinus(replaces)
	stables = stables.setMinus(replaces).setMinus(diffs)
	return &rpc.DiffResponse{
		Replaces:            replaces.elements(),
		Stables:             stables.elements(),
		DeleteBeforeReplace: deleteBeforeReplace,
		Changes:             changes,
		Diffs:               diffs.elements(),

		HasDetailedDiff: hasDetailedDiff,
		DetailedDiff:    detailedDiff,
	}, m.muxedErrors(errs)
}

func (m *muxer) Configure(ctx context.Context, req *rpc.ConfigureRequest) (*rpc.ConfigureResponse, error) {
	// Configure determines what the values the provider understands. We take the
	// `and` of configure values.
	subs := make([]func() tuple[*rpc.ConfigureResponse, error], len(m.servers))
	for i, s := range m.servers {
		i, s := i, s
		subs[i] = func() tuple[*rpc.ConfigureResponse, error] {
			req := proto.Clone(req).(*rpc.ConfigureRequest)
			m.filterConfigVariables(i, req.Variables)
			m.filterConfigArgs(i, req.Args)
			r, err := s.Configure(ctx, req)
			return newTuple(r, err)
		}
	}
	response := &rpc.ConfigureResponse{
		AcceptSecrets:   true,
		SupportsPreview: true,
		AcceptResources: true,
		AcceptOutputs:   true,
	}
	errs := new(multierror.Error)
	for _, r := range asyncJoin(subs) {
		if r.B != nil {
			errs.Errors = append(errs.Errors, r.B)
			continue
		}
		response.AcceptOutputs = response.AcceptOutputs && r.A.GetAcceptOutputs()
		response.AcceptResources = response.AcceptResources && r.A.GetAcceptResources()
		response.AcceptSecrets = response.AcceptSecrets && r.A.GetAcceptSecrets()
		response.SupportsPreview = response.SupportsPreview && r.A.GetSupportsPreview()
	}
	return response, m.muxedErrors(errs)
}

type resourceRequest interface {
	GetUrn() string
}

func resourceMethod[T resourceRequest, R any](m *muxer, req T, f func(m server) (R, error)) (R, error) {
	urn := urn.URN(req.GetUrn())
	if !urn.IsValid() {
		return zero[R](), fmt.Errorf("URN '%s' is not valid", string(urn))
	}
	token := string(urn.Type())
	var server rpc.ResourceProviderServer
	if i, ok := m.mapping.DispatchResource(token); ok {
		server = m.servers[i]
	}
	if server == nil {
		return zero[R](), status.Errorf(codes.NotFound, "Resource type '%s' not found.", urn.Type())
	}
	return f(server)
}

func (m *muxer) Invoke(ctx context.Context, req *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	token := req.GetTok()
	var server rpc.ResourceProviderServer
	if i, ok := m.mapping.DispatchFunction(token); ok {
		server = m.servers[i]
	}
	if server == nil {
		return nil, status.Errorf(codes.NotFound, "Invoke '%s' not found.", req.GetTok())
	}
	return server.Invoke(ctx, req)
}

func (m *muxer) StreamInvoke(req *rpc.InvokeRequest, s rpc.ResourceProvider_StreamInvokeServer) error {
	server := m.getFunction(req.GetTok())
	if server == nil {
		return status.Errorf(codes.NotFound, "Invoke '%s' not found.", req.GetTok())
	}
	return server.StreamInvoke(req, s)
}

func (m *muxer) Call(ctx context.Context, req *rpc.CallRequest) (*rpc.CallResponse, error) {
	server := m.getFunction(req.GetTok())
	if server == nil {
		return nil, status.Errorf(codes.NotFound, "Resource Method '%s' not found.", req.GetTok())
	}
	return server.Call(ctx, req)
}

func (m *muxer) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return resourceMethod(m, req, func(m server) (*rpc.CheckResponse, error) {
		return m.Check(ctx, req)
	})
}

func (m *muxer) Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	return resourceMethod(m, req, func(m server) (*rpc.DiffResponse, error) {
		return m.Diff(ctx, req)
	})
}

func (m *muxer) Create(ctx context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	return resourceMethod(m, req, func(m server) (*rpc.CreateResponse, error) {
		return m.Create(ctx, req)
	})
}

func (m *muxer) Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	return resourceMethod(m, req, func(m server) (*rpc.ReadResponse, error) {
		return m.Read(ctx, req)
	})
}

func (m *muxer) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	return resourceMethod(m, req, func(m server) (*rpc.UpdateResponse, error) {
		return m.Update(ctx, req)
	})
}

func (m *muxer) Delete(ctx context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error) {
	return resourceMethod(m, req, func(m server) (*emptypb.Empty, error) {
		return m.Delete(ctx, req)
	})
}

func (m *muxer) Construct(ctx context.Context, req *rpc.ConstructRequest) (*rpc.ConstructResponse, error) {
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

func (m *muxer) GetPluginInfo(ctx context.Context, e *emptypb.Empty) (*rpc.PluginInfo, error) {
	// rpc.PluginInfo only returns the version. We just return the version
	// of the most prominent plugin.
	return m.servers[0].GetPluginInfo(ctx, e)
}

func (m *muxer) Attach(ctx context.Context, req *rpc.PluginAttach) (*emptypb.Empty, error) {
	host, err := provider.NewHostClient(req.GetAddress())
	if err != nil {
		return nil, err
	}
	if err := m.host.Close(); err != nil {
		return nil, err
	}
	// Here we override the underlying host. This should replace the host instance of
	// each subsidiary provider.
	*m.host = *host
	return &emptypb.Empty{}, nil
}

func (m *muxer) GetMapping(ctx context.Context, req *rpc.GetMappingRequest) (*rpc.GetMappingResponse, error) {
	subs := make([]func() tuple[*rpc.GetMappingResponse, error], len(m.servers))
	for i, s := range m.servers {
		i, s := i, s
		subs[i] = func() tuple[*rpc.GetMappingResponse, error] {
			return newTuple(s.GetMapping(ctx, proto.Clone(req).(*rpc.GetMappingRequest)))
		}
	}
	errs := new(multierror.Error)
	results := [][]byte{}
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
		if response.Provider == "" {
			errs.Errors = append(errs.Errors,
				fmt.Errorf("Missing provider name for subprovider %d", i))
			continue
		} else if providerName == "" {
			providerName = response.Provider
		} else if providerName != response.Provider {
			errs = multierror.Append(errs,
				m.Warnf(ctx, "GetMapping",
					"Ignoring Mapping data due to provider name mismatch: %s != %s",
					providerName, response.Provider))
			continue
		}
		results = append(results, response.Data)
	}

	if err := m.muxedErrors(errs); err != nil {
		return nil, err
	}

	switch len(results) {
	case 0:
		// There are no results and some sub-providers implemented the
		// method. This means that no provider responded to this key. We return an
		// empty response.
		return &rpc.GetMappingResponse{}, nil
	case 1:
		// We don't need to worry about merging GetMapping data if there is only one
		// server with valid data.
		return &rpc.GetMappingResponse{
			Provider: providerName,
			Data:     results[0],
		}, nil
	}

	// We need to merge multiple mappings
	combineMapping, found := m.getMappingByKey[req.Key]
	if !found {
		return nil, fmt.Errorf("No multi-mapping handler for GetMapping key '%s'", req.Key)
	}

	data, err := combineMapping(providerName, results)
	if err != nil {
		return nil, fmt.Errorf("Failed to combine '%s' multi-mapping for '%s': %w", req.Key, providerName, err)
	}

	return &rpc.GetMappingResponse{
		Provider: providerName,
		Data:     data,
	}, nil
}

func (m *muxer) Warnf(ctx context.Context, method, msg string, a ...any) error {
	return m.host.Log(ctx, diag.Warning, "", fmt.Sprintf("[muxer/"+method+"] "+msg, a...))
}

// `mergeDetailedDiff` copies values from `src` to  `dst`.
//
// A returned err indicates a conflict between src and dst.
func mergeDetailedDiff(dst map[string]*rpc.PropertyDiff, src map[string]*rpc.PropertyDiff) error {
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
