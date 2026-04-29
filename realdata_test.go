package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestRealDataImport(t *testing.T) {
	f, err := os.Open("/tmp/health-sample/HealthAutoExport-2025-12-01-2026-03-01.json")
	if err != nil {
		t.Skip("sample data not available:", err)
	}
	defer f.Close()

	var payload IngestPayload
	if err := json.NewDecoder(f).Decode(&payload); err != nil {
		t.Fatal("failed to parse real data:", err)
	}

	env := newTestEnv(t)

	// Save metrics
	metricCount, err := saveMetrics(env.db, payload.Data.Metrics)
	if err != nil {
		t.Fatal("failed to save metrics:", err)
	}
	t.Logf("Saved %d metric data points from %d metric types", metricCount, len(payload.Data.Metrics))

	// Save workouts
	wCount, rCount, err := saveWorkouts(env.db, payload.Data.Workouts)
	if err != nil {
		t.Fatal("failed to save workouts:", err)
	}
	t.Logf("Saved %d workouts and %d routes", wCount, rCount)

	// Verify data is queryable
	steps, err := queryMetrics(env.db, "step_count", "", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("step_count rows: %d", len(steps))

	hr, err := queryMetrics(env.db, "heart_rate", "", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("heart_rate rows: %d", len(hr))

	sleep, err := queryMetrics(env.db, "sleep_analysis", "", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("sleep_analysis rows: %d", len(sleep))

	workouts, err := queryWorkouts(env.db, "", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("workouts: %d", len(workouts))

	if len(workouts) == 0 {
		t.Fatal("expected workouts")
	}

	// Check a workout detail
	detail, err := queryWorkoutDetail(env.db, workouts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail == nil {
		t.Fatal("expected workout detail")
	}
}
