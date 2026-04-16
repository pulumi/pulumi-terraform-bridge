// Copyright 2016-2026, Pulumi Corporation.
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
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/convert"
	"github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/internal/runtypes"
)

const (
	defaultListPageSize       int64 = 100
	listSessionTTL                  = 5 * time.Minute
	listSessionReaperInterval       = 30 * time.Second
	// Use this only when Pulumi requested an unbounded list (limit=0).
	terraformListFetchLimit = math.MaxInt64
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

	var (
		session *listSession
		token   string
		err     error
	)

	if req.GetContinuationToken() == "" {
		session, token, err = p.startListSession(ctx, req)
		if err != nil {
			return err
		}
	} else {
		token = req.GetContinuationToken()
		var ok bool
		session, ok = p.listSessions.get(token)
		if !ok {
			return status.Error(codes.InvalidArgument, "invalid or expired continuation_token")
		}
	}

	if err := session.acquire(ctx); err != nil {
		return err
	}
	defer session.release()

	start, end, page, err := session.preparePage(ctx, pageSize)
	if err != nil {
		if errorsIsContext(err) {
			return status.Error(codes.Canceled, err.Error())
		}
		return err
	}

	for _, item := range page {
		if err := stream.Send(&pulumirpc.ListResponse{
			Response: &pulumirpc.ListResponse_Result_{Result: item},
		}); err != nil {
			return err
		}
	}

	hasMore, terminalErr := session.commit(start, end)
	if !hasMore {
		p.listSessions.remove(token)
		if len(page) == 0 && terminalErr != nil {
			return terminalErr
		}
		return sendContinuation(stream, "")
	}

	return sendContinuation(stream, token)
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
	config, err := listQueryToDynamicValue(req.GetQuery(), configSchema)
	if err != nil {
		return nil, "", status.Errorf(codes.InvalidArgument, "invalid list query: %v", err)
	}

	token, err := newContinuationToken()
	if err != nil {
		return nil, "", err
	}

	// Session lifetime should span multiple RPC calls, so do not tie this context to the current call context.
	listCtx, cancel := context.WithCancel(context.Background())
	session := newListSession(cancel, req.GetLimit())
	p.listSessions.put(token, session)
	var tfLimit int64 = terraformListFetchLimit
	if req.GetLimit() > 0 {
		tfLimit = req.GetLimit()
	}

	go p.populateListSession(listCtx, session, tfListServer, rh, objectType, config, tfLimit)

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

	for result := range listStream.Results {
		if err := p.processDiagnostics(ctx, result.Diagnostics); err != nil {
			session.finish(err)
			return
		}
		item, err := p.convertListResult(ctx, rh, objectType, result)
		if err != nil {
			session.finish(err)
			return
		}
		if !session.append(item) {
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
		stateMap, err = transformFromState(ctx, rh, stateMap)
		if err != nil {
			return nil, fmt.Errorf("terraform list result could not be transformed: %w", err)
		}

		id, err := extractID(ctx, rh.terraformResourceName, rh.pulumiResourceInfo, stateMap)
		if err != nil {
			return nil, fmt.Errorf("terraform list result does not contain a valid id: %w", err)
		}
		item.Id = string(id)
	} else if result.Identity != nil {
		id, err := identityDataToID(result.Identity)
		if err != nil {
			return nil, fmt.Errorf("terraform list result identity could not be converted to id: %w", err)
		}
		item.Id = id
	} else {
		return nil, status.Error(codes.Internal, "terraform list result was missing both resource and identity")
	}

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

	// Fallback for providers that use the managed resource schema for list queries.
	if schemaResp.ResourceSchemas != nil {
		if schema, ok := schemaResp.ResourceSchemas[tfResourceName]; ok {
			return schema, nil
		}
	}

	return nil, status.Errorf(codes.Unimplemented,
		"terraform provider does not expose a list schema for %q", tfResourceName)
}

func listQueryToDynamicValue(query *structpb.Struct, schema *tfprotov6.Schema) (*tfprotov6.DynamicValue, error) {
	queryMap := map[string]any{}
	if query != nil {
		queryMap = query.AsMap()
	}
	queryJSON, err := json.Marshal(queryMap)
	if err != nil {
		return nil, err
	}

	valueType := schema.ValueType()
	value, err := tftypes.ValueFromJSON(queryJSON, valueType) //nolint:staticcheck
	if err != nil {
		return nil, err
	}
	dv, err := tfprotov6.NewDynamicValue(valueType, value)
	if err != nil {
		return nil, err
	}
	return &dv, nil
}

func identityDataToID(data *tfprotov6.ResourceIdentityData) (string, error) {
	if data == nil || data.IdentityData == nil {
		return "", fmt.Errorf("identity data is empty")
	}
	value, err := data.IdentityData.Unmarshal(tftypes.DynamicPseudoType)
	if err != nil {
		return "", err
	}
	dv, err := tfprotov6.NewDynamicValue(tftypes.DynamicPseudoType, value)
	if err != nil {
		return "", err
	}
	// Keep IDs stable and opaque while preserving identity data.
	return "identity:" + base64.RawURLEncoding.EncodeToString(dv.MsgPack), nil
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
	maxResults int64
	next       int64
	items      []*pulumirpc.ListResponse_Result
	done       bool
	err        error
}

func newListSession(cancel context.CancelFunc, maxResults int64) *listSession {
	s := &listSession{
		cancel:     cancel,
		lastAccess: time.Now(),
		maxResults: maxResults,
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

func (s *listSession) lock() {
	s.mu.Lock()
	if !s.done {
		s.lastAccess = time.Now()
	}
}

func (s *listSession) unlock() {
	s.mu.Unlock()
}

func (s *listSession) append(item *pulumirpc.ListResponse_Result) bool {
	s.lock()
	defer s.unlock()
	if s.maxResults > 0 && int64(len(s.items)) >= s.maxResults {
		s.cond.Broadcast()
		return false
	}
	s.items = append(s.items, item)
	s.cond.Broadcast()
	return true
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
	s.cancel()
}

func (s *listSession) acquire(ctx context.Context) error {
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
	s.lock()
	defer s.unlock()
	s.inUse = false
	s.cond.Broadcast()
}

func (s *listSession) preparePage(
	ctx context.Context, limit int64,
) (int64, int64, []*pulumirpc.ListResponse_Result, error) {
	s.lock()
	defer s.unlock()

	start := s.next
	stop := contextWakeup(ctx, s.cond)
	defer stop()

	for {
		if ctx.Err() != nil {
			return 0, 0, nil, ctx.Err()
		}
		available := s.availableAfterLocked(start)
		if available >= limit || s.done {
			break
		}
		s.cond.Wait()
	}

	if s.availableAfterLocked(start) == 0 && s.done && s.err != nil {
		return 0, 0, nil, s.err
	}

	end := start + limit
	visibleLen := s.visibleLenLocked()
	if end > visibleLen {
		end = visibleLen
	}
	page := append([]*pulumirpc.ListResponse_Result(nil), s.items[int(start):int(end)]...)
	return start, end, page, nil
}

func (s *listSession) commit(start, end int64) (hasMore bool, terminalErr error) {
	s.lock()
	defer s.unlock()

	// Invariant while held by acquire/release, but keep defensive behavior.
	if s.next != start {
		start = s.next
	}
	if end < start {
		end = start
	}
	s.next = end

	visibleLen := s.visibleLenLocked()
	hasMore = s.next < visibleLen || (!s.done && (s.maxResults == 0 || s.next < s.maxResults))
	if !hasMore && s.next >= visibleLen && s.err != nil {
		terminalErr = s.err
	}
	return hasMore, terminalErr
}

func (s *listSession) visibleLenLocked() int64 {
	n := int64(len(s.items))
	if s.maxResults > 0 && n > s.maxResults {
		return s.maxResults
	}
	return n
}

func (s *listSession) availableAfterLocked(start int64) int64 {
	visibleLen := s.visibleLenLocked()
	if start >= visibleLen {
		return 0
	}
	return visibleLen - start
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
