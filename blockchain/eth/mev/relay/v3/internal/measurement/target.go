package measurement

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mevrelayv3/internal/model"
	"mevrelayv3/internal/relay"
	"mevrelayv3/internal/telemetry"
)

type Target interface {
	Submit(context.Context, model.JSONRPCRequest, string, string) (model.BundleRecord, error)
	Health(context.Context) (relay.HealthReport, error)
	Metrics(context.Context) (telemetry.Snapshot, error)
}

type EmbeddedTarget struct {
	Svc *relay.Service
}

func (t EmbeddedTarget) Submit(ctx context.Context, req model.JSONRPCRequest, clientID, regionID string) (model.BundleRecord, error) {
	return t.Svc.SubmitWithIdentity(ctx, req, clientID, regionID)
}

func (t EmbeddedTarget) Health(ctx context.Context) (relay.HealthReport, error) {
	return t.Svc.AssessHealth(ctx), nil
}

func (t EmbeddedTarget) Metrics(context.Context) (telemetry.Snapshot, error) {
	return t.Svc.MetricsSnapshot(), nil
}

type HTTPTarget struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func (t HTTPTarget) client() *http.Client {
	if t.Client != nil {
		return t.Client
	}
	return &http.Client{Timeout: 5 * time.Second}
}

func (t HTTPTarget) Submit(ctx context.Context, req model.JSONRPCRequest, clientID, regionID string) (model.BundleRecord, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return model.BundleRecord{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(t.BaseURL, "/")+"/relay/v3/bundle", bytes.NewReader(body))
	if err != nil {
		return model.BundleRecord{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if t.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+t.Token)
	}
	httpReq.Header.Set("X-Client-ID", clientID)
	httpReq.Header.Set("X-Region-ID", regionID)
	resp, err := t.client().Do(httpReq)
	if err != nil {
		return model.BundleRecord{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return model.BundleRecord{}, fmt.Errorf("submit %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int64  `json:"id"`
		Result  struct {
			BundleID string `json:"bundleId"`
			State    string `json:"state"`
			ShardID  string `json:"shardId"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return model.BundleRecord{}, err
	}
	return model.BundleRecord{ID: out.Result.BundleID, State: model.BundleState(out.Result.State), ShardID: out.Result.ShardID}, nil
}

func (t HTTPTarget) Health(ctx context.Context) (relay.HealthReport, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(t.BaseURL, "/")+"/healthz", nil)
	if err != nil {
		return relay.HealthReport{}, err
	}
	resp, err := t.client().Do(req)
	if err != nil {
		return relay.HealthReport{}, err
	}
	defer resp.Body.Close()
	var out relay.HealthReport
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return relay.HealthReport{}, err
	}
	return out, nil
}

func (t HTTPTarget) Metrics(ctx context.Context) (telemetry.Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(t.BaseURL, "/")+"/metrics", nil)
	if err != nil {
		return telemetry.Snapshot{}, err
	}
	resp, err := t.client().Do(req)
	if err != nil {
		return telemetry.Snapshot{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return telemetry.Snapshot{}, err
	}
	return parsePrometheusMetrics(string(raw)), nil
}

func parsePrometheusMetrics(raw string) telemetry.Snapshot {
	s := telemetry.Snapshot{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		v, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "mevrelay_submitted_total":
			s.Submitted = uint64(v)
		case "mevrelay_accepted_total":
			s.Accepted = uint64(v)
		case "mevrelay_rejected_total":
			s.Rejected = uint64(v)
		case "mevrelay_forwarded_total":
			s.Forwarded = uint64(v)
		case "mevrelay_dead_letters_total":
			s.DeadLetters = uint64(v)
		case "mevrelay_retry_pending_total":
			s.RetryPending = uint64(v)
		case "mevrelay_retry_scheduled_total":
			s.RetryScheduled = uint64(v)
		case "mevrelay_duplicates_total":
			s.Duplicates = uint64(v)
		case "mevrelay_queue_overflow_total":
			s.QueueOverflow = uint64(v)
		case "mevrelay_inflight_limit_total":
			s.InflightLimit = uint64(v)
		case "mevrelay_backend_errors_total":
			s.BackendErrors = uint64(v)
		case "mevrelay_state_errors_total":
			s.StateErrors = uint64(v)
		case "mevrelay_wal_errors_total":
			s.WALErrors = uint64(v)
		case "mevrelay_broker_errors_total":
			s.BrokerErrors = uint64(v)
		case "mevrelay_terminal_errors_total":
			s.TerminalErrors = uint64(v)
		case "mevrelay_stale_authority_total":
			s.StaleAuthority = uint64(v)
		case "mevrelay_wrong_shard_total":
			s.WrongShard = uint64(v)
		case "mevrelay_checkpoint_errors_total":
			s.CheckpointErrors = uint64(v)
		case "mevrelay_policy_adjustments_total":
			s.PolicyAdjustments = uint64(v)
		case "mevrelay_queue_depth":
			s.QueueDepth = uint64(v)
		case "mevrelay_queue_capacity":
			s.QueueCap = uint64(v)
		case "mevrelay_queue_oldest_age_ms":
			s.QueueOldestAgeMS = uint64(v)
		case "mevrelay_queue_net_value":
			s.QueueNetValue = v
		case "mevrelay_retry_debt":
			s.RetryDebt = v
		case "mevrelay_health_state":
			s.HealthStateCode = uint32(v)
		case "mevrelay_recovery_state":
			s.RecoveryStateCode = uint32(v)
		case "mevrelay_rollout_state":
			s.RolloutStateCode = uint32(v)
		case "mevrelay_policy_revision":
			s.PolicyRevision = uint64(v)
		case "mevrelay_policy_pressure":
			s.PolicyPressure = v
		case "mevrelay_policy_confidence":
			s.PolicyConfidence = v
		case "mevrelay_backend_latency_ms":
			s.BackendLatencyMS = uint64(v)
		case "mevrelay_state_latency_ms":
			s.StateLatencyMS = uint64(v)
		case "mevrelay_broker_latency_ms":
			s.BrokerLatencyMS = uint64(v)
		case "mevrelay_wal_latency_ms":
			s.WALLatencyMS = uint64(v)
		case "mevrelay_checkpoint_latency_ms":
			s.CheckpointLatencyMS = uint64(v)
		}
	}
	return s
}
