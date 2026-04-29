package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// toISO8601 converts date strings from the iPhone app format to ISO 8601 (like JavaScript's new Date().toISOString()).
// e.g. "2026-01-21 00:00:00 +0900" → "2026-01-20T15:00:00.000Z"
func toISO8601(s string) string {
	if s == "" {
		return ""
	}
	t := parseDate(s)
	if t == nil {
		return s
	}
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

// saveMetrics dispatches metric data to the correct table based on metric name.
// Returns the number of metric groups saved (not individual data points).
func saveMetrics(db *sql.DB, metrics []RawMetric) (int, error) {
	groupCount := 0
	for _, m := range metrics {
		for _, raw := range m.Data {
			var err error
			switch m.Name {
			case "heart_rate":
				err = upsertHeartRate(db, m.Units, raw)
			case "blood_pressure":
				err = upsertBloodPressure(db, m.Units, raw)
			case "sleep_analysis":
				err = upsertSleep(db, m.Units, raw)
			default:
				err = upsertBaseMetric(db, m.Name, m.Units, raw)
			}
			if err != nil {
				return groupCount, fmt.Errorf("saving %s: %w", m.Name, err)
			}
		}
		groupCount++
	}
	return groupCount, nil
}

func upsertBaseMetric(db *sql.DB, name, units string, raw json.RawMessage) error {
	var d BaseMetricData
	if err := json.Unmarshal(raw, &d); err != nil {
		return err
	}
	_, err := db.Exec(`
		INSERT INTO metrics (name, qty, units, date, source, metadata)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (name, date, source) DO UPDATE SET qty=excluded.qty, units=excluded.units, metadata=excluded.metadata`,
		name, d.Qty, units, toISO8601(d.Date), d.Source, rawStr(d.Metadata))
	return err
}

func upsertHeartRate(db *sql.DB, units string, raw json.RawMessage) error {
	var d HeartRateData
	if err := json.Unmarshal(raw, &d); err != nil {
		return err
	}
	_, err := db.Exec(`
		INSERT INTO heart_rate (min, avg, max, units, date, source, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (date, source) DO UPDATE SET min=excluded.min, avg=excluded.avg, max=excluded.max, units=excluded.units, metadata=excluded.metadata`,
		d.Min, d.Avg, d.Max, units, toISO8601(d.Date), d.Source, rawStr(d.Metadata))
	return err
}

func upsertBloodPressure(db *sql.DB, units string, raw json.RawMessage) error {
	var d BloodPressureData
	if err := json.Unmarshal(raw, &d); err != nil {
		return err
	}
	_, err := db.Exec(`
		INSERT INTO blood_pressure (systolic, diastolic, units, date, source, metadata)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (date, source) DO UPDATE SET systolic=excluded.systolic, diastolic=excluded.diastolic, units=excluded.units, metadata=excluded.metadata`,
		d.Systolic, d.Diastolic, units, toISO8601(d.Date), d.Source, rawStr(d.Metadata))
	return err
}

func upsertSleep(db *sql.DB, units string, raw json.RawMessage) error {
	var d SleepData
	if err := json.Unmarshal(raw, &d); err != nil {
		return err
	}
	_, err := db.Exec(`
		INSERT INTO sleep_analysis (date, in_bed_start, in_bed_end, sleep_start, sleep_end, core, rem, deep, awake, in_bed, asleep, total_sleep, units, source, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (date, source) DO UPDATE SET
			in_bed_start=excluded.in_bed_start, in_bed_end=excluded.in_bed_end,
			sleep_start=excluded.sleep_start, sleep_end=excluded.sleep_end,
			core=excluded.core, rem=excluded.rem, deep=excluded.deep,
			awake=excluded.awake, in_bed=excluded.in_bed, asleep=excluded.asleep, total_sleep=excluded.total_sleep,
			units=excluded.units, metadata=excluded.metadata`,
		toISO8601(d.Date), toISO8601(d.InBedStart), toISO8601(d.InBedEnd), toISO8601(d.SleepStart), toISO8601(d.SleepEnd),
		d.Core, d.Rem, d.Deep, d.Awake, d.InBed, d.Asleep, d.TotalSleep,
		units, d.Source, rawStr(d.Metadata))
	return err
}

// saveWorkouts saves workout and route data. Dates are converted to ISO 8601.
func saveWorkouts(db *sql.DB, workouts []RawWorkout) (int, int, error) {
	wCount, rCount := 0, 0
	for _, w := range workouts {
		hrdJSON, _ := json.Marshal(w.HeartRateData)
		hrrJSON, _ := json.Marshal(w.HeartRateRecovery)
		scJSON, _ := json.Marshal(w.StepCount)

		var aebQty, aebUnits, distQty, distUnits, tempQty, humQty, intQty interface{}
		if w.ActiveEnergyBurned != nil {
			aebQty = w.ActiveEnergyBurned.Qty
			aebUnits = w.ActiveEnergyBurned.Units
		}
		if w.Distance != nil {
			distQty = w.Distance.Qty
			distUnits = w.Distance.Units
		}
		if w.Temperature != nil {
			tempQty = w.Temperature.Qty
		}
		if w.Humidity != nil {
			humQty = w.Humidity.Qty
		}
		if w.Intensity != nil {
			intQty = w.Intensity.Qty
		}

		_, err := db.Exec(`
			INSERT INTO workouts (workout_id, name, start, end, duration,
				active_energy_burned_qty, active_energy_burned_units,
				distance_qty, distance_units,
				heart_rate_data, heart_rate_recovery, step_count,
				temperature_qty, humidity_qty, intensity_qty)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (workout_id) DO UPDATE SET
				name=excluded.name, start=excluded.start, end=excluded.end, duration=excluded.duration,
				active_energy_burned_qty=excluded.active_energy_burned_qty,
				active_energy_burned_units=excluded.active_energy_burned_units,
				distance_qty=excluded.distance_qty, distance_units=excluded.distance_units,
				heart_rate_data=excluded.heart_rate_data, heart_rate_recovery=excluded.heart_rate_recovery,
				step_count=excluded.step_count,
				temperature_qty=excluded.temperature_qty, humidity_qty=excluded.humidity_qty,
				intensity_qty=excluded.intensity_qty,
				updated_at=datetime('now')`,
			w.ID, w.Name, toISO8601(w.Start), toISO8601(w.End), w.Duration,
			aebQty, aebUnits, distQty, distUnits,
			string(hrdJSON), string(hrrJSON), string(scJSON),
			tempQty, humQty, intQty)
		if err != nil {
			return wCount, rCount, fmt.Errorf("saving workout %s: %w", w.ID, err)
		}
		wCount++

		if len(w.Route) > 0 {
			locJSON, _ := json.Marshal(w.Route)
			_, err := db.Exec(`
				INSERT INTO workout_routes (workout_id, locations)
				VALUES (?, ?)
				ON CONFLICT (workout_id) DO UPDATE SET locations=excluded.locations, updated_at=datetime('now')`,
				w.ID, string(locJSON))
			if err != nil {
				return wCount, rCount, fmt.Errorf("saving route for %s: %w", w.ID, err)
			}
			rCount++
		}
	}
	return wCount, rCount, nil
}

// queryMetrics returns rows from the appropriate table with correct field names.
func queryMetrics(db *sql.DB, metricType, from, to string) ([]map[string]interface{}, error) {
	switch metricType {
	case "heart_rate":
		return queryHeartRate(db, from, to)
	case "blood_pressure":
		return queryBloodPressure(db, from, to)
	case "sleep_analysis":
		return querySleep(db, from, to)
	default:
		return queryBaseMetric(db, metricType, from, to)
	}
}

func queryBaseMetric(db *sql.DB, metricType, from, to string) ([]map[string]interface{}, error) {
	query := "SELECT qty, units, date, source, metadata FROM metrics WHERE name = ?"
	args := []interface{}{metricType}

	if fromTo := dateFilter(from, to); fromTo != "" {
		query += " AND " + fromTo
		args = append(args, dateFilterArgs(from, to)...)
	}
	query += " ORDER BY date ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var qty float64
		var units, date, source string
		var metadata sql.NullString
		if err := rows.Scan(&qty, &units, &date, &source, &metadata); err != nil {
			return nil, err
		}
		row := map[string]interface{}{
			"qty":    qty,
			"units":  units,
			"date":   date,
			"source": source,
		}
		if m := parseMetadataForResponse(metadata); m != nil {
			row["metadata"] = m
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

func queryHeartRate(db *sql.DB, from, to string) ([]map[string]interface{}, error) {
	query := "SELECT min, avg, max, units, date, source, metadata FROM heart_rate"
	var args []interface{}

	if fromTo := dateFilter(from, to); fromTo != "" {
		query += " WHERE " + fromTo
		args = dateFilterArgs(from, to)
	}
	query += " ORDER BY date ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var min, avg, max float64
		var units, date, source string
		var metadata sql.NullString
		if err := rows.Scan(&min, &avg, &max, &units, &date, &source, &metadata); err != nil {
			return nil, err
		}
		row := map[string]interface{}{
			"Min":    min,
			"Avg":    avg,
			"Max":    max,
			"units":  units,
			"date":   date,
			"source": source,
		}
		if m := parseMetadataForResponse(metadata); m != nil {
			row["metadata"] = m
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

func queryBloodPressure(db *sql.DB, from, to string) ([]map[string]interface{}, error) {
	query := "SELECT systolic, diastolic, units, date, source, metadata FROM blood_pressure"
	var args []interface{}

	if fromTo := dateFilter(from, to); fromTo != "" {
		query += " WHERE " + fromTo
		args = dateFilterArgs(from, to)
	}
	query += " ORDER BY date ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var systolic, diastolic float64
		var units, date, source string
		var metadata sql.NullString
		if err := rows.Scan(&systolic, &diastolic, &units, &date, &source, &metadata); err != nil {
			return nil, err
		}
		row := map[string]interface{}{
			"systolic":  systolic,
			"diastolic": diastolic,
			"units":     units,
			"date":      date,
			"source":    source,
		}
		if m := parseMetadataForResponse(metadata); m != nil {
			row["metadata"] = m
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

func querySleep(db *sql.DB, from, to string) ([]map[string]interface{}, error) {
	query := "SELECT date, in_bed_start, in_bed_end, sleep_start, sleep_end, core, rem, deep, awake, in_bed, asleep, total_sleep, units, source, metadata FROM sleep_analysis"
	var args []interface{}

	if fromTo := dateFilter(from, to); fromTo != "" {
		query += " WHERE " + fromTo
		args = dateFilterArgs(from, to)
	}
	query += " ORDER BY date ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var date, inBedStart, inBedEnd, sleepStart, sleepEnd, units, source string
		var core, rem, deep, awake, inBed, asleep, totalSleep float64
		var metadata sql.NullString
		if err := rows.Scan(&date, &inBedStart, &inBedEnd, &sleepStart, &sleepEnd, &core, &rem, &deep, &awake, &inBed, &asleep, &totalSleep, &units, &source, &metadata); err != nil {
			return nil, err
		}
		row := map[string]interface{}{
			"date":       date,
			"inBedStart": inBedStart,
			"inBedEnd":   inBedEnd,
			"sleepStart": sleepStart,
			"sleepEnd":   sleepEnd,
			"core":       core,
			"rem":        rem,
			"deep":       deep,
			"awake":      awake,
			"inBed":      inBed,
			"asleep":     asleep,
			"totalSleep": totalSleep,
			"units":      units,
			"source":     source,
		}
		if m := parseMetadataForResponse(metadata); m != nil {
			row["metadata"] = m
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// queryWorkouts returns workout summaries with ISO dates.
func queryWorkouts(db *sql.DB, startDate, endDate string) ([]WorkoutSummary, error) {
	query := "SELECT workout_id, name, start, end, duration, active_energy_burned_qty FROM workouts"
	var args []interface{}

	if startDate != "" && endDate != "" {
		fromTime := parseDate(startDate)
		toTime := parseDate(endDate)
		if fromTime != nil && toTime != nil {
			query += " WHERE start >= ? AND start <= ?"
			args = append(args, fromTime.UTC().Format("2006-01-02T15:04:05.000Z"), toTime.UTC().Format("2006-01-02T15:04:05.000Z"))
		}
	}
	query += " ORDER BY start DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []WorkoutSummary
	for rows.Next() {
		var wid, name, start, end string
		var duration float64
		var aeb sql.NullFloat64
		if err := rows.Scan(&wid, &name, &start, &end, &duration, &aeb); err != nil {
			return nil, err
		}
		ws := WorkoutSummary{
			ID:              wid,
			WorkoutType:     name,
			StartTime:       start,
			EndTime:         end,
			DurationMinutes: duration / 60,
		}
		if aeb.Valid && aeb.Float64 != 0 {
			ws.CaloriesBurned = &aeb.Float64
		}
		results = append(results, ws)
	}
	return results, rows.Err()
}

// queryWorkoutDetail returns transformed heart rate data, recovery, and route.
func queryWorkoutDetail(db *sql.DB, workoutID string) (*WorkoutDetail, error) {
	var hrd, hrr sql.NullString
	err := db.QueryRow("SELECT heart_rate_data, heart_rate_recovery FROM workouts WHERE workout_id = ?", workoutID).Scan(&hrd, &hrr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	detail := &WorkoutDetail{}
	detail.HeartRateData = transformHeartRateData(hrd, "Heart Rate")
	detail.HeartRateRecovery = transformHeartRateData(hrr, "Heart Rate Recovery")

	var loc sql.NullString
	err = db.QueryRow("SELECT locations FROM workout_routes WHERE workout_id = ?", workoutID).Scan(&loc)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	detail.Route = transformRoute(loc)

	return detail, nil
}

// transformHeartRateData converts stored HR data to {type, timestamp, value} format.
func transformHeartRateData(raw sql.NullString, hrType string) json.RawMessage {
	if !raw.Valid || raw.String == "" || raw.String == "null" || raw.String == "[]" {
		return json.RawMessage("[]")
	}
	var entries []json.RawMessage
	if err := json.Unmarshal([]byte(raw.String), &entries); err != nil {
		return json.RawMessage("[]")
	}
	if len(entries) == 0 {
		return json.RawMessage("[]")
	}

	type hrEntry struct {
		Avg  float64 `json:"Avg"`
		Date string  `json:"date"`
	}
	type hrOut struct {
		Type      string `json:"type"`
		Timestamp string `json:"timestamp"`
		Value     float64 `json:"value"`
	}

	var out []hrOut
	for _, e := range entries {
		var h hrEntry
		if err := json.Unmarshal(e, &h); err != nil {
			continue
		}
		out = append(out, hrOut{
			Type:      hrType,
			Timestamp: toISO8601(h.Date),
			Value:     h.Avg,
		})
	}
	result, _ := json.Marshal(out)
	return json.RawMessage(result)
}

// transformRoute converts stored route to {latitude, longitude, time} format.
func transformRoute(raw sql.NullString) json.RawMessage {
	if !raw.Valid || raw.String == "" {
		return json.RawMessage("[]")
	}
	var points []RoutePoint
	if err := json.Unmarshal([]byte(raw.String), &points); err != nil {
		return json.RawMessage("[]")
	}
	if len(points) == 0 {
		return json.RawMessage("[]")
	}

	type routeOut struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Time      string  `json:"time"`
	}

	out := make([]routeOut, len(points))
	for i, p := range points {
		out[i] = routeOut{
			Latitude:  p.Latitude,
			Longitude: p.Longitude,
			Time:      toISO8601(p.Timestamp),
		}
	}
	result, _ := json.Marshal(out)
	return json.RawMessage(result)
}

// parseMetadataForResponse converts stored JSON string to a raw JSON value for proper serialization.
func parseMetadataForResponse(s sql.NullString) interface{} {
	if !s.Valid || s.String == "" {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal([]byte(s.String), &v); err != nil {
		return nil
	}
	return v
}

func rawStr(r *json.RawMessage) interface{} {
	if r == nil {
		return nil
	}
	return string(*r)
}

// dateFilter returns a WHERE clause fragment for date filtering.
func dateFilter(from, to string) string {
	if from == "" || to == "" {
		return ""
	}
	fromTime := parseDate(from)
	toTime := parseDate(to)
	if fromTime == nil || toTime == nil {
		return ""
	}
	return "date >= ? AND date <= ?"
}

func dateFilterArgs(from, to string) []interface{} {
	fromTime := parseDate(from)
	toTime := parseDate(to)
	return []interface{}{
		fromTime.UTC().Format("2006-01-02T15:04:05.000Z"),
		toTime.UTC().Format("2006-01-02T15:04:05.000Z"),
	}
}

// parseDate handles various date formats: unix timestamp, YYYY/MM/DD, YYYY-MM-DD, YYYY-MM-DD HH:MM:SS, ISO 8601.
func parseDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	// Try as Unix timestamp (milliseconds or seconds)
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		var t time.Time
		if n > 1e12 {
			t = time.UnixMilli(n)
		} else {
			t = time.Unix(n, 0)
		}
		return &t
	}
	// Try as float Unix timestamp
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		t := time.UnixMilli(int64(f))
		return &t
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"2006/01/02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return &t
		}
	}
	return nil
}
