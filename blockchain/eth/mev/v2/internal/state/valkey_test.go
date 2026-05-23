package state

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"mevrelayv2/internal/model"
)

func TestRecordRoundTripIncludesEconomicFields(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	rec := model.BundleRecord{
		ID:                "bundle-1",
		BundleHash:        "0xabc",
		ClientID:          "client-1",
		RegionID:          "region-1",
		State:             model.StateQueued,
		RetryCount:        2,
		Score:             3.5,
		ProfitEth:         0.75,
		Reason:            "queued",
		Terminal:          "",
		Version:           4,
		Sequence:          5,
		CreatedAt:         now,
		UpdatedAt:         now.Add(time.Millisecond),
		QueuedAt:          now.Add(2 * time.Millisecond),
		CompletedAt:       now.Add(3 * time.Millisecond),
		DeadlineAt:        now.Add(5 * time.Second),
		ExpectedValue:     7.25,
		ExpectedCost:      1.5,
		ExpectedServiceMS: 42,
		Priority:          0.88,
	}

	got, ok, err := mapToRecord(stringMap(recordToMap(rec)))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected record")
	}
	if got.DeadlineAt.UnixNano() != rec.DeadlineAt.UnixNano() {
		t.Fatalf("deadline mismatch: got=%v want=%v", got.DeadlineAt, rec.DeadlineAt)
	}
	if got.ExpectedValue != rec.ExpectedValue || got.ExpectedCost != rec.ExpectedCost {
		t.Fatalf("economic mismatch: got=%+v want=%+v", got, rec)
	}
	if got.ExpectedServiceMS != rec.ExpectedServiceMS {
		t.Fatalf("service mismatch: got=%d want=%d", got.ExpectedServiceMS, rec.ExpectedServiceMS)
	}
	if got.Priority != rec.Priority {
		t.Fatalf("priority mismatch: got=%v want=%v", got.Priority, rec.Priority)
	}
}

func stringMap(src map[string]interface{}) map[string]string {
	out := make(map[string]string, len(src))
	for k, v := range src {
		switch x := v.(type) {
		case string:
			out[k] = x
		case []byte:
			out[k] = string(x)
		case int:
			out[k] = strconv.Itoa(x)
		case int64:
			out[k] = strconv.FormatInt(x, 10)
		case float64:
			out[k] = strconv.FormatFloat(x, 'f', -1, 64)
		default:
			out[k] = fmt.Sprint(x)
		}
	}
	return out
}
