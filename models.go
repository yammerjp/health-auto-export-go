package main

import "encoding/json"

// IngestPayload is the top-level JSON from the iPhone app.
type IngestPayload struct {
	Data struct {
		Metrics  []RawMetric  `json:"metrics"`
		Workouts []RawWorkout `json:"workouts"`
	} `json:"data"`
}

// RawMetric is a metric group as sent by the app.
type RawMetric struct {
	Name  string            `json:"name"`
	Units string            `json:"units"`
	Data  []json.RawMessage `json:"data"`
}

// MetricDataPoint is the common fields in every data point.
type MetricDataPoint struct {
	Date     string           `json:"date"`
	Source   string           `json:"source"`
	Metadata *json.RawMessage `json:"metadata,omitempty"`
}

// BaseMetricData is for generic qty-based metrics (step_count, etc.).
type BaseMetricData struct {
	MetricDataPoint
	Qty float64 `json:"qty"`
}

// HeartRateData holds heart rate min/avg/max.
type HeartRateData struct {
	MetricDataPoint
	Min float64 `json:"Min"`
	Avg float64 `json:"Avg"`
	Max float64 `json:"Max"`
}

// BloodPressureData holds systolic/diastolic.
type BloodPressureData struct {
	MetricDataPoint
	Systolic  float64 `json:"systolic"`
	Diastolic float64 `json:"diastolic"`
}

// SleepData holds sleep analysis data.
type SleepData struct {
	MetricDataPoint
	InBedStart string  `json:"inBedStart"`
	InBedEnd   string  `json:"inBedEnd"`
	SleepStart string  `json:"sleepStart"`
	SleepEnd   string  `json:"sleepEnd"`
	Core       float64 `json:"core"`
	Rem        float64 `json:"rem"`
	Deep       float64 `json:"deep"`
	Awake      float64 `json:"awake"`
	InBed      float64 `json:"inBed"`
	Asleep     float64 `json:"asleep"`
	TotalSleep float64 `json:"totalSleep"`
}

// RawWorkout is a workout as sent by the app.
type RawWorkout struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Start              string            `json:"start"`
	End                string            `json:"end"`
	Duration           float64           `json:"duration"`
	ActiveEnergyBurned *QtyUnits         `json:"activeEnergyBurned"`
	Distance           *QtyUnits         `json:"distance"`
	HeartRateData      []json.RawMessage `json:"heartRateData"`
	HeartRateRecovery  []json.RawMessage `json:"heartRateRecovery"`
	StepCount          []json.RawMessage `json:"stepCount"`
	Temperature        *QtyUnits         `json:"temperature"`
	Humidity           *QtyUnits         `json:"humidity"`
	Intensity          *QtyUnits         `json:"intensity"`
	Route              []RoutePoint      `json:"route"`
}

type QtyUnits struct {
	Qty   float64 `json:"qty"`
	Units string  `json:"units"`
}

type RoutePoint struct {
	Latitude           float64 `json:"latitude"`
	Longitude          float64 `json:"longitude"`
	Timestamp          string  `json:"timestamp"`
	Altitude           float64 `json:"altitude,omitempty"`
	Speed              float64 `json:"speed,omitempty"`
	SpeedAccuracy      float64 `json:"speedAccuracy,omitempty"`
	Course             float64 `json:"course,omitempty"`
	CourseAccuracy     float64 `json:"courseAccuracy,omitempty"`
	HorizontalAccuracy float64 `json:"horizontalAccuracy,omitempty"`
	VerticalAccuracy   float64 `json:"verticalAccuracy,omitempty"`
}

// Response types

type IngestResponse struct {
	Metrics  *IngestResult `json:"metrics"`
	Workouts *IngestResult `json:"workouts"`
}

type IngestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// WorkoutSummary is returned by GET /api/workouts.
type WorkoutSummary struct {
	ID              string   `json:"id"`
	WorkoutType     string   `json:"workout_type"`
	StartTime       string   `json:"start_time"`
	EndTime         string   `json:"end_time"`
	DurationMinutes float64  `json:"duration_minutes"`
	CaloriesBurned  *float64 `json:"calories_burned"`
}

// WorkoutDetail is returned by GET /api/workouts/:id.
// Fields are always present (empty arrays, not omitted).
type WorkoutDetail struct {
	HeartRateData     json.RawMessage `json:"heartRateData"`
	HeartRateRecovery json.RawMessage `json:"heartRateRecovery"`
	Route             json.RawMessage `json:"route"`
}
