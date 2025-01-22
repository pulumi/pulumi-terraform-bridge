package main

import (
	"bytes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"google.golang.org/protobuf/types/known/structpb"
)

type ModuleState struct {
	rawState []byte
}

func (ms *ModuleState) IsEmpty() bool {
	return len(ms.rawState) == 0
}

func (ms *ModuleState) Equal(other ModuleState) bool {
	return bytes.Equal(ms.rawState, other.rawState)
}

func (ms *ModuleState) Unmarshal(s *structpb.Struct) {
	if s == nil {
		return // empty
	}
	state, ok := s.Fields["state"]
	if !ok {
		return // empty
	}
	stateString := state.GetStringValue()
	ms.rawState = []byte(stateString)
}

func (ms *ModuleState) Marhsal() *structpb.Struct {
	s, err := structpb.NewStruct(map[string]any{
		"state": string(ms.rawState),
	})
	contract.AssertNoErrorf(err, "structpb.NewStruct should not fail")
	return s
}

type ModuleStateStore interface {
	// Blocks until the the old state becomes available. If this method is called early it would lock up - needs to
	// be called after the ModuleStateResource is allocated.
	AwaitOldState(*promise[*ModuleStateResource]) ModuleState

	// Stores the new state once it is known. Panics if called twice.
	SetNewState(ModuleState)
}
