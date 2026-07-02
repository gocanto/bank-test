package domain

import (
	"github.com/oullin/workflow/store"
	oworkflow "github.com/oullin/workflow/workflow"
)

const (
	StateOpen   = "open"
	StateClosed = "closed"
)

func (b *Bill) GetState() string {
	if b == nil {
		return ""
	}

	return b.State
}

func (b *Bill) SetState(state string) {
	if b != nil {
		b.State = state
	}
}

func billStateMachine() (*oworkflow.StateMachine[*Bill], error) {
	definition, err := oworkflow.NewDefinitionBuilder().
		AddPlace(StateOpen).
		AddPlace(StateClosed).
		SetInitialPlaces(StateOpen).
		AddTransition("close", []string{StateOpen}, []string{StateClosed}).
		Build()

	if err != nil {
		return nil, err
	}

	return oworkflow.NewStateMachine("bill", definition, &store.SingleState[*Bill]{
		Getter: (*Bill).GetState,
		Setter: (*Bill).SetState,
	}, nil)
}
