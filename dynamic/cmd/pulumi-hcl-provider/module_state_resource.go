package main

import (
	"context"
	"fmt"
	"os"
	"reflect"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	moduleStateResourceType = "hcl:index:ModuleState"
	moduleStateResourceName = "moduleState"
	moduleStateResourceId   = "currentModuleState"
)

type ModuleStateResource struct {
	pulumi.CustomResourceState

	// There is a "state" output of type string but we do not model it here.
}

type ModuleStateResourceArgs struct{}

func (ModuleStateResourceArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*ModuleStateResourceArgs)(nil)).Elem()
}

func NewModuleStateResource(ctx *pulumi.Context, opts ...pulumi.ResourceOption) (*ModuleStateResource, error) {
	args := &ModuleStateResourceArgs{}
	var resource ModuleStateResource
	err := ctx.RegisterResource(moduleStateResourceType, moduleStateResourceName, args, &resource, opts...)
	if err != nil {
		return nil, fmt.Errorf("RegisterResource failed for ModuleStateResource: %w", err)
	}
	return &resource, nil
}

// The implementation of the ModuleComponentResource life-cycle.
type moduleStateHandler struct {
	oldState *promise[ModuleState]
	newState *promise[ModuleState]
	hc       *provider.HostClient
}

var _ ModuleStateStore = (*moduleStateHandler)(nil)

func newModuleStateHandler(hc *provider.HostClient) *moduleStateHandler {
	return &moduleStateHandler{
		oldState: newPromise[ModuleState](),
		newState: newPromise[ModuleState](),
		hc:       hc,
	}
}

// Blocks until the the old state becomes available. Receives a *ModuleStateResource handle to help make sure that the
// resource was allocated prior to calling this method, so the engine is already processing RegisterResource and looking
// up the state. If this method is called early it would lock up.
func (h *moduleStateHandler) AwaitOldState(*promise[*ModuleStateResource]) ModuleState {
	return h.oldState.await()
}

// Stores the new state once it is known. Panics if called twice.
func (h *moduleStateHandler) SetNewState(st ModuleState) {
	h.newState.fulfill(st)
}

// Check is generic and does not do anything.
func (h *moduleStateHandler) Check(
	ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{
		Inputs:   req.News,
		Failures: nil,
	}, nil
}

// Diff spies on old state from the engine and publishes that so the rest of the system can proceed.
// It also waits on the new state to decide if there are changes or not.
func (h *moduleStateHandler) Diff(
	ctx context.Context,
	req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	oldState := ModuleState{}
	oldState.Unmarshal(req.Olds)
	h.oldState.fulfill(oldState)
	newState := h.newState.await()
	changes := pulumirpc.DiffResponse_DIFF_NONE
	if !newState.Equal(oldState) {
		changes = pulumirpc.DiffResponse_DIFF_SOME
	}
	return &pulumirpc.DiffResponse{Changes: changes}, nil
}

// Create exposes empty old state and returns the new state.
func (h *moduleStateHandler) Create(
	ctx context.Context,
	req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	h.hc.Log(ctx, diag.Error, "", fmt.Sprintf("Create served by PID=%d", os.Getpid()))
	oldState := ModuleState{}
	h.oldState.fulfill(oldState)
	newState := h.newState.await()
	h.hc.Log(ctx, diag.Warning, "", fmt.Sprintf("Creating state as %q", string(newState.rawState)))
	return &pulumirpc.CreateResponse{
		Id:         moduleStateResourceId,
		Properties: newState.Marhsal(),
	}, nil
}

// Update simply returns the new state.
func (h *moduleStateHandler) Update(
	ctx context.Context,
	req *pulumirpc.UpdateRequest,
) (*pulumirpc.UpdateResponse, error) {
	newState := h.newState.await()
	h.hc.Log(ctx, diag.Warning, "", fmt.Sprintf("Updating state to %q", string(newState.rawState)))
	return &pulumirpc.UpdateResponse{
		Properties: newState.Marhsal(),
	}, nil
}

// Delete does not do anything.
func (h *moduleStateHandler) Delete(
	ctx context.Context,
	req *pulumirpc.DeleteRequest,
) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
