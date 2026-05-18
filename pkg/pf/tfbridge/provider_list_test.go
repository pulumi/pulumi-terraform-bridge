package tfbridge

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestListSessionStoreReapExpiredRemovesAndCancels(t *testing.T) {
	t.Parallel()

	store := newListSessionStore(time.Minute)
	t.Cleanup(store.close)

	var canceled atomic.Int32
	session := newListSession(func() { canceled.Add(1) }, 0, "", "")
	session.mu.Lock()
	session.lastAccess = time.Now().Add(-2 * time.Minute)
	session.mu.Unlock()
	store.put("expired", session)

	store.reapExpired(time.Now())

	_, ok := store.get("expired")
	assert.False(t, ok)
	assert.EqualValues(t, 1, canceled.Load())
}

func TestListSessionStoreCloseCancelsAllAndRejectsNewSessions(t *testing.T) {
	t.Parallel()

	store := newListSessionStore(time.Minute)
	var canceled atomic.Int32

	store.put("a", newListSession(func() { canceled.Add(1) }, 0, "", ""))
	store.put("b", newListSession(func() { canceled.Add(1) }, 0, "", ""))
	store.close()

	assert.EqualValues(t, 2, canceled.Load())

	store.put("c", newListSession(func() { canceled.Add(1) }, 0, "", ""))
	assert.EqualValues(t, 3, canceled.Load())

	_, okA := store.get("a")
	_, okB := store.get("b")
	_, okC := store.get("c")
	assert.False(t, okA)
	assert.False(t, okB)
	assert.False(t, okC)
}

func TestListWithContextContinuationPaging(t *testing.T) {
	t.Parallel()

	p := &provider{listSessions: newListSessionStore(time.Minute)}
	t.Cleanup(p.listSessions.close)

	session := newListSession(func() {}, 0, "", "")
	for i := 0; i < 3; i++ {
		ok, err := session.append(context.Background(), &pulumirpc.ListResponse_Result{Id: fmt.Sprintf("id-%d", i+1)})
		require.NoError(t, err)
		require.True(t, ok)
	}
	session.finish(nil)

	token := "tok"
	p.listSessions.put(token, session)

	req := &pulumirpc.ListRequest{ContinuationToken: token, PageSize: 2}

	stream1 := newRecordingListStream(context.Background())
	require.NoError(t, p.ListWithContext(context.Background(), req, stream1))
	results1, cont1 := splitResponses(stream1.sent)
	require.Len(t, results1, 2)
	assert.Equal(t, token, cont1)

	stream2 := newRecordingListStream(context.Background())
	require.NoError(t, p.ListWithContext(context.Background(), req, stream2))
	results2, cont2 := splitResponses(stream2.sent)
	require.Len(t, results2, 1)
	assert.Empty(t, cont2)

	_, ok := p.listSessions.get(token)
	assert.False(t, ok)
}

func TestListWithContextDefaultPageSize(t *testing.T) {
	t.Parallel()

	p := &provider{listSessions: newListSessionStore(time.Minute)}
	t.Cleanup(p.listSessions.close)

	session := newListSession(func() {}, 0, "", "")
	for i := int64(0); i < defaultListPageSize+50; i++ {
		ok, err := session.append(context.Background(), &pulumirpc.ListResponse_Result{Id: fmt.Sprintf("id-%d", i+1)})
		require.NoError(t, err)
		require.True(t, ok)
	}
	session.finish(nil)

	token := "tok-default-size"
	p.listSessions.put(token, session)

	req := &pulumirpc.ListRequest{ContinuationToken: token, PageSize: 0}
	stream := newRecordingListStream(context.Background())
	require.NoError(t, p.ListWithContext(context.Background(), req, stream))
	results, cont := splitResponses(stream.sent)

	assert.Len(t, results, int(defaultListPageSize))
	assert.Equal(t, token, cont)
}

func TestListWithContextCapsOversizedPageSize(t *testing.T) {
	t.Parallel()

	p := &provider{listSessions: newListSessionStore(time.Minute)}
	t.Cleanup(p.listSessions.close)

	session := newListSession(func() {}, 0, "", "")
	for i := 0; i < maxBufferedListResults; i++ {
		ok, err := session.append(context.Background(), &pulumirpc.ListResponse_Result{Id: fmt.Sprintf("id-%d", i+1)})
		require.NoError(t, err)
		require.True(t, ok)
	}

	token := "tok-oversized-page"
	p.listSessions.put(token, session)

	req := &pulumirpc.ListRequest{ContinuationToken: token, PageSize: int64(maxBufferedListResults + 1)}
	stream := newRecordingListStream(context.Background())
	require.NoError(t, p.ListWithContext(context.Background(), req, stream))

	results, cont := splitResponses(stream.sent)
	assert.Len(t, results, maxBufferedListResults)
	assert.Equal(t, token, cont)
}

func TestListWithContextRejectsNegativeBounds(t *testing.T) {
	t.Parallel()

	p := &provider{listSessions: newListSessionStore(time.Minute)}
	t.Cleanup(p.listSessions.close)

	stream := newRecordingListStream(context.Background())
	err := p.ListWithContext(context.Background(), &pulumirpc.ListRequest{Limit: -1}, stream)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limit must be >= 0")

	stream = newRecordingListStream(context.Background())
	err = p.ListWithContext(context.Background(), &pulumirpc.ListRequest{PageSize: -1}, stream)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page_size must be >= 0")
}

func TestListWithContextContinuationRejectsChangedRequest(t *testing.T) {
	t.Parallel()

	p := &provider{listSessions: newListSessionStore(time.Minute)}
	t.Cleanup(p.listSessions.close)

	query, err := listQueryFingerprint(resource.PropertyMap{
		"name": resource.NewStringProperty("original"),
	})
	require.NoError(t, err)

	session := newListSession(func() {}, 0, "pkg:index:Res", query)
	ok, err := session.append(context.Background(), &pulumirpc.ListResponse_Result{Id: "id-1"})
	require.NoError(t, err)
	require.True(t, ok)

	token := "tok-bound"
	p.listSessions.put(token, session)

	stream := newRecordingListStream(context.Background())
	err = p.ListWithContext(context.Background(), &pulumirpc.ListRequest{
		ContinuationToken: token,
		Token:             "pkg:index:Other",
		Query:             mustStruct(t, map[string]any{"name": "original"}),
	}, stream)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "different token")

	stream = newRecordingListStream(context.Background())
	err = p.ListWithContext(context.Background(), &pulumirpc.ListRequest{
		ContinuationToken: token,
		Token:             "pkg:index:Res",
		Limit:             1,
		Query:             mustStruct(t, map[string]any{"name": "original"}),
	}, stream)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "different limit")

	stream = newRecordingListStream(context.Background())
	err = p.ListWithContext(context.Background(), &pulumirpc.ListRequest{
		ContinuationToken: token,
		Token:             "pkg:index:Res",
		Query:             mustStruct(t, map[string]any{"name": "changed"}),
	}, stream)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "different query")
}

func TestListWithContextReturnsTerminalErrorAfterPartialResults(t *testing.T) {
	t.Parallel()

	p := &provider{listSessions: newListSessionStore(time.Minute)}
	t.Cleanup(p.listSessions.close)

	session := newListSession(func() {}, 0, "", "")
	ok, err := session.append(context.Background(), &pulumirpc.ListResponse_Result{Id: "id-1"})
	require.NoError(t, err)
	require.True(t, ok)
	session.finish(errors.New("terminal list failure"))

	token := "tok-terminal-error"
	p.listSessions.put(token, session)

	stream := newRecordingListStream(context.Background())
	err = p.ListWithContext(context.Background(), &pulumirpc.ListRequest{ContinuationToken: token, PageSize: 10}, stream)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal list failure")

	results, cont := splitResponses(stream.sent)
	require.Len(t, results, 1)
	assert.Empty(t, cont)
}

func TestListWithContextResultSendFailureRemovesSession(t *testing.T) {
	t.Parallel()

	p := &provider{listSessions: newListSessionStore(time.Minute)}
	t.Cleanup(p.listSessions.close)

	var canceled atomic.Int32
	session := newListSession(func() { canceled.Add(1) }, 0, "", "")
	ok, err := session.append(context.Background(), &pulumirpc.ListResponse_Result{Id: "id-1"})
	require.NoError(t, err)
	require.True(t, ok)

	token := "tok-result-send-failure"
	p.listSessions.put(token, session)

	sendErr := errors.New("send failed")
	stream := newFailingListStream(context.Background(), 1, sendErr)
	err = p.ListWithContext(context.Background(), &pulumirpc.ListRequest{ContinuationToken: token, PageSize: 1}, stream)
	require.ErrorIs(t, err, sendErr)

	_, ok = p.listSessions.get(token)
	assert.False(t, ok)
	assert.EqualValues(t, 1, canceled.Load())
}

func TestListWithContextContinuationSendFailureRemovesSession(t *testing.T) {
	t.Parallel()

	p := &provider{listSessions: newListSessionStore(time.Minute)}
	t.Cleanup(p.listSessions.close)

	var canceled atomic.Int32
	session := newListSession(func() { canceled.Add(1) }, 0, "", "")
	ok, err := session.append(context.Background(), &pulumirpc.ListResponse_Result{Id: "id-1"})
	require.NoError(t, err)
	require.True(t, ok)

	token := "tok-continuation-send-failure"
	p.listSessions.put(token, session)

	sendErr := errors.New("send failed")
	stream := newFailingListStream(context.Background(), 2, sendErr)
	err = p.ListWithContext(context.Background(), &pulumirpc.ListRequest{ContinuationToken: token, PageSize: 1}, stream)
	require.ErrorIs(t, err, sendErr)

	_, ok = p.listSessions.get(token)
	assert.False(t, ok)
	assert.EqualValues(t, 1, canceled.Load())
}

func TestPopulateListSessionIntegration(t *testing.T) {
	t.Parallel()

	p := &provider{listSessions: newListSessionStore(time.Minute)}
	t.Cleanup(p.listSessions.close)

	session := newListSession(func() {}, 0, "", "")
	token := "tok-integration"
	p.listSessions.put(token, session)

	caller := &fakeListResourceCaller{
		results: []tfprotov6.ListResourceResult{
			{DisplayName: "one", Identity: mustIdentityData(t, "one")},
			{DisplayName: "two", Identity: mustIdentityData(t, "two")},
			{DisplayName: "three", Identity: mustIdentityData(t, "three")},
		},
	}

	p.populateListSession(context.Background(), session, caller,
		resourceHandle{terraformResourceName: "test_resource"}, tftypes.Object{}, &tfprotov6.ResourceIdentitySchema{
			IdentityAttributes: []*tfprotov6.ResourceIdentitySchemaAttribute{
				{Name: "name", Type: tftypes.String, RequiredForImport: true},
			},
		}, nil, terraformListFetchLimit)

	req := &pulumirpc.ListRequest{ContinuationToken: token, PageSize: 2}
	stream := newRecordingListStream(context.Background())
	require.NoError(t, p.ListWithContext(context.Background(), req, stream))

	results, cont := splitResponses(stream.sent)
	require.Len(t, results, 2)
	assert.Equal(t, "one", results[0].Name)
	assert.Equal(t, "two", results[1].Name)
	assert.Equal(t, "one", results[0].Id)
	assert.Equal(t, token, cont)
	require.NotNil(t, caller.listRequest)
	assert.True(t, caller.listRequest.IncludeResource)
}

func TestIdentityDataToID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		attrs  []*tfprotov6.ResourceIdentitySchemaAttribute
		values map[string]tftypes.Value
		want   string
		err    string
	}{
		{
			name: "id attribute wins",
			attrs: []*tfprotov6.ResourceIdentitySchemaAttribute{
				{Name: "id", Type: tftypes.String, RequiredForImport: true},
			},
			values: map[string]tftypes.Value{
				"id": tftypes.NewValue(tftypes.String, "id-value"),
			},
			want: "id-value",
		},
		{
			name: "single attribute",
			attrs: []*tfprotov6.ResourceIdentitySchemaAttribute{
				{Name: "arn", Type: tftypes.String, RequiredForImport: true},
			},
			values: map[string]tftypes.Value{
				"arn": tftypes.NewValue(tftypes.String, "arn-value"),
			},
			want: "arn-value",
		},
		{
			name: "compound attributes sort and join by name",
			attrs: []*tfprotov6.ResourceIdentitySchemaAttribute{
				{Name: "zeta", Type: tftypes.String, RequiredForImport: true},
				{Name: "alpha", Type: tftypes.String, RequiredForImport: true},
			},
			values: map[string]tftypes.Value{
				"zeta":  tftypes.NewValue(tftypes.String, "z"),
				"alpha": tftypes.NewValue(tftypes.String, "a"),
			},
			want: "a,z",
		},
		{
			name: "id attribute wins in compound identity",
			attrs: []*tfprotov6.ResourceIdentitySchemaAttribute{
				{Name: "zeta", Type: tftypes.String, RequiredForImport: true},
				{Name: "id", Type: tftypes.String, RequiredForImport: true},
			},
			values: map[string]tftypes.Value{
				"zeta": tftypes.NewValue(tftypes.String, "z"),
				"id":   tftypes.NewValue(tftypes.String, "id-value"),
			},
			want: "id-value",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			schema := &tfprotov6.ResourceIdentitySchema{IdentityAttributes: tt.attrs}
			got, err := identityDataToID("test_resource", schema, mustIdentityDataWithSchema(t, schema, tt.values))
			if tt.err != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPropertyMapIDUsesConcreteTopLevelID(t *testing.T) {
	t.Parallel()

	id, ok, err := propertyMapID(resource.PropertyMap{
		"id": resource.NewStringProperty("resource-id"),
	})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "resource-id", id)
}

func TestUnmarshalListQueryPreservesUnknowns(t *testing.T) {
	t.Parallel()

	query, err := plugin.MarshalProperties(resource.PropertyMap{
		"name": resource.MakeComputed(resource.NewStringProperty("")),
	}, plugin.MarshalOptions{KeepUnknowns: true})
	require.NoError(t, err)

	props, err := unmarshalListQuery(query)
	require.NoError(t, err)
	assert.True(t, props.ContainsUnknowns())
}

type fakeListResourceCaller struct {
	results     []tfprotov6.ListResourceResult
	err         error
	listRequest *tfprotov6.ListResourceRequest
}

func (f *fakeListResourceCaller) ValidateListResourceConfig(
	_ context.Context, _ *tfprotov6.ValidateListResourceConfigRequest,
) (*tfprotov6.ValidateListResourceConfigResponse, error) {
	return &tfprotov6.ValidateListResourceConfigResponse{}, nil
}

func (f *fakeListResourceCaller) ListResource(
	_ context.Context, req *tfprotov6.ListResourceRequest,
) (*tfprotov6.ListResourceServerStream, error) {
	f.listRequest = req
	if f.err != nil {
		return nil, f.err
	}
	return &tfprotov6.ListResourceServerStream{
		Results: func(yield func(tfprotov6.ListResourceResult) bool) {
			for _, r := range f.results {
				if !yield(r) {
					return
				}
			}
		},
	}, nil
}

type recordingListStream struct {
	ctx  context.Context
	sent []*pulumirpc.ListResponse
}

func newRecordingListStream(ctx context.Context) *recordingListStream {
	return &recordingListStream{ctx: ctx}
}

func (s *recordingListStream) Send(resp *pulumirpc.ListResponse) error {
	s.sent = append(s.sent, resp)
	return nil
}

type failingListStream struct {
	recordingListStream
	failOn int
	err    error
}

func newFailingListStream(ctx context.Context, failOn int, err error) *failingListStream {
	return &failingListStream{
		recordingListStream: recordingListStream{ctx: ctx},
		failOn:              failOn,
		err:                 err,
	}
}

func (s *failingListStream) Send(resp *pulumirpc.ListResponse) error {
	if len(s.sent)+1 == s.failOn {
		return s.err
	}
	s.sent = append(s.sent, resp)
	return nil
}

func (s *recordingListStream) SetHeader(metadata.MD) error { return nil }

func (s *recordingListStream) SendHeader(metadata.MD) error { return nil }

func (s *recordingListStream) SetTrailer(metadata.MD) {}

func (s *recordingListStream) Context() context.Context { return s.ctx }

func (s *recordingListStream) SendMsg(any) error { return nil }

func (s *recordingListStream) RecvMsg(any) error { return nil }

func splitResponses(resps []*pulumirpc.ListResponse) ([]*pulumirpc.ListResponse_Result, string) {
	var results []*pulumirpc.ListResponse_Result
	var continuation string
	for _, r := range resps {
		if rr := r.GetResult(); rr != nil {
			results = append(results, rr)
		}
		if cc := r.GetContinuation(); cc != nil {
			continuation = cc.GetContinuationToken()
		}
	}
	return results, continuation
}

func mustStruct(t *testing.T, values map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(values)
	require.NoError(t, err)
	return s
}

func mustIdentityData(t *testing.T, value string) *tfprotov6.ResourceIdentityData {
	t.Helper()
	schema := &tfprotov6.ResourceIdentitySchema{
		IdentityAttributes: []*tfprotov6.ResourceIdentitySchemaAttribute{
			{Name: "name", Type: tftypes.String, RequiredForImport: true},
		},
	}
	return mustIdentityDataWithSchema(t, schema, map[string]tftypes.Value{
		"name": tftypes.NewValue(tftypes.String, value),
	})
}

func mustIdentityDataWithSchema(
	t *testing.T, schema *tfprotov6.ResourceIdentitySchema, values map[string]tftypes.Value,
) *tfprotov6.ResourceIdentityData {
	t.Helper()
	identityValue := tftypes.NewValue(schema.ValueType(), values)
	dv, err := tfprotov6.NewDynamicValue(schema.ValueType(), identityValue)
	require.NoError(t, err)
	return &tfprotov6.ResourceIdentityData{IdentityData: &dv}
}
