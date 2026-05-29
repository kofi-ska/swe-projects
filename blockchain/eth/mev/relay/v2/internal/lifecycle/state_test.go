package lifecycle

import (
	"testing"

	"mevrelayv2/internal/model"
)

func TestReachableAndTransitions(t *testing.T) {
	if !CanTransition(model.StateReceived, model.StateValidated) {
		t.Fatal("expected received -> validated")
	}
	if CanTransition(model.StateCompleted, model.StateReceived) {
		t.Fatal("unexpected completed -> received")
	}
	if !Reachable(model.StateReceived, model.StateCompleted) {
		t.Fatal("expected terminal reachability")
	}
	if !IsTerminal(model.StateRejected) {
		t.Fatal("expected rejected terminal")
	}
}
