package measurement

import (
	"encoding/json"
	"errors"
	"os"
)

func LoadBaseline(path string) (Baseline, error) {
	if path == "" {
		return Baseline{}, errors.New("missing baseline path")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return Baseline{}, err
	}
	var b Baseline
	return b, json.Unmarshal(body, &b)
}

func SaveReport(path string, report Report) error {
	if path == "" {
		return errors.New("missing report path")
	}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func Compare(result Result, baseline Baseline) (bool, []string) {
	if baseline.Name == "" {
		return true, nil
	}
	var reasons []string
	if baseline.Result.P99Latency > 0 && result.P99Latency > baseline.Result.P99Latency+(baseline.Result.P99Latency/5) {
		reasons = append(reasons, "p99 latency regressed")
	}
	if baseline.Result.Successes > 0 && result.Successes < baseline.Result.Successes {
		reasons = append(reasons, "success count regressed")
	}
	if baseline.Result.Rejected > 0 && result.Rejected > baseline.Result.Rejected*2 {
		reasons = append(reasons, "rejection rate regressed")
	}
	if baseline.Result.HealthAfter.Ready && !result.HealthAfter.Ready {
		reasons = append(reasons, "ready state regressed")
	}
	return len(reasons) == 0, reasons
}
