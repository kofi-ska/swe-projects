package lifecycle

import (
	"testing"

	"mevrelayv1/internal/model"
)

func TestValidateTransition(t *testing.T) {
	if err := ValidateTransition(model.StateReceived, model.StateValidated); err != nil {
		t.Fatalf("expected valid transition: %v", err)
	}
	if err := ValidateTransition(model.StateReceived, model.StateSimulating); err == nil {
		t.Fatalf("expected illegal transition")
	}
}

