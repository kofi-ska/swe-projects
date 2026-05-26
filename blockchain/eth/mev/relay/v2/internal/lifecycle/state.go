package lifecycle

import (
	"fmt"

	"mevrelayv2/internal/model"
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
		model.StateRejected:  {},
	},
	model.StateValidated: {
		model.StateQueued:   {},
		model.StateRejected: {},
	},
	model.StateQueued: {
		model.StateSimulating: {},
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
		model.StateRejected: {},
	},
	model.StateScored: {
		model.StateForwarded: {},
		model.StateRejected:  {},
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

// CanTransition reports whether a state change is legal.
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

// AllowedNext returns the legal next states for a given state.
func AllowedNext(from model.BundleState) []model.BundleState {
	next := allowedTransitions[from]
	out := make([]model.BundleState, 0, len(next))
	for state := range next {
		out = append(out, state)
	}
	return out
}

// IsTerminal reports whether state is terminal.
func IsTerminal(state model.BundleState) bool {
	_, ok := terminalStates[state]
	return ok
}

// ValidateTransition checks whether a state move is allowed.
func ValidateTransition(from, to model.BundleState) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("illegal transition %s -> %s", from, to)
	}
	return nil
}

// Reachable reports whether the target can be reached by legal transitions.
func Reachable(from, target model.BundleState) bool {
	if from == target {
		return true
	}
	seen := map[model.BundleState]struct{}{from: {}}
	queue := []model.BundleState{from}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range AllowedNext(current) {
			if next == target {
				return true
			}
			if _, ok := seen[next]; ok {
				continue
			}
			seen[next] = struct{}{}
			queue = append(queue, next)
		}
	}
	return false
}
