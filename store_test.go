package main

import (
	"encoding/json"
	"testing"
)

func setupTestDB(t *testing.T) *testEnv {
	t.Helper()
	return newTestEnv(t)
}

// --- JSON Parse Tests ---

func TestParseBaseMetric(t *testing.T) {
	env := setupTestDB(t)
	raw := json.RawMessage(`{"qty": 5000, "date": "2026-01-21 00:00:00 +0900", "source": "Apple Watch"}`)
	if err := upsertBaseMetric(env.db, "step_count", "count", raw); err != nil {
		t.Fatal(err)
	}
	rows, _ := queryMetrics(env.db, "step_count", "", "")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["qty"].(float64) != 5000 {
		t.Errorf("expected qty=5000, got %v", rows[0]["qty"])
	}
}

func TestParseHeartRate(t *testing.T) {
	env := setupTestDB(t)
	raw := json.RawMessage(`{"Min": 60, "Avg": 72.5, "Max": 85, "date": "2026-01-21 00:00:00 +0900", "source": "Apple Watch"}`)
	if err := upsertHeartRate(env.db, "count/min", raw); err != nil {
		t.Fatal(err)
	}
	rows, _ := queryMetrics(env.db, "heart_rate", "", "")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["Avg"].(float64) != 72.5 {
		t.Errorf("expected Avg=72.5, got %v", rows[0]["Avg"])
	}
}

func TestParseBloodPressure(t *testing.T) {
	env := setupTestDB(t)
	raw := json.RawMessage(`{"systolic": 120, "diastolic": 80, "date": "2026-01-21 00:00:00 +0900", "source": "Manual"}`)
	if err := upsertBloodPressure(env.db, "mmHg", raw); err != nil {
		t.Fatal(err)
	}
	rows, _ := queryMetrics(env.db, "blood_pressure", "", "")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["systolic"].(float64) != 120 {
		t.Errorf("expected systolic=120, got %v", rows[0]["systolic"])
	}
}

func TestParseSleep(t *testing.T) {
	env := setupTestDB(t)
	raw := json.RawMessage(`{
		"date": "2026-01-21 00:00:00 +0900",
		"inBedStart": "2026-01-20 23:00:00 +0900",
		"inBedEnd": "2026-01-21 07:00:00 +0900",
		"sleepStart": "2026-01-20 23:30:00 +0900",
		"sleepEnd": "2026-01-21 06:50:00 +0900",
		"core": 3.5, "rem": 1.5, "deep": 1.0, "awake": 0.5, "inBed": 8.0,
		"asleep": 0, "totalSleep": 6.5
	}`)
	if err := upsertSleep(env.db, "hr", raw); err != nil {
		t.Fatal(err)
	}
	rows, _ := queryMetrics(env.db, "sleep_analysis", "", "")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["core"].(float64) != 3.5 {
		t.Errorf("expected core=3.5, got %v", rows[0]["core"])
	}
}

func TestParseMetricWithoutSource(t *testing.T) {
	env := setupTestDB(t)
	raw := json.RawMessage(`{"qty": 500, "date": "2026-01-21 00:00:00 +0900"}`)
	if err := upsertBaseMetric(env.db, "six_minute_walking_test_distance", "m", raw); err != nil {
		t.Fatal(err)
	}
	rows, _ := queryMetrics(env.db, "six_minute_walking_test_distance", "", "")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["source"].(string) != "" {
		t.Errorf("expected empty source, got %q", rows[0]["source"])
	}
}

func TestParseWorkout(t *testing.T) {
	env := setupTestDB(t)
	workouts := []RawWorkout{{
		ID:       "workout-001",
		Name:     "Running",
		Start:    "2026-01-21 06:00:00 +0900",
		End:      "2026-01-21 06:45:00 +0900",
		Duration: 2700,
		ActiveEnergyBurned: &QtyUnits{Qty: 450, Units: "kcal"},
		Distance:           &QtyUnits{Qty: 5.2, Units: "km"},
		Route: []RoutePoint{
			{Latitude: 35.68, Longitude: 139.76, Timestamp: "2026-01-21 06:00:00 +0900"},
		},
	}}
	wc, rc, err := saveWorkouts(env.db, workouts)
	if err != nil {
		t.Fatal(err)
	}
	if wc != 1 || rc != 1 {
		t.Errorf("expected 1 workout + 1 route, got %d + %d", wc, rc)
	}
}

func TestParseEmptyData(t *testing.T) {
	env := setupTestDB(t)
	count, err := saveMetrics(env.db, nil)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	env := setupTestDB(t)
	raw := json.RawMessage(`{invalid}`)
	err := upsertBaseMetric(env.db, "step_count", "count", raw)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- UPSERT Tests ---

func TestUpsertDuplicateBaseMetric(t *testing.T) {
	env := setupTestDB(t)
	raw1 := json.RawMessage(`{"qty": 5000, "date": "2026-01-21 00:00:00 +0900", "source": "Apple Watch"}`)
	raw2 := json.RawMessage(`{"qty": 6000, "date": "2026-01-21 00:00:00 +0900", "source": "Apple Watch"}`)
	upsertBaseMetric(env.db, "step_count", "count", raw1)
	upsertBaseMetric(env.db, "step_count", "count", raw2)
	rows, _ := queryMetrics(env.db, "step_count", "", "")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after upsert, got %d", len(rows))
	}
	if rows[0]["qty"].(float64) != 6000 {
		t.Errorf("expected updated qty=6000, got %v", rows[0]["qty"])
	}
}

func TestUpsertDifferentSource(t *testing.T) {
	env := setupTestDB(t)
	raw1 := json.RawMessage(`{"qty": 5000, "date": "2026-01-21 00:00:00 +0900", "source": "Apple Watch"}`)
	raw2 := json.RawMessage(`{"qty": 3000, "date": "2026-01-21 00:00:00 +0900", "source": "iPhone"}`)
	upsertBaseMetric(env.db, "step_count", "count", raw1)
	upsertBaseMetric(env.db, "step_count", "count", raw2)
	rows, _ := queryMetrics(env.db, "step_count", "", "")
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows for different sources, got %d", len(rows))
	}
}

func TestUpsertDuplicateHeartRate(t *testing.T) {
	env := setupTestDB(t)
	raw1 := json.RawMessage(`{"Min": 60, "Avg": 72, "Max": 85, "date": "2026-01-21 00:00:00 +0900", "source": "Apple Watch"}`)
	raw2 := json.RawMessage(`{"Min": 55, "Avg": 70, "Max": 90, "date": "2026-01-21 00:00:00 +0900", "source": "Apple Watch"}`)
	upsertHeartRate(env.db, "bpm", raw1)
	upsertHeartRate(env.db, "bpm", raw2)
	rows, _ := queryMetrics(env.db, "heart_rate", "", "")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["Min"].(float64) != 55 {
		t.Errorf("expected updated Min=55, got %v", rows[0]["Min"])
	}
}

func TestUpsertDuplicateWorkout(t *testing.T) {
	env := setupTestDB(t)
	w1 := []RawWorkout{{ID: "w1", Name: "Running", Start: "2026-01-21 06:00:00 +0900", End: "2026-01-21 06:45:00 +0900", Duration: 2700}}
	w2 := []RawWorkout{{ID: "w1", Name: "Cycling", Start: "2026-01-21 06:00:00 +0900", End: "2026-01-21 06:45:00 +0900", Duration: 2700}}
	saveWorkouts(env.db, w1)
	saveWorkouts(env.db, w2)
	workouts, _ := queryWorkouts(env.db, "", "")
	if len(workouts) != 1 {
		t.Fatalf("expected 1 workout, got %d", len(workouts))
	}
	if workouts[0].WorkoutType != "Cycling" {
		t.Errorf("expected updated name=Cycling, got %s", workouts[0].WorkoutType)
	}
}

func TestUpsertDuplicateSleep(t *testing.T) {
	env := setupTestDB(t)
	raw1 := json.RawMessage(`{"date": "2026-01-21 00:00:00 +0900", "inBedStart": "", "inBedEnd": "", "sleepStart": "", "sleepEnd": "", "core": 3.5, "rem": 1.5, "deep": 1.0, "awake": 0.5, "inBed": 8.0, "asleep": 0, "totalSleep": 6.5}`)
	raw2 := json.RawMessage(`{"date": "2026-01-21 00:00:00 +0900", "inBedStart": "", "inBedEnd": "", "sleepStart": "", "sleepEnd": "", "core": 4.0, "rem": 2.0, "deep": 1.5, "awake": 0.3, "inBed": 7.8, "asleep": 0, "totalSleep": 7.5}`)
	upsertSleep(env.db, "hr", raw1)
	upsertSleep(env.db, "hr", raw2)
	rows, _ := queryMetrics(env.db, "sleep_analysis", "", "")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["core"].(float64) != 4.0 {
		t.Errorf("expected updated core=4.0, got %v", rows[0]["core"])
	}
}

// --- saveMetrics dispatch test ---

func TestSaveMetricsDispatch(t *testing.T) {
	env := setupTestDB(t)
	metrics := []RawMetric{
		{Name: "step_count", Units: "count", Data: []json.RawMessage{
			json.RawMessage(`{"qty": 5000, "date": "2026-01-21 00:00:00 +0900", "source": "Watch"}`),
		}},
		{Name: "heart_rate", Units: "bpm", Data: []json.RawMessage{
			json.RawMessage(`{"Min": 60, "Avg": 72, "Max": 85, "date": "2026-01-21 00:00:00 +0900", "source": "Watch"}`),
		}},
	}
	count, err := saveMetrics(env.db, metrics)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 metrics saved, got %d", count)
	}
}
