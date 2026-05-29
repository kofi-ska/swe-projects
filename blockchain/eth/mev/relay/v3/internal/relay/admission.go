package relay

import (
	"math"
	"strings"
	"time"

	"mevrelayv3/internal/model"
)

type admissionDecision struct {
	accepted  bool
	reason    string
	deadline  time.Time
	value     float64
	cost      float64
	serviceMS int64
	priority  float64
}

func (s *Service) scoreAdmission(rec model.BundleRecord) admissionDecision {
	req := rec.Request
	now := time.Now().UTC()
	policy := s.policy.Snapshot()
	deadline := now.Add(s.cfg.RequestTimeout)
	if req.MaxTimestamp > 0 {
		if t := time.Unix(req.MaxTimestamp, 0).UTC(); !t.IsZero() {
			deadline = t
		}
	}
	slack := time.Until(deadline)
	if slack <= 0 {
		return admissionDecision{reason: "stale deadline", deadline: deadline}
	}
	txCount := len(req.Txs)
	freshness := clamp(float64(slack)/float64(s.cfg.RequestTimeout), 0, 1)
	value := s.cfg.ValuePerTx * (1 + math.Log1p(float64(txCount))) * freshness
	serviceMS := estimateServiceMS(s.cfg.BackendKind, txCount)
	cost := float64(serviceMS)*s.cfg.CostPerMS + float64(txCount)*s.cfg.CostPerTx
	net := value - cost
	priority := 0.0
	if serviceMS > 0 {
		priority = net/float64(serviceMS) + freshness
	}
	if policy.Confidence < policy.ConfidenceFloor {
		return admissionDecision{reason: "control confidence below floor", deadline: deadline, value: value, cost: cost, serviceMS: serviceMS, priority: priority}
	}
	if slack <= policy.MinDeadlineSlack {
		return admissionDecision{reason: "insufficient slack", deadline: deadline, value: value, cost: cost, serviceMS: serviceMS, priority: priority}
	}
	if time.Duration(serviceMS)*time.Millisecond > slack {
		return admissionDecision{reason: "insufficient slack", deadline: deadline, value: value, cost: cost, serviceMS: serviceMS, priority: priority}
	}
	if net < policy.MinNetValue {
		return admissionDecision{reason: "negative net value", deadline: deadline, value: value, cost: cost, serviceMS: serviceMS, priority: priority}
	}
	return admissionDecision{accepted: true, deadline: deadline, value: value, cost: cost, serviceMS: serviceMS, priority: priority}
}

func estimateServiceMS(kind string, txCount int) int64 {
	kind = strings.ToLower(kind)
	base := int64(100)
	step := int64(20)
	switch kind {
	case "local":
		base = 50
		step = 10
	case "anvil":
		base = 100
		step = 20
	case "sepolia":
		base = 150
		step = 25
	}
	if txCount < 0 {
		txCount = 0
	}
	return base + int64(txCount)*step
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
