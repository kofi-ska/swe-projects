package relay

import "testing"

func TestRecoveryControllerTransitions(t *testing.T) {
	c := NewRecoveryController()
	if !c.Ready() {
		t.Fatal("expected initial recovery to be ready")
	}
	c.BeginReplay("replay")
	if c.AllowWrites() {
		t.Fatal("expected replay to block writes")
	}
	c.Quarantine("mismatch")
	if c.Ready() {
		t.Fatal("expected quarantine to be not ready")
	}
	c.Validate("ok")
	if !c.Ready() {
		t.Fatal("expected validated recovery to be ready")
	}
}
