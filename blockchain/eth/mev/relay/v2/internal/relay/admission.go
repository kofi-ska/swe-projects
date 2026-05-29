package relay

import (
	"math"
	"strings"
	"time"

	"mevrelayv2/internal/backend"
	"mevrelayv2/internal/model"
)

type admissionDecision struct {
	value     float64
	cost      float64
	serviceMS int64
	deadline  time.Time
	slack     time.Duration
	priority  float64
	reason    string
	accepted  bool
}

func (s *Service) scoreAdmission(rec model.BundleRecord) admissionDecision {
	now := time.Now().UTC()
	deadlineAt := time.Unix(rec.Request.MaxTimestamp, 0).UTC()
	if rec.Request.MaxTimestamp <= 0 {
		deadlineAt = now.Add(s.cfg.RequestTimeout)
	}
	count := len(rec.Request.Txs)
	freshness := 1.0
	if s.cfg.RequestTimeout > 0 {
		freshness = clamp01(float64(deadlineAt.Sub(now)) / float64(s.cfg.RequestTimeout))
	}
	value := s.cfg.ValuePerTx * (1 + math.Log1p(float64(count))) * freshness
	serviceMS := s.estimateServiceMS(count)
	cost := float64(serviceMS)*s.cfg.CostPerMS + float64(count)*s.cfg.CostPerTx
	slack := deadlineAt.Sub(now)
	net := value - cost
	if serviceMS <= 0 {
		serviceMS = 1
	}
	priority := (net / float64(serviceMS)) + freshness
	accepted := true
	reason := "admitted"
	if accepted && slack <= s.cfg.MinDeadlineSlack {
		accepted = false
		reason = ErrStaleDeadline.Error()
	}
	if accepted && serviceMS > int64(slack/time.Millisecond) {
		accepted = false
		reason = ErrInsufficientDeadline.Error()
	}
	if accepted && net < s.cfg.MinNetValue {
		accepted = false
		reason = ErrNegativeNetValue.Error()
	}
	return admissionDecision{
		value:     value,
		cost:      cost,
		serviceMS: serviceMS,
		deadline:  deadlineAt,
		slack:     slack,
		priority:  priority,
		reason:    reason,
		accepted:  accepted,
	}
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

func (s *Service) estimateServiceMS(txCount int) int64 {
	base := int64(25)
	perTx := int64(5)
	switch strings.ToLower(s.cfg.BackendKind) {
	case string(backend.KindAnvil):
		base = 40
		perTx = 8
	case string(backend.KindSepolia):
		base = 120
		perTx = 14
	case string(backend.KindLocal):
		base = 10
		perTx = 3
	}
	if txCount < 0 {
		txCount = 0
	}
	return base + perTx*int64(txCount)
}
