// Copyright 2026, Pulumi Corporation.
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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
	shim "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfshim"
)

const (
	defaultListPageSize       int64 = 100
	listSessionTTL                  = 5 * time.Minute
	listSessionReaperInterval       = 30 * time.Second
	// Use this only when Pulumi requested an unbounded list (limit=0).
	terraformListFetchLimit = math.MaxInt64
	maxBufferedListResults  = 4 * int(defaultListPageSize)
)

func (p *provider) ListWithContext(
	ctx context.Context,
	req *pulumirpc.ListRequest,
	stream grpc.ServerStreamingServer[pulumirpc.ListResponse],
) error {
	ctx = p.initLogging(ctx, p.logSink, "")

	if req.GetLimit() < 0 {
		return status.Error(codes.InvalidArgument, "limit must be >= 0")
	}
	if req.GetPageSize() < 0 {
		return status.Error(codes.InvalidArgument, "page_size must be >= 0")
	}

	pageSize := req.GetPageSize()
	if pageSize <= 0 {
		pageSize = defaultListPageSize
	}
	pageSize = min(pageSize, int64(maxBufferedListResults))

	var (
		session    *listSession
		token      string
		err        error
		newSession bool
	)

	if req.GetContinuationToken() == "" {
		newSession = true
		session, token, err = p.startListSession(ctx, req)
		if err != nil {
			return err
		}
		if session.computed {
			return stream.Send(&pulumirpc.ListResponse{
				Response: &pulumirpc.ListResponse_Computed_{
					Computed: &pulumirpc.ListResponse_Computed{},
				},
			})
		}
	} else {
		token = req.GetContinuationToken()
		var ok bool
		session, ok = p.listSessions.get(token)
		if !ok {
			return status.Error(codes.InvalidArgument, "invalid or expired continuation_token")
		}
		if err := session.validateContinuation(req); err != nil {
			return err
		}
	}

	if err := session.acquire(ctx); err != nil {
		return err
	}
	defer session.release()

	start, end, page, err := session.preparePage(ctx, pageSize)
	if err != nil {
		if errorsIsContext(err) {
			if newSession {
				p.listSessions.remove(token)
			}
			return status.Error(codes.Canceled, err.Error())
		}
		p.listSessions.remove(token)
		return err
	}

	for _, item := range page {
		if err := stream.Send(&pulumirpc.ListResponse{
			Response: &pulumirpc.ListResponse_Result_{Result: item},
		}); err != nil {
			p.listSessions.remove(token)
			return err
		}
	}

	hasMore, terminalErr := session.commit(start, end)
	if terminalErr != nil {
		p.listSessions.remove(token)
		return terminalErr
	}
	if !hasMore {
		p.listSessions.remove(token)
		return sendContinuation(stream, "")
	}

	if err := sendContinuation(stream, token); err != nil {
		p.listSessions.remove(token)
		return err
	}
	return nil
}

func (p *provider) startListSession(
	ctx context.Context, req *pulumirpc.ListRequest,
) (*listSession, string, error) {
	if req.GetToken() == "" {
		return nil, "", status.Error(codes.InvalidArgument, "token is required")
	}

	tfListServer, ok := p.tfServer.(tfprotov6.ListResourceServer)
	if !ok {
		return nil, "", status.Error(codes.Unimplemented, "terraform provider does not implement list resources")
	}

	tfNameOrRenamed, err := p.terraformResourceNameOrRenamedEntity(tokens.Type(req.GetToken()))
	if err != nil {
		return nil, "", status.Error(codes.InvalidArgument, err.Error())
	}
	rh, objectType, err := p.listResourceHandle(ctx, tfNameOrRenamed, tokens.Type(req.GetToken()))
	if err != nil {
		return nil, "", status.Error(codes.InvalidArgument, err.Error())
	}

	configSchema, err := p.listConfigSchema(ctx, rh.terraformResourceName)
	if err != nil {
		return nil, "", err
	}
	query, config, err := p.listQueryToDynamicValue(req.GetQuery(), configSchema, rh.terraformResourceName)
	if err != nil {
		return nil, "", status.Errorf(codes.InvalidArgument, "invalid list query: %v", err)
	}
	if query.computed {
		return newComputedListSession(), "", nil
	}

	validateResp, err := tfListServer.ValidateListResourceConfig(ctx, &tfprotov6.ValidateListResourceConfigRequest{
		TypeName: rh.terraformResourceName,
		Config:   config,
	})
	if err != nil {
		return nil, "", fmt.Errorf("error calling ValidateListResourceConfig: %w", err)
	}
	if err := p.processDiagnostics(ctx, validateResp.Diagnostics); err != nil {
		return nil, "", err
	}

	token, err := newContinuationToken()
	if err != nil {
		return nil, "", err
	}

	// Session lifetime should span multiple RPC calls, so do not tie this context to the current call context.
	listCtx, cancel := context.WithCancel(context.Background())
	session := newListSession(cancel, req.GetLimit(), req.GetToken(), query.fingerprint)
	p.listSessions.put(token, session)
	var tfLimit int64 = terraformListFetchLimit
	if req.GetLimit() > 0 {
		tfLimit = req.GetLimit()
	}

	go p.populateListSession(listCtx, session, tfListServer, rh, objectType, config, tfLimit,
		func(ctx context.Context) (*tfprotov6.ResourceIdentitySchema, error) {
			return p.resourceIdentitySchema(ctx, rh.terraformResourceName)
		})

	return session, token, nil
}

func (p *provider) populateListSession(
	ctx context.Context,
	session *listSession,
	tfListServer tfprotov6.ListResourceServer,
	rh resourceHandle,
	objectType tftypes.Object,
	config *tfprotov6.DynamicValue,
	tfLimit int64,
	loadIdentitySchema func(context.Context) (*tfprotov6.ResourceIdentitySchema, error),
) {
	listStream, err := tfListServer.ListResource(ctx, &tfprotov6.ListResourceRequest{
		TypeName:        rh.terraformResourceName,
		Config:          config,
		IncludeResource: true,
		Limit:           tfLimit,
	})
	if err != nil {
		session.finish(err)
		return
	}

	var (
		identitySchema       *tfprotov6.ResourceIdentitySchema
		identitySchemaLoaded bool
		identitySchemaErr    error
	)
	getIdentitySchema := func(ctx context.Context) (*tfprotov6.ResourceIdentitySchema, error) {
		if !identitySchemaLoaded {
			identitySchema, identitySchemaErr = loadIdentitySchema(ctx)
			identitySchemaLoaded = true
		}
		return identitySchema, identitySchemaErr
	}

	for result := range listStream.Results {
		if err := p.processDiagnostics(ctx, result.Diagnostics); err != nil {
			session.finish(err)
			return
		}
		item, err := p.convertListResult(ctx, rh, objectType, getIdentitySchema, result)
		if err != nil {
			session.finish(err)
			return
		}
		ok, err := session.append(ctx, item)
		if err != nil {
			session.finish(err)
			return
		}
		if !ok {
			session.finish(nil)
			return
		}
	}

	session.finish(nil)
}

func (p *provider) convertListResult(
	ctx context.Context,
	rh resourceHandle,
	objectType tftypes.Object,
	getIdentitySchema func(context.Context) (*tfprotov6.ResourceIdentitySchema, error),
	result tfprotov6.ListResourceResult,
) (*pulumirpc.ListResponse_Result, error) {
	item := &pulumirpc.ListResponse_Result{Name: result.DisplayName}

	if result.Resource != nil {
		stateValue, err := result.Resource.Unmarshal(objectType)
		if err != nil {
			return nil, fmt.Errorf("terraform list result could not be unmarshaled: %w", err)
		}
		stateMap, err := convert.DecodePropertyMap(ctx, rh.decoder, stateValue)
		if err != nil {
			return nil, fmt.Errorf("terraform list result could not be decoded: %w", err)
		}
		if id, ok, err := propertyMapID(stateMap); ok && err == nil {
			item.Id = id
			return item, nil
		} else if err != nil && result.Identity == nil {
			return nil, fmt.Errorf("terraform list result id could not be stringified: %w", err)
		}
	}

	if result.Identity == nil {
		return nil, status.Errorf(codes.Internal,
			"terraform list result for %q has no top-level id and no identity data", rh.terraformResourceName)
	}
	identitySchema, err := getIdentitySchema(ctx)
	if err != nil {
		return nil, err
	}
	id, err := identityDataToID(rh.terraformResourceName, identitySchema, result.Identity)
	if err != nil {
		return nil, fmt.Errorf("terraform list result identity could not be converted to id: %w", err)
	}
	item.Id = id
	return item, nil
}

func (p *provider) listResourceHandle(
	ctx context.Context,
	tfNameOrRenamedEntity string,
	token tokens.Type,
) (resourceHandle, tftypes.Object, error) {
	rt := runtypes.TypeOrRenamedEntityName(tfNameOrRenamedEntity)
	if !p.resources.Has(rt) {
		return resourceHandle{}, tftypes.Object{},
			fmt.Errorf("[pf/tfbridge] unknown resource type: %q", tfNameOrRenamedEntity)
	}

	schema := p.resources.Schema(rt)
	objectType, ok := schema.Type(ctx).(tftypes.Object)
	if !ok {
		return resourceHandle{}, tftypes.Object{},
			fmt.Errorf("[pf/tfbridge] resource %q schema is not an object", tfNameOrRenamedEntity)
	}

	decoder, err := p.encoding.NewResourceDecoder(tfNameOrRenamedEntity, objectType)
	if err != nil {
		return resourceHandle{}, tftypes.Object{}, fmt.Errorf("failed to prepare resource decoder: %w", err)
	}

	rh := resourceHandle{
		token:                 token,
		terraformResourceName: string(schema.TFName()),
		schema:                schema,
		decoder:               decoder,
	}
	if info, ok := p.info.Resources[tfNameOrRenamedEntity]; ok {
		rh.pulumiResourceInfo = info
	}

	return rh, objectType, nil
}

func (p *provider) listConfigSchema(ctx context.Context, tfResourceName string) (*tfprotov6.Schema, error) {
	schemaResp, err := p.tfServer.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		return nil, err
	}
	if err := p.processDiagnostics(ctx, schemaResp.Diagnostics); err != nil {
		return nil, err
	}

	if schemaResp.ListResourceSchemas != nil {
		if schema, ok := schemaResp.ListResourceSchemas[tfResourceName]; ok {
			return schema, nil
		}
	}

	return nil, status.Errorf(codes.Unimplemented,
		"terraform provider does not expose a list schema for %q", tfResourceName)
}

type listQuery struct {
	fingerprint string
	computed    bool
}

func (p *provider) listQueryToDynamicValue(
	query *structpb.Struct, schema *tfprotov6.Schema, tfResourceName string,
) (listQuery, *tfprotov6.DynamicValue, error) {
	queryMap, err := unmarshalListQuery(query)
	if err != nil {
		return listQuery{}, nil, err
	}
	fingerprint, err := listQueryFingerprint(queryMap)
	if err != nil {
		return listQuery{}, nil, err
	}
	if queryMap.ContainsUnknowns() {
		return listQuery{fingerprint: fingerprint, computed: true}, nil, nil
	}

	valueType := schema.ValueType()
	objectType, ok := valueType.(tftypes.Object)
	if !ok {
		return listQuery{}, nil, fmt.Errorf("list config schema for %q is not an object", tfResourceName)
	}

	listResourcesProvider, ok := p.schemaOnlyProvider.(interface{ ListResourcesMap() shim.ResourceMap })
	if !ok {
		return listQuery{}, nil, status.Errorf(codes.Unimplemented,
			"terraform provider does not expose list schemas")
	}
	listResource, ok := listResourcesProvider.ListResourcesMap().GetOk(tfResourceName)
	if !ok {
		return listQuery{}, nil, status.Errorf(codes.Unimplemented,
			"terraform provider does not expose a list schema for %q", tfResourceName)
	}
	encoder, err := convert.NewObjectEncoder(convert.ObjectSchema{
		SchemaMap: listResource.Schema(),
		Object:    &objectType,
	})
	if err != nil {
		return listQuery{}, nil, err
	}
	dv, err := convert.EncodePropertyMapToDynamic(encoder, objectType, queryMap)
	if err != nil {
		return listQuery{}, nil, err
	}
	return listQuery{fingerprint: fingerprint}, dv, nil
}

func unmarshalListQuery(query *structpb.Struct) (resource.PropertyMap, error) {
	if query != nil {
		return plugin.UnmarshalProperties(query, plugin.MarshalOptions{
			Label:        "list query",
			KeepUnknowns: true,
			SkipNulls:    true,
			RejectAssets: true,
		})
	}
	return resource.PropertyMap{}, nil
}

func listQueryFingerprint(query resource.PropertyMap) (string, error) {
	value, err := json.Marshal(canonicalPropertyMap(query))
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func canonicalPropertyMap(pm resource.PropertyMap) map[string]any {
	result := map[string]any{}
	for _, key := range pm.StableKeys() {
		result[string(key)] = canonicalPropertyValue(pm[key])
	}
	return result
}

func canonicalPropertyValue(v resource.PropertyValue) any {
	switch {
	case v.IsComputed() || (v.IsOutput() && !v.OutputValue().Known):
		return map[string]any{"unknown": true}
	case v.IsOutput():
		return canonicalPropertyValue(v.OutputValue().Element)
	case v.IsSecret():
		return canonicalPropertyValue(v.SecretValue().Element)
	case v.IsArray():
		values := make([]any, len(v.ArrayValue()))
		for i, elem := range v.ArrayValue() {
			values[i] = canonicalPropertyValue(elem)
		}
		return values
	case v.IsObject():
		return canonicalPropertyMap(v.ObjectValue())
	default:
		return v.Mappable()
	}
}

func (p *provider) resourceIdentitySchema(
	ctx context.Context, tfResourceName string,
) (*tfprotov6.ResourceIdentitySchema, error) {
	resp, err := p.tfServer.GetResourceIdentitySchemas(ctx, &tfprotov6.GetResourceIdentitySchemasRequest{})
	if err != nil {
		return nil, err
	}
	if err := p.processDiagnostics(ctx, resp.Diagnostics); err != nil {
		return nil, err
	}
	if resp.IdentitySchemas == nil {
		return nil, nil
	}
	return resp.IdentitySchemas[tfResourceName], nil
}

func propertyMapID(stateMap resource.PropertyMap) (string, bool, error) {
	idValue, ok := stateMap[resource.PropertyKey("id")]
	if !ok || idValue.IsNull() {
		return "", false, nil
	}
	id, err := stringifyPropertyValue(idValue)
	return id, true, err
}

func stringifyPropertyValue(v resource.PropertyValue) (string, error) {
	switch {
	case v.IsComputed() || (v.IsOutput() && !v.OutputValue().Known):
		return "", fmt.Errorf("value is unknown")
	case v.IsOutput():
		return stringifyPropertyValue(v.OutputValue().Element)
	case v.IsSecret():
		return stringifyPropertyValue(v.SecretValue().Element)
	case v.IsString():
		return v.StringValue(), nil
	case v.IsBool():
		return strconv.FormatBool(v.BoolValue()), nil
	case v.IsNumber():
		return strconv.FormatFloat(v.NumberValue(), 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("unsupported id value type %s", v.TypeString())
	}
}

func identityDataToID(
	tfResourceName string, schema *tfprotov6.ResourceIdentitySchema, data *tfprotov6.ResourceIdentityData,
) (string, error) {
	if data == nil || data.IdentityData == nil {
		return "", fmt.Errorf("identity data is empty")
	}
	if schema == nil {
		return "", fmt.Errorf("terraform resource %q does not expose an identity schema", tfResourceName)
	}
	value, err := data.IdentityData.Unmarshal(schema.ValueType())
	if err != nil {
		return "", err
	}
	values := map[string]tftypes.Value{}
	if err := value.As(&values); err != nil {
		return "", err
	}
	if len(values) == 0 {
		return "", fmt.Errorf("terraform resource %q identity data is empty", tfResourceName)
	}

	if idValue, ok := values["id"]; ok {
		return stringifyTerraformValue(idValue)
	}

	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	parts := make([]string, len(names))
	for i, name := range names {
		part, err := stringifyTerraformValue(values[name])
		if err != nil {
			return "", err
		}
		parts[i] = part
	}

	return strings.Join(parts, ","), nil
}

func stringifyTerraformValue(value tftypes.Value) (string, error) {
	if !value.IsFullyKnown() {
		return "", fmt.Errorf("value is unknown")
	}
	if value.IsNull() {
		return "", fmt.Errorf("value is null")
	}
	switch {
	case value.Type().Is(tftypes.String):
		var s string
		if err := value.As(&s); err != nil {
			return "", err
		}
		return s, nil
	case value.Type().Is(tftypes.Bool):
		var b bool
		if err := value.As(&b); err != nil {
			return "", err
		}
		return strconv.FormatBool(b), nil
	case value.Type().Is(tftypes.Number):
		var n big.Float
		if err := value.As(&n); err != nil {
			return "", err
		}
		return n.Text('f', -1), nil
	default:
		elements, err := stringifyTerraformList(value)
		if err != nil {
			return "", fmt.Errorf("unsupported identity value type %s", value.Type())
		}
		bytes, err := json.Marshal(elements)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}
}

func stringifyTerraformList(value tftypes.Value) ([]string, error) {
	var values []tftypes.Value
	if err := value.As(&values); err != nil {
		return nil, err
	}
	result := make([]string, len(values))
	for i, elem := range values {
		s, err := stringifyTerraformValue(elem)
		if err != nil {
			return nil, err
		}
		result[i] = s
	}
	return result, nil
}

func sendContinuation(stream grpc.ServerStreamingServer[pulumirpc.ListResponse], token string) error {
	return stream.Send(&pulumirpc.ListResponse{
		Response: &pulumirpc.ListResponse_Continuation_{
			Continuation: &pulumirpc.ListResponse_Continuation{ContinuationToken: token},
		},
	})
}

func newContinuationToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

type listSession struct {
	cancel context.CancelFunc

	mu         sync.Mutex
	cond       *sync.Cond
	lastAccess time.Time
	inUse      bool
	// maxResults is the overall ListRequest.limit bound across all pages.
	// A value of 0 means unbounded.
	maxResults       int64
	requestToken     string
	queryFingerprint string
	produced         int64
	consumed         int64
	items            []*pulumirpc.ListResponse_Result
	done             bool
	err              error
	computed         bool
}

func newListSession(cancel context.CancelFunc, maxResults int64, requestToken, queryFingerprint string) *listSession {
	s := &listSession{
		cancel:           cancel,
		lastAccess:       time.Now(),
		maxResults:       maxResults,
		requestToken:     requestToken,
		queryFingerprint: queryFingerprint,
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

func newComputedListSession() *listSession {
	return &listSession{computed: true}
}

func (s *listSession) lock() {
	s.mu.Lock()
	s.lastAccess = time.Now()
}

func (s *listSession) unlock() {
	s.mu.Unlock()
}

func (s *listSession) append(ctx context.Context, item *pulumirpc.ListResponse_Result) (bool, error) {
	s.lock()
	defer s.unlock()
	stop := contextWakeup(ctx, s.cond)
	defer stop()
	for len(s.items) >= maxBufferedListResults && !s.done {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		s.cond.Wait()
	}
	if s.done {
		return false, nil
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if s.maxResults > 0 && s.produced >= s.maxResults {
		return false, nil
	}
	s.items = append(s.items, item)
	s.produced++
	s.cond.Broadcast()
	return true, nil
}

func (s *listSession) finish(err error) {
	s.lock()
	defer s.unlock()
	if s.done {
		return
	}
	s.done = true
	s.err = err
	s.cond.Broadcast()
}

func (s *listSession) close() {
	if s.cancel != nil {
		s.cancel()
	}
	s.finish(context.Canceled)
}

func (s *listSession) acquire(ctx context.Context) error {
	if s.computed {
		return nil
	}
	s.lock()
	defer s.unlock()
	stop := contextWakeup(ctx, s.cond)
	defer stop()

	for s.inUse {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.cond.Wait()
	}
	s.inUse = true
	return nil
}

func (s *listSession) release() {
	if s.computed {
		return
	}
	s.lock()
	defer s.unlock()
	s.inUse = false
	s.cond.Broadcast()
}

func (s *listSession) validateContinuation(req *pulumirpc.ListRequest) error {
	if s.requestToken != "" && req.GetToken() != s.requestToken {
		return status.Error(codes.InvalidArgument, "continuation_token was used with a different token")
	}
	if req.GetLimit() != s.maxResults {
		return status.Error(codes.InvalidArgument, "continuation_token was used with a different limit")
	}
	queryMap, err := unmarshalListQuery(req.GetQuery())
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid list query: %v", err)
	}
	fingerprint, err := listQueryFingerprint(queryMap)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid list query: %v", err)
	}
	if s.queryFingerprint != "" && fingerprint != s.queryFingerprint {
		return status.Error(codes.InvalidArgument, "continuation_token was used with a different query")
	}
	return nil
}

func (s *listSession) preparePage(
	ctx context.Context, limit int64,
) (int64, int64, []*pulumirpc.ListResponse_Result, error) {
	if s.computed {
		return 0, 0, []*pulumirpc.ListResponse_Result{nil}, nil
	}
	s.lock()
	defer s.unlock()

	start := int64(0)
	stop := contextWakeup(ctx, s.cond)
	defer stop()

	for {
		if ctx.Err() != nil {
			return 0, 0, nil, ctx.Err()
		}
		available := int64(len(s.items))
		if available >= limit || s.done {
			break
		}
		s.cond.Wait()
	}

	if len(s.items) == 0 && s.done && s.err != nil {
		return 0, 0, nil, s.err
	}

	end := start + limit
	visibleLen := int64(len(s.items))
	if end > visibleLen {
		end = visibleLen
	}
	page := append([]*pulumirpc.ListResponse_Result(nil), s.items[int(start):int(end)]...)
	return start, end, page, nil
}

func (s *listSession) commit(start, end int64) (hasMore bool, terminalErr error) {
	if s.computed {
		return false, nil
	}
	s.lock()
	defer s.unlock()

	if end < start {
		end = start
	}
	if end > int64(len(s.items)) {
		end = int64(len(s.items))
	}
	if end > 0 {
		copy(s.items, s.items[end:])
		for i := int64(len(s.items)) - end; i < int64(len(s.items)); i++ {
			s.items[i] = nil
		}
		s.items = s.items[:int64(len(s.items))-end]
		s.consumed += end
		s.cond.Broadcast()
	}

	hasMore = len(s.items) > 0 || (!s.done && (s.maxResults == 0 || s.consumed < s.maxResults))
	if !hasMore && len(s.items) == 0 && s.err != nil {
		terminalErr = s.err
	}
	return hasMore, terminalErr
}

func (s *listSession) isExpired(now time.Time, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return now.Sub(s.lastAccess) > ttl
}

func contextWakeup(ctx context.Context, cond *sync.Cond) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			cond.L.Lock()
			cond.Broadcast()
			cond.L.Unlock()
		case <-done:
		}
	}()
	return func() {
		close(done)
	}
}

type listSessionStore struct {
	ttl      time.Duration
	mu       sync.Mutex
	sessions map[string]*listSession
	started  bool
	closed   bool
	stopCh   chan struct{}
}

func newListSessionStore(ttl time.Duration) *listSessionStore {
	return &listSessionStore{
		ttl:      ttl,
		sessions: map[string]*listSession{},
		stopCh:   make(chan struct{}),
	}
}

func (s *listSessionStore) ensureReaper() {
	if s.started || s.closed {
		return
	}
	s.started = true
	go s.reapLoop()
}

func (s *listSessionStore) put(token string, session *listSession) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		session.close()
		return
	}
	defer s.mu.Unlock()
	s.ensureReaper()
	s.sessions[token] = session
}

func (s *listSessionStore) get(token string) (*listSession, bool) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, false
	}
	session, ok := s.sessions[token]
	s.mu.Unlock()
	if !ok {
		return nil, false
	}
	session.lock()
	session.unlock()
	return session, true
}

func (s *listSessionStore) remove(token string) {
	s.mu.Lock()
	session, ok := s.sessions[token]
	if ok {
		delete(s.sessions, token)
	}
	s.mu.Unlock()
	if ok {
		session.close()
	}
}

func (s *listSessionStore) reapLoop() {
	ticker := time.NewTicker(listSessionReaperInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
		}

		s.reapExpired(time.Now())
	}
}

func (s *listSessionStore) reapExpired(now time.Time) {
	var expiredTokens []string

	s.mu.Lock()
	for token, session := range s.sessions {
		if session.isExpired(now, s.ttl) {
			expiredTokens = append(expiredTokens, token)
		}
	}
	s.mu.Unlock()

	for _, token := range expiredTokens {
		s.remove(token)
	}
}

func (s *listSessionStore) close() {
	var sessions map[string]*listSession

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.stopCh)
	sessions = s.sessions
	s.sessions = map[string]*listSession{}
	s.mu.Unlock()

	for _, session := range sessions {
		session.close()
	}
}

func errorsIsContext(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
