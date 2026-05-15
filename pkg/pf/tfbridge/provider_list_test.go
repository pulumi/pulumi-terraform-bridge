package tfbridge

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestListSessionStoreReapExpiredRemovesAndCancels(t *testing.T) {
	t.Parallel()

	store := newListSessionStore(time.Minute)
	t.Cleanup(store.close)

	var canceled atomic.Int32
	session := newListSession(func() { canceled.Add(1) }, 0)
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

	store.put("a", newListSession(func() { canceled.Add(1) }, 0))
	store.put("b", newListSession(func() { canceled.Add(1) }, 0))
	store.close()

	assert.EqualValues(t, 2, canceled.Load())

	store.put("c", newListSession(func() { canceled.Add(1) }, 0))
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

	session := newListSession(func() {}, 0)
	for i := 0; i < 3; i++ {
		ok := session.append(&pulumirpc.ListResponse_Result{Id: fmt.Sprintf("id-%d", i+1)})
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

	session := newListSession(func() {}, 0)
	for i := int64(0); i < defaultListPageSize+50; i++ {
		ok := session.append(&pulumirpc.ListResponse_Result{Id: fmt.Sprintf("id-%d", i+1)})
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

func TestPopulateListSessionIntegration(t *testing.T) {
	t.Parallel()

	p := &provider{listSessions: newListSessionStore(time.Minute)}
	t.Cleanup(p.listSessions.close)

	session := newListSession(func() {}, 0)
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
		resourceHandle{}, tftypes.Object{}, nil, terraformListFetchLimit)

	req := &pulumirpc.ListRequest{ContinuationToken: token, PageSize: 2}
	stream := newRecordingListStream(context.Background())
	require.NoError(t, p.ListWithContext(context.Background(), req, stream))

	results, cont := splitResponses(stream.sent)
	require.Len(t, results, 2)
	assert.Equal(t, "one", results[0].Name)
	assert.Equal(t, "two", results[1].Name)
	assert.Contains(t, results[0].Id, "identity:")
	assert.Equal(t, token, cont)
}

type fakeListResourceCaller struct {
	results []tfprotov6.ListResourceResult
	err     error
}

func (f *fakeListResourceCaller) ValidateListResourceConfig(
	_ context.Context, _ *tfprotov6.ValidateListResourceConfigRequest,
) (*tfprotov6.ValidateListResourceConfigResponse, error) {
	return &tfprotov6.ValidateListResourceConfigResponse{}, nil
}

func (f *fakeListResourceCaller) ListResource(
	_ context.Context, _ *tfprotov6.ListResourceRequest,
) (*tfprotov6.ListResourceServerStream, error) {
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

func mustIdentityData(t *testing.T, value string) *tfprotov6.ResourceIdentityData {
	t.Helper()
	identityType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"name": tftypes.String}}
	identityValue := tftypes.NewValue(identityType, map[string]tftypes.Value{
		"name": tftypes.NewValue(tftypes.String, value),
	})
	dv, err := tfprotov6.NewDynamicValue(tftypes.DynamicPseudoType, identityValue)
	require.NoError(t, err)
	return &tfprotov6.ResourceIdentityData{IdentityData: &dv}
}
