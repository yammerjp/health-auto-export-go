package main

import (
	"encoding/json"
	"io"
	"testing"
)

// These tests pin the JSON contract expected by Health Auto Export iOS clients
// and downstream dashboards. They are the spec for the public API.

// --- Date format tests ---

func TestDateStoredAsISO8601(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [{
				"name": "step_count", "units": "count",
				"data": [{"qty": 5000, "date": "2026-01-21 00:00:00 +0900", "source": "Watch"}]
			}],
			"workouts": []
		}
	}`
	env.post("/api/data", payload)

	resp := env.get("/api/metrics/step_count", nil)
	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
	date, ok := rows[0]["date"].(string)
	if !ok {
		t.Fatalf("date not a string: %T", rows[0]["date"])
	}
	// Dates are normalised to ISO 8601 in UTC before storage.
	expected := "2026-01-20T15:00:00.000Z"
	if date != expected {
		t.Errorf("date format mismatch:\n  got:  %s\n  want: %s", date, expected)
	}
}

func TestDateFilteringWorks(t *testing.T) {
	env := newTestEnv(t)
	// Insert data at two different dates
	payload := `{
		"data": {
			"metrics": [{
				"name": "step_count", "units": "count",
				"data": [
					{"qty": 5000, "date": "2026-01-20 00:00:00 +0900", "source": "Watch"},
					{"qty": 6000, "date": "2026-01-22 00:00:00 +0900", "source": "Watch"},
					{"qty": 7000, "date": "2026-01-24 00:00:00 +0900", "source": "Watch"}
				]
			}],
			"workouts": []
		}
	}`
	env.post("/api/data", payload)

	// Filter: should return only the middle one
	resp := env.get("/api/metrics/step_count?from=2026-01-21&to=2026-01-23", nil)
	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)
	if len(rows) != 1 {
		t.Fatalf("expected 1 filtered row, got %d", len(rows))
	}
	if rows[0]["qty"].(float64) != 6000 {
		t.Errorf("expected qty=6000, got %v", rows[0]["qty"])
	}
}

func TestDateFilteringWithUnixTimestamp(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [{
				"name": "step_count", "units": "count",
				"data": [
					{"qty": 5000, "date": "2026-01-21 00:00:00 +0900", "source": "Watch"}
				]
			}],
			"workouts": []
		}
	}`
	env.post("/api/data", payload)

	// Date filters also accept Unix timestamps (milliseconds since epoch).
	// Stored date: "2026-01-21 00:00:00 +0900" = "2026-01-20T15:00:00.000Z"
	// from=2026-01-20T00:00:00Z (1768867200000), to=2026-01-21T00:00:00Z (1768953600000)
	resp := env.get("/api/metrics/step_count?from=1768867200000&to=1768953600000", nil)
	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row with unix timestamp filter, got %d", len(rows))
	}
}

// --- Heart rate field name tests ---

func TestHeartRateFieldNamesCapitalized(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [{
				"name": "heart_rate", "units": "count/min",
				"data": [{"Min": 60, "Avg": 72.5, "Max": 85, "date": "2026-01-21 00:00:00 +0900", "source": "Watch"}]
			}],
			"workouts": []
		}
	}`
	env.post("/api/data", payload)

	resp := env.get("/api/metrics/heart_rate", nil)
	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
	// Heart rate fields keep the capitalised keys used in the iOS payload.
	if _, ok := rows[0]["Min"]; !ok {
		t.Error("expected 'Min' (capitalized), not 'min'")
	}
	if _, ok := rows[0]["Avg"]; !ok {
		t.Error("expected 'Avg' (capitalized), not 'avg'")
	}
	if _, ok := rows[0]["Max"]; !ok {
		t.Error("expected 'Max' (capitalized), not 'max'")
	}
}

// --- Sleep field name tests ---

func TestSleepFieldNamesCamelCase(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [{
				"name": "sleep_analysis", "units": "hr",
				"data": [{
					"date": "2026-01-21 00:00:00 +0900",
					"inBedStart": "2026-01-20 23:00:00 +0900",
					"inBedEnd": "2026-01-21 07:00:00 +0900",
					"sleepStart": "2026-01-20 23:30:00 +0900",
					"sleepEnd": "2026-01-21 06:50:00 +0900",
					"core": 3.5, "rem": 1.5, "deep": 1.0, "awake": 0.5, "inBed": 8.0
				}]
			}],
			"workouts": []
		}
	}`
	env.post("/api/data", payload)

	resp := env.get("/api/metrics/sleep_analysis", nil)
	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
	// Sleep fields keep the camelCase keys used in the iOS payload.
	for _, field := range []string{"inBedStart", "inBedEnd", "sleepStart", "sleepEnd", "inBed"} {
		if _, ok := rows[0][field]; !ok {
			t.Errorf("expected camelCase field %q in response", field)
		}
	}
	// Should NOT have snake_case
	for _, field := range []string{"in_bed_start", "in_bed_end", "sleep_start", "sleep_end", "in_bed"} {
		if _, ok := rows[0][field]; ok {
			t.Errorf("unexpected snake_case field %q in response", field)
		}
	}
}

// --- Base metrics should NOT include "name" field ---

func TestBaseMetricResponseNoNameField(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [{
				"name": "step_count", "units": "count",
				"data": [{"qty": 5000, "date": "2026-01-21 00:00:00 +0900", "source": "Watch"}]
			}],
			"workouts": []
		}
	}`
	env.post("/api/data", payload)

	resp := env.get("/api/metrics/step_count", nil)
	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
	// The metric type is in the URL, so it isn't echoed back in each row.
	if _, ok := rows[0]["name"]; ok {
		t.Error("base metric response should NOT include 'name' field")
	}
}

// --- Metadata should be returned as JSON object, not string ---

func TestMetadataReturnedAsObject(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [{
				"name": "step_count", "units": "count",
				"data": [{"qty": 5000, "date": "2026-01-21 00:00:00 +0900", "source": "Watch", "metadata": {"key": "value"}}]
			}],
			"workouts": []
		}
	}`
	env.post("/api/data", payload)

	resp := env.get("/api/metrics/step_count", nil)
	body, _ := io.ReadAll(resp.Body)

	var rows []map[string]json.RawMessage
	json.Unmarshal(body, &rows)
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}

	meta := rows[0]["metadata"]
	if meta == nil {
		t.Fatal("expected metadata")
	}
	// Should be a JSON object, not a quoted string
	var obj map[string]interface{}
	if err := json.Unmarshal(meta, &obj); err != nil {
		t.Errorf("metadata should be a JSON object, got: %s", string(meta))
	}
	if obj["key"] != "value" {
		t.Errorf("expected metadata.key=value, got %v", obj["key"])
	}
}

// --- Workout detail format ---

func TestWorkoutDetailHeartRateTransformed(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [],
			"workouts": [{
				"id": "W-001",
				"name": "Running",
				"start": "2026-01-21 06:00:00 +0900",
				"end": "2026-01-21 06:45:00 +0900",
				"duration": 2700,
				"heartRateData": [
					{"Min": 120, "Avg": 140, "Max": 160, "date": "2026-01-21 06:10:00 +0900", "units": "count/min", "source": "Watch"}
				],
				"route": [
					{"latitude": 35.68, "longitude": 139.76, "timestamp": "2026-01-21 06:00:00 +0900", "altitude": 10, "speed": 2.8}
				]
			}]
		}
	}`
	env.post("/api/data", payload)

	resp := env.get("/api/workouts/W-001", nil)
	body, _ := io.ReadAll(resp.Body)

	var detail map[string]json.RawMessage
	json.Unmarshal(body, &detail)

	// heartRateData should be transformed to {type, timestamp, value}
	var hrData []map[string]interface{}
	json.Unmarshal(detail["heartRateData"], &hrData)
	if len(hrData) == 0 {
		t.Fatal("expected heartRateData")
	}
	if hrData[0]["type"] != "Heart Rate" {
		t.Errorf("expected type='Heart Rate', got %v", hrData[0]["type"])
	}
	if _, ok := hrData[0]["timestamp"]; !ok {
		t.Error("expected 'timestamp' field in heart rate data")
	}
	if v, ok := hrData[0]["value"].(float64); !ok || v != 140 {
		t.Errorf("expected value=140 (Avg), got %v", hrData[0]["value"])
	}
	// Should NOT have Min/Avg/Max raw fields
	if _, ok := hrData[0]["Min"]; ok {
		t.Error("heartRateData should be transformed, not raw")
	}

	// route should be transformed to {latitude, longitude, time}
	var route []map[string]interface{}
	json.Unmarshal(detail["route"], &route)
	if len(route) == 0 {
		t.Fatal("expected route")
	}
	if _, ok := route[0]["time"]; !ok {
		t.Error("expected 'time' field (not 'timestamp') in route")
	}
	if _, ok := route[0]["altitude"]; ok {
		t.Error("route should only have latitude, longitude, time — not altitude")
	}
}

func TestWorkoutDetailEmptyArraysNotOmitted(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [],
			"workouts": [{
				"id": "W-002",
				"name": "Walking",
				"start": "2026-01-21 06:00:00 +0900",
				"end": "2026-01-21 06:30:00 +0900",
				"duration": 1800
			}]
		}
	}`
	env.post("/api/data", payload)

	resp := env.get("/api/workouts/W-002", nil)
	body, _ := io.ReadAll(resp.Body)

	var detail map[string]json.RawMessage
	json.Unmarshal(body, &detail)

	// Workout detail always includes heartRateData and route — empty arrays,
	// not omitted fields — so clients can rely on them existing.
	if _, ok := detail["heartRateData"]; !ok {
		t.Error("heartRateData should be present (empty array), not omitted")
	}
	if _, ok := detail["route"]; !ok {
		t.Error("route should be present (empty array), not omitted")
	}

	// Verify they are empty arrays
	var hrData []interface{}
	json.Unmarshal(detail["heartRateData"], &hrData)
	if len(hrData) != 0 {
		t.Errorf("expected empty heartRateData, got %d items", len(hrData))
	}

	var route []interface{}
	json.Unmarshal(detail["route"], &route)
	if len(route) != 0 {
		t.Errorf("expected empty route, got %d items", len(route))
	}
}

// --- Workout list date format ---

func TestWorkoutListDateFormat(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [],
			"workouts": [{
				"id": "W-001",
				"name": "Running",
				"start": "2026-01-21 06:00:00 +0900",
				"end": "2026-01-21 06:45:00 +0900",
				"duration": 2700,
				"activeEnergyBurned": {"qty": 450, "units": "kcal"}
			}]
		}
	}`
	env.post("/api/data", payload)

	resp := env.get("/api/workouts", nil)
	var workouts []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&workouts)
	if len(workouts) == 0 {
		t.Fatal("expected workouts")
	}
	// Workout list dates are ISO 8601 in UTC.
	startTime := workouts[0]["start_time"].(string)
	expected := "2026-01-20T21:00:00.000Z"
	if startTime != expected {
		t.Errorf("start_time format mismatch:\n  got:  %s\n  want: %s", startTime, expected)
	}
}

// --- calories_burned should be null when missing ---

func TestCaloriesBurnedNullWhenMissing(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [],
			"workouts": [{
				"id": "W-001",
				"name": "Walking",
				"start": "2026-01-21 06:00:00 +0900",
				"end": "2026-01-21 06:30:00 +0900",
				"duration": 1800
			}]
		}
	}`
	env.post("/api/data", payload)

	resp := env.get("/api/workouts", nil)
	body, _ := io.ReadAll(resp.Body)

	var workouts []map[string]interface{}
	json.Unmarshal(body, &workouts)
	if len(workouts) == 0 {
		t.Fatal("expected workouts")
	}
	// calories_burned is null when activeEnergyBurned is missing.
	cal := workouts[0]["calories_burned"]
	if cal != nil {
		t.Errorf("expected calories_burned=null when missing, got %v", cal)
	}
}

// --- Metric responses don't expose internal database IDs ---

func TestMetricResponseNoSQLiteID(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [{
				"name": "step_count", "units": "count",
				"data": [{"qty": 5000, "date": "2026-01-21 00:00:00 +0900", "source": "Watch"}]
			}],
			"workouts": []
		}
	}`
	env.post("/api/data", payload)

	resp := env.get("/api/metrics/step_count", nil)
	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
	// Should not have SQLite integer "id"
	if _, ok := rows[0]["id"]; ok {
		t.Error("should not expose SQLite integer 'id' in response")
	}
}

// --- Auth error should return JSON Content-Type ---

func TestAuthErrorReturnsJSON(t *testing.T) {
	env := newTestEnv(t)
	resp := env.getNoAuth("/api/metrics/step_count")
	ct := resp.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("auth error Content-Type should be application/json, got %s", ct)
	}
	// Should be parseable JSON
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Errorf("auth error response should be valid JSON: %v", err)
	}
}

// --- Ingest response for empty metrics/workouts ---

func TestIngestResponseForEmptyData(t *testing.T) {
	env := newTestEnv(t)
	payload := `{"data": {"metrics": [], "workouts": []}}`
	resp := env.post("/api/data", payload)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]json.RawMessage
	json.Unmarshal(body, &result)

	// Ingest response always includes both keys, even when their input is empty.
	if _, ok := result["metrics"]; !ok {
		t.Error("should include 'metrics' key even when empty")
	}
	if _, ok := result["workouts"]; !ok {
		t.Error("should include 'workouts' key even when empty")
	}
}

// --- Sleep date fields should also be ISO 8601 ---

func TestSleepDateFieldsISO8601(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [{
				"name": "sleep_analysis", "units": "hr",
				"data": [{
					"date": "2026-01-21 00:00:00 +0900",
					"inBedStart": "2026-01-20 23:00:00 +0900",
					"inBedEnd": "2026-01-21 07:00:00 +0900",
					"sleepStart": "2026-01-20 23:30:00 +0900",
					"sleepEnd": "2026-01-21 06:50:00 +0900",
					"core": 3.5, "rem": 1.5, "deep": 1.0, "awake": 0.5, "inBed": 8.0
				}]
			}],
			"workouts": []
		}
	}`
	env.post("/api/data", payload)

	resp := env.get("/api/metrics/sleep_analysis", nil)
	var rows []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rows)
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
	// All date fields should be ISO 8601
	inBedStart, _ := rows[0]["inBedStart"].(string)
	expected := "2026-01-20T14:00:00.000Z"
	if inBedStart != expected {
		t.Errorf("inBedStart format:\n  got:  %s\n  want: %s", inBedStart, expected)
	}
}

// --- Workout date filtering ---

func TestWorkoutDateFiltering(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [],
			"workouts": [
				{"id": "W1", "name": "Run", "start": "2026-01-20 06:00:00 +0900", "end": "2026-01-20 07:00:00 +0900", "duration": 3600},
				{"id": "W2", "name": "Bike", "start": "2026-01-22 06:00:00 +0900", "end": "2026-01-22 07:00:00 +0900", "duration": 3600},
				{"id": "W3", "name": "Swim", "start": "2026-01-24 06:00:00 +0900", "end": "2026-01-24 07:00:00 +0900", "duration": 3600}
			]
		}
	}`
	env.post("/api/data", payload)

	resp := env.get("/api/workouts?startDate=2026-01-21&endDate=2026-01-23", nil)
	var workouts []WorkoutSummary
	json.NewDecoder(resp.Body).Decode(&workouts)
	if len(workouts) != 1 {
		t.Fatalf("expected 1 filtered workout, got %d", len(workouts))
	}
	if workouts[0].ID != "W2" {
		t.Errorf("expected W2, got %s", workouts[0].ID)
	}
}

// --- Ingest response: count metric groups, not data points ---

func TestIngestCountsMetricGroups(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [
				{"name": "step_count", "units": "count", "data": [
					{"qty": 5000, "date": "2026-01-21 00:00:00 +0900", "source": "Watch"},
					{"qty": 6000, "date": "2026-01-22 00:00:00 +0900", "source": "Watch"}
				]},
				{"name": "heart_rate", "units": "bpm", "data": [
					{"Min": 60, "Avg": 72, "Max": 85, "date": "2026-01-21 00:00:00 +0900", "source": "Watch"}
				]}
			],
			"workouts": []
		}
	}`
	resp := env.post("/api/data", payload)
	var result IngestResponse
	json.NewDecoder(resp.Body).Decode(&result)
	// The ingest message counts metric groups (2 here), not individual points (3).
	expected := "2 metrics saved successfully"
	if result.Metrics.Message != expected {
		t.Errorf("ingest message:\n  got:  %s\n  want: %s", result.Metrics.Message, expected)
	}
}
