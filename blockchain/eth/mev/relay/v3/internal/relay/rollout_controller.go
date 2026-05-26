package relay

import (
	"sync"
	"time"
)

type RolloutStage string

const (
	RolloutReady    RolloutStage = "ready"
	RolloutDraining RolloutStage = "draining"
	RolloutCutover  RolloutStage = "cutover"
	RolloutRollback RolloutStage = "rollback"
	RolloutBlocked  RolloutStage = "blocked"
)

type RolloutSnapshot struct {
	Stage       RolloutStage
	Version     string
	Reason      string
	StartedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt time.Time
}

type RolloutController struct {
	mu   sync.RWMutex
	snap RolloutSnapshot
}

func NewRolloutController(version string) *RolloutController {
	return &RolloutController{
		snap: RolloutSnapshot{
			Stage:   RolloutReady,
			Version: version,
		},
	}
}

func (c *RolloutController) BeginDrain(reason string) RolloutSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now().UTC()
	c.snap.Stage = RolloutDraining
	c.snap.Reason = reason
	c.snap.UpdatedAt = now
	if c.snap.StartedAt.IsZero() {
		c.snap.StartedAt = now
	}
	return c.snap
}

func (c *RolloutController) BeginCutover(version, reason string) RolloutSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now().UTC()
	c.snap.Stage = RolloutCutover
	c.snap.Version = version
	c.snap.Reason = reason
	c.snap.UpdatedAt = now
	if c.snap.StartedAt.IsZero() {
		c.snap.StartedAt = now
	}
	return c.snap
}

func (c *RolloutController) CompleteCutover(version, reason string) RolloutSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now().UTC()
	c.snap.Stage = RolloutReady
	c.snap.Version = version
	c.snap.Reason = reason
	c.snap.UpdatedAt = now
	c.snap.CompletedAt = now
	return c.snap
}

func (c *RolloutController) Rollback(reason string) RolloutSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now().UTC()
	c.snap.Stage = RolloutRollback
	c.snap.Reason = reason
	c.snap.UpdatedAt = now
	if c.snap.StartedAt.IsZero() {
		c.snap.StartedAt = now
	}
	return c.snap
}

func (c *RolloutController) Block(reason string) RolloutSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snap.Stage = RolloutBlocked
	c.snap.Reason = reason
	c.snap.UpdatedAt = time.Now().UTC()
	return c.snap
}

func (c *RolloutController) Snapshot() RolloutSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snap
}

func (c *RolloutController) AllowWrites() bool {
	return c.Snapshot().Stage == RolloutReady
}

func (c *RolloutController) Ready() bool {
	return c.Snapshot().Stage == RolloutReady
}
