package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- GET Metrics Tests ---

func TestGetMetricsEmpty(t *testing.T) {
	env := newTestEnv(t)
	resp := env.get("/api/metrics/step_count", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var result []interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

func TestGetMetricsDateFilter(t *testing.T) {
	env := newTestEnv(t)
	// Insert two data points
	raw1 := json.RawMessage(`{"qty": 5000, "date": "2026-01-21 00:00:00 +0900", "source": "Watch"}`)
	raw2 := json.RawMessage(`{"qty": 6000, "date": "2026-01-25 00:00:00 +0900", "source": "Watch"}`)
	upsertBaseMetric(env.db, "step_count", "count", raw1)
	upsertBaseMetric(env.db, "step_count", "count", raw2)

	// Query with date filter
	resp := env.get("/api/metrics/step_count?from=2026-01-22&to=2026-01-26", nil)
	var result []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result) != 1 {
		t.Fatalf("expected 1 filtered result, got %d", len(result))
	}
	if result[0]["qty"].(float64) != 6000 {
		t.Errorf("expected qty=6000, got %v", result[0]["qty"])
	}
}

func TestGetMetricsIncludeFields(t *testing.T) {
	env := newTestEnv(t)
	raw := json.RawMessage(`{"qty": 5000, "date": "2026-01-21 00:00:00 +0900", "source": "Watch"}`)
	upsertBaseMetric(env.db, "step_count", "count", raw)

	resp := env.get("/api/metrics/step_count?include=date,qty", nil)
	var result []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if _, ok := result[0]["source"]; ok {
		t.Error("expected source to be excluded when using include filter")
	}
	if _, ok := result[0]["date"]; !ok {
		t.Error("expected date to be included")
	}
}

func TestGetMetricsExcludeFields(t *testing.T) {
	env := newTestEnv(t)
	raw := json.RawMessage(`{"qty": 5000, "date": "2026-01-21 00:00:00 +0900", "source": "Watch"}`)
	upsertBaseMetric(env.db, "step_count", "count", raw)

	resp := env.get("/api/metrics/step_count?exclude=id,metadata", nil)
	var result []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if _, ok := result[0]["id"]; ok {
		t.Error("expected id to be excluded")
	}
}

func TestGetHeartRateMetrics(t *testing.T) {
	env := newTestEnv(t)
	raw := json.RawMessage(`{"Min": 60, "Avg": 72.5, "Max": 85, "date": "2026-01-21 00:00:00 +0900", "source": "Watch"}`)
	upsertHeartRate(env.db, "count/min", raw)

	resp := env.get("/api/metrics/heart_rate", nil)
	var result []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0]["Avg"].(float64) != 72.5 {
		t.Errorf("expected Avg=72.5, got %v", result[0]["Avg"])
	}
}

// --- GET Workouts Tests ---

func TestGetWorkoutsEmpty(t *testing.T) {
	env := newTestEnv(t)
	resp := env.get("/api/workouts", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var result []interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

func TestGetWorkoutsList(t *testing.T) {
	env := newTestEnv(t)
	saveWorkouts(env.db, []RawWorkout{
		{ID: "w1", Name: "Running", Start: "2026-01-21 06:00:00 +0900", End: "2026-01-21 06:45:00 +0900", Duration: 2700, ActiveEnergyBurned: &QtyUnits{Qty: 450, Units: "kcal"}},
		{ID: "w2", Name: "Cycling", Start: "2026-01-22 07:00:00 +0900", End: "2026-01-22 08:00:00 +0900", Duration: 3600},
	})

	resp := env.get("/api/workouts", nil)
	var result []WorkoutSummary
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	// Sorted by start DESC
	if result[0].WorkoutType != "Cycling" {
		t.Errorf("expected first=Cycling, got %s", result[0].WorkoutType)
	}
	if result[0].DurationMinutes != 60.0 {
		t.Errorf("expected 60 min, got %f", result[0].DurationMinutes)
	}
}

func TestGetWorkoutDetail(t *testing.T) {
	env := newTestEnv(t)
	hrd := []json.RawMessage{json.RawMessage(`{"Min":120,"Avg":140,"Max":160}`)}
	saveWorkouts(env.db, []RawWorkout{
		{
			ID: "w1", Name: "Running",
			Start: "2026-01-21 06:00:00 +0900", End: "2026-01-21 06:45:00 +0900",
			Duration:      2700,
			HeartRateData: hrd,
			Route: []RoutePoint{
				{Latitude: 35.68, Longitude: 139.76, Timestamp: "2026-01-21 06:00:00 +0900"},
			},
		},
	})

	resp := env.get("/api/workouts/w1", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	body, _ := io.ReadAll(resp.Body)
	var detail map[string]json.RawMessage
	json.Unmarshal(body, &detail)
	if _, ok := detail["heartRateData"]; !ok {
		t.Error("expected heartRateData in response")
	}
	if _, ok := detail["route"]; !ok {
		t.Error("expected route in response")
	}
}

func TestGetWorkoutDetail404(t *testing.T) {
	env := newTestEnv(t)
	resp := env.get("/api/workouts/nonexistent", nil)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

// --- Auth Tests ---

func TestAuthRequired(t *testing.T) {
	env := newTestEnv(t)

	// No token
	resp := env.getNoAuth("/api/metrics/step_count")
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", resp.Code)
	}

	// Wrong token
	resp = env.getWithToken("/api/metrics/step_count", "sk-wrong")
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with wrong token, got %d", resp.Code)
	}

	// Token without sk- prefix
	resp = env.getWithToken("/api/metrics/step_count", "no-prefix")
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without sk- prefix, got %d", resp.Code)
	}
}

func TestWriteAuthRequired(t *testing.T) {
	env := newTestEnv(t)
	body := `{"data":{"metrics":[],"workouts":[]}}`

	// No token
	resp := env.postNoAuth("/api/data", body)
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without write token, got %d", resp.Code)
	}

	// Read token on write endpoint
	resp = env.postWithToken("/api/data", body, env.readToken)
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with read token on write endpoint, got %d", resp.Code)
	}
}

// --- Integration: POST → GET round trip ---

func TestPostThenGetMetrics(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [
				{
					"name": "step_count",
					"units": "count",
					"data": [
						{"qty": 8000, "date": "2026-01-21 00:00:00 +0900", "source": "Apple Watch"},
						{"qty": 9000, "date": "2026-01-22 00:00:00 +0900", "source": "Apple Watch"}
					]
				},
				{
					"name": "heart_rate",
					"units": "count/min",
					"data": [
						{"Min": 55, "Avg": 68, "Max": 130, "date": "2026-01-21 00:00:00 +0900", "source": "Apple Watch"}
					]
				}
			],
			"workouts": []
		}
	}`

	resp := env.post("/api/data", payload)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var ir IngestResponse
	json.NewDecoder(resp.Body).Decode(&ir)
	if !ir.Metrics.Success {
		t.Fatalf("metrics ingest failed: %s", ir.Metrics.Message)
	}

	// GET step_count
	resp2 := env.get("/api/metrics/step_count", nil)
	var steps []map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&steps)
	if len(steps) != 2 {
		t.Fatalf("expected 2 step_count rows, got %d", len(steps))
	}

	// GET heart_rate
	resp3 := env.get("/api/metrics/heart_rate", nil)
	var hr []map[string]interface{}
	json.NewDecoder(resp3.Body).Decode(&hr)
	if len(hr) != 1 {
		t.Fatalf("expected 1 heart_rate row, got %d", len(hr))
	}
}

func TestPostThenGetWorkouts(t *testing.T) {
	env := newTestEnv(t)
	payload := `{
		"data": {
			"metrics": [],
			"workouts": [
				{
					"id": "W-001",
					"name": "Running",
					"start": "2026-01-21 06:00:00 +0900",
					"end": "2026-01-21 06:45:00 +0900",
					"duration": 2700,
					"activeEnergyBurned": {"qty": 450, "units": "kcal"},
					"distance": {"qty": 5.2, "units": "km"},
					"heartRateData": [{"Min":120,"Avg":140,"Max":160,"date":"2026-01-21 06:10:00 +0900","units":"bpm"}],
					"route": [
						{"latitude": 35.68, "longitude": 139.76, "timestamp": "2026-01-21 06:00:00 +0900"}
					]
				}
			]
		}
	}`

	resp := env.post("/api/data", payload)
	if resp.Code != http.StatusOK {
		t.Fatalf("POST expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// GET workouts list
	resp2 := env.get("/api/workouts", nil)
	var workouts []WorkoutSummary
	json.NewDecoder(resp2.Body).Decode(&workouts)
	if len(workouts) != 1 {
		t.Fatalf("expected 1 workout, got %d", len(workouts))
	}
	if workouts[0].ID != "W-001" {
		t.Errorf("expected id=W-001, got %s", workouts[0].ID)
	}
	if workouts[0].CaloriesBurned == nil || *workouts[0].CaloriesBurned != 450 {
		t.Errorf("expected calories=450, got %v", workouts[0].CaloriesBurned)
	}

	// GET workout detail
	resp3 := env.get("/api/workouts/W-001", nil)
	body, _ := io.ReadAll(resp3.Body)
	var detail map[string]json.RawMessage
	json.Unmarshal(body, &detail)
	if _, ok := detail["route"]; !ok {
		t.Error("expected route in detail")
	}
}

func TestHealthCheck(t *testing.T) {
	env := newTestEnv(t)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	env.handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result map[string]string
	json.NewDecoder(w.Body).Decode(&result)
	if result["message"] != "Hello world!" {
		t.Errorf("unexpected message: %s", result["message"])
	}
}

func TestPostInvalidJSON(t *testing.T) {
	env := newTestEnv(t)
	resp := env.post("/api/data", `{invalid}`)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

// --- Test helper: testEnv ---

type testEnv struct {
	db         *sql.DB
	handler    http.Handler
	readToken  string
	writeToken string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	db, err := openDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	rt := "sk-test-read"
	wt := "sk-test-write"
	srv := NewServer(db, rt, wt)
	return &testEnv{db: db, handler: srv.Handler(), readToken: rt, writeToken: wt}
}

func (e *testEnv) get(path string, _ interface{}) *httptest.ResponseRecorder {
	return e.getWithToken(path, e.readToken)
}

func (e *testEnv) getNoAuth(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, req)
	return w
}

func (e *testEnv) getWithToken(path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("api-key", token)
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, req)
	return w
}

func (e *testEnv) post(path, body string) *httptest.ResponseRecorder {
	return e.postWithToken(path, body, e.writeToken)
}

func (e *testEnv) postNoAuth(path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, req)
	return w
}

func (e *testEnv) postWithToken(path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", token)
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, req)
	return w
}
