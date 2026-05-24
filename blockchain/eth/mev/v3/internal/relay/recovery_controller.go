package relay

import (
	"sync"
	"time"
)

type RecoveryStage string

const (
	RecoveryIdle        RecoveryStage = "idle"
	RecoveryReplaying   RecoveryStage = "replaying"
	RecoveryValidating  RecoveryStage = "validating"
	RecoveryQuarantined RecoveryStage = "quarantined"
	RecoveryRejoined    RecoveryStage = "rejoined"
)

type RecoverySnapshot struct {
	Stage          RecoveryStage
	LastReplay     time.Time
	LastValidated  time.Time
	LastUpdated    time.Time
	LastReason     string
	ReplayCount    uint64
	Quarantined    bool
	RejoinedCount  uint64
	ValidationPass bool
}

type RecoveryController struct {
	mu   sync.RWMutex
	snap RecoverySnapshot
}

func NewRecoveryController() *RecoveryController {
	return &RecoveryController{
		snap: RecoverySnapshot{
			Stage:          RecoveryIdle,
			ValidationPass: true,
		},
	}
}

func (c *RecoveryController) BeginReplay(reason string) RecoverySnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snap.Stage = RecoveryReplaying
	c.snap.LastReplay = time.Now().UTC()
	c.snap.LastUpdated = c.snap.LastReplay
	c.snap.LastReason = reason
	c.snap.ReplayCount++
	c.snap.Quarantined = false
	c.snap.ValidationPass = false
	return c.snap
}

func (c *RecoveryController) BeginValidation(reason string) RecoverySnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snap.Stage = RecoveryValidating
	c.snap.LastUpdated = time.Now().UTC()
	c.snap.LastReason = reason
	return c.snap
}

func (c *RecoveryController) Validate(reason string) RecoverySnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snap.Stage = RecoveryRejoined
	c.snap.LastValidated = time.Now().UTC()
	c.snap.LastUpdated = c.snap.LastValidated
	c.snap.LastReason = reason
	c.snap.ValidationPass = true
	c.snap.Quarantined = false
	c.snap.RejoinedCount++
	return c.snap
}

func (c *RecoveryController) Quarantine(reason string) RecoverySnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snap.Stage = RecoveryQuarantined
	c.snap.LastUpdated = time.Now().UTC()
	c.snap.LastReason = reason
	c.snap.Quarantined = true
	c.snap.ValidationPass = false
	return c.snap
}

func (c *RecoveryController) Snapshot() RecoverySnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snap
}

func (c *RecoveryController) AllowWrites() bool {
	snap := c.Snapshot()
	return !snap.Quarantined && snap.Stage != RecoveryReplaying && snap.Stage != RecoveryValidating
}

func (c *RecoveryController) Ready() bool {
	snap := c.Snapshot()
	return snap.Stage == RecoveryIdle || snap.Stage == RecoveryRejoined
}
