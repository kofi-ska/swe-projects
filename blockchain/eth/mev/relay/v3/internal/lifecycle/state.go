package lifecycle

import (
	"fmt"

	"mevrelayv3/internal/model"
)

var terminalStates = map[model.BundleState]struct{}{
	model.StateForwarded:  {},
	model.StateRejected:   {},
	model.StateDeadLetter: {},
	model.StateCompleted:  {},
}

var allowedTransitions = map[model.BundleState]map[model.BundleState]struct{}{
	model.StateReceived: {
		model.StateValidated: {},
		model.StateDeadLetter: {},
		model.StateRejected:  {},
	},
	model.StateValidated: {
		model.StateQueued:   {},
		model.StateDeadLetter: {},
		model.StateRejected: {},
	},
	model.StateQueued: {
		model.StateSimulating: {},
		model.StateDeadLetter: {},
		model.StateRejected:   {},
	},
	model.StateSimulating: {
		model.StateSimulated:    {},
		model.StateRetryPending: {},
		model.StateRejected:     {},
		model.StateDeadLetter:   {},
	},
	model.StateSimulated: {
		model.StateScored:   {},
		model.StateDeadLetter: {},
		model.StateRejected: {},
	},
	model.StateScored: {
		model.StateForwarded: {},
		model.StateDeadLetter: {},
		model.StateRejected:   {},
	},
	model.StateRetryPending: {
		model.StateQueued:     {},
		model.StateDeadLetter: {},
	},
	model.StateForwarded: {
		model.StatePersisted: {},
	},
	model.StateRejected: {
		model.StatePersisted: {},
	},
	model.StateDeadLetter: {
		model.StatePersisted: {},
	},
	model.StatePersisted: {
		model.StateCompleted: {},
	},
}

func CanTransition(from, to model.BundleState) bool {
	if from == to {
		return false
	}
	next, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	_, ok = next[to]
	return ok
}

func AllowedNext(from model.BundleState) []model.BundleState {
	next := allowedTransitions[from]
	out := make([]model.BundleState, 0, len(next))
	for state := range next {
		out = append(out, state)
	}
	return out
}

func IsTerminal(state model.BundleState) bool {
	_, ok := terminalStates[state]
	return ok
}

func ValidateTransition(from, to model.BundleState) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("illegal transition %s -> %s", from, to)
	}
	return nil
}
