package relay

import "testing"

func TestRolloutControllerTransitions(t *testing.T) {
	c := NewRolloutController("v3")
	if !c.Ready() {
		t.Fatal("expected initial rollout to be ready")
	}
	c.BeginDrain("drain")
	if c.AllowWrites() {
		t.Fatal("expected draining rollout to block writes")
	}
	c.BeginCutover("v4", "cutover")
	if c.Ready() {
		t.Fatal("expected cutover to block readiness")
	}
	c.CompleteCutover("v4", "done")
	if !c.Ready() {
		t.Fatal("expected completed cutover to be ready")
	}
}
