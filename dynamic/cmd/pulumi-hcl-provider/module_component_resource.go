package main

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Parameterized component resource representing the top-level tree of resources for a particular TF module.
type ModuleComponentResource struct {
	pulumi.ResourceState
}

type ModuleComponentArgs struct{}

func NewModuleComponentResource(
	ctx *pulumi.Context,
	stateStore ModuleStateStore,
	t string,
	name string,
	args *ModuleComponentArgs,
	opts ...pulumi.ResourceOption,
) (*ModuleComponentResource, error) {
	component := ModuleComponentResource{}
	err := ctx.RegisterComponentResource(t, name, &component, opts...)
	if err != nil {
		return nil, fmt.Errorf("RegisterComponentResource failed: %w", err)
	}

	moduleStateResourcePromise := goPromise(func() *ModuleStateResource {
		r, err := NewModuleStateResource(ctx, pulumi.Parent(&component))
		contract.AssertNoErrorf(err, "NewModuleStateResource failed")
		return r
	})

	modState := stateStore.AwaitOldState(moduleStateResourcePromise)

	defer func() {
		// TODO make sure the stored state is modified as needed.
		if modState.IsEmpty() {
			modState.rawState = []byte("42") // just testing
		} else {
			modState.rawState = []byte("43") // just testing
		}
		stateStore.SetNewState(modState)
	}()

	w, err := newTFWorkspace(".tf", modState.rawState)
	if err != nil {
		return nil, fmt.Errorf("newTFWorkspace failed: %w", err)
	}

	if ctx.DryRun() {
		w.Plan()
	} else {
		w.Apply()
	}

	if err := ctx.RegisterResourceOutputs(&component, pulumi.Map{}); err != nil {
		return nil, fmt.Errorf("RegisterResourceOutputs failed: %w", err)
	}

	return &component, nil
}
