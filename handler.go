package main

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

//go:embed dashboard.html
var dashboardHTML string

type Server struct {
	db         *sql.DB
	readToken  string
	writeToken string
}

func NewServer(db *sql.DB, readToken, writeToken string) *Server {
	return &Server{db: db, readToken: readToken, writeToken: writeToken}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleHealthCheck)
	mux.HandleFunc("POST /api/data", s.requireWriteAuth(s.handleIngest))
	mux.HandleFunc("GET /api/metrics/{type}", s.requireReadAuth(s.handleGetMetrics))
	mux.HandleFunc("GET /api/workouts", s.requireReadAuth(s.handleGetWorkouts))
	mux.HandleFunc("GET /api/workouts/{id}", s.requireReadAuth(s.handleGetWorkoutDetail))
	mux.HandleFunc("GET /dashboard", s.handleDashboard)
	return logMiddleware(corsMiddleware(mux))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Microsecond))
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireReadAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("api-key")
		if token == "" || !strings.HasPrefix(token, "sk-") || token != s.readToken {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized: Invalid read token"})
			return
		}
		next(w, r)
	}
}

func (s *Server) requireWriteAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("api-key")
		if token == "" || !strings.HasPrefix(token, "sk-") || token != s.writeToken {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized: Invalid write token"})
			return
		}
		next(w, r)
	}
}

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "Hello world!"})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	var payload IngestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	resp := IngestResponse{}
	hasError := false

	if len(payload.Data.Metrics) > 0 {
		count, err := saveMetrics(s.db, payload.Data.Metrics)
		if err != nil {
			resp.Metrics = &IngestResult{Success: false, Message: err.Error()}
			hasError = true
		} else {
			resp.Metrics = &IngestResult{Success: true, Message: fmt.Sprintf("%d metrics saved successfully", count)}
		}
	} else {
		resp.Metrics = &IngestResult{Success: true, Message: "No metrics data provided"}
	}

	if len(payload.Data.Workouts) > 0 {
		wCount, rCount, err := saveWorkouts(s.db, payload.Data.Workouts)
		if err != nil {
			resp.Workouts = &IngestResult{Success: false, Message: err.Error()}
			hasError = true
		} else {
			resp.Workouts = &IngestResult{Success: true, Message: fmt.Sprintf("%d Workouts and %d Routes saved successfully", wCount, rCount)}
		}
	} else {
		resp.Workouts = &IngestResult{Success: true, Message: "No workout data provided"}
	}

	status := http.StatusOK
	if hasError {
		if (resp.Metrics != nil && resp.Metrics.Success) || (resp.Workouts != nil && resp.Workouts.Success) {
			status = http.StatusMultiStatus // 207
		} else {
			status = http.StatusInternalServerError
		}
	}
	writeJSON(w, status, resp)
}

func (s *Server) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	metricType := r.PathValue("type")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	include := r.URL.Query().Get("include")
	exclude := r.URL.Query().Get("exclude")

	rows, err := queryMetrics(s.db, metricType, from, to)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"error": err.Error()})
		return
	}
	if rows == nil {
		rows = []map[string]interface{}{}
	}

	filtered := filterFieldsList(rows, include, exclude)
	writeJSON(w, http.StatusOK, filtered)
}

func (s *Server) handleGetWorkouts(w http.ResponseWriter, r *http.Request) {
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")
	include := r.URL.Query().Get("include")
	exclude := r.URL.Query().Get("exclude")

	workouts, err := queryWorkouts(s.db, startDate, endDate)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Error fetching workouts"})
		return
	}
	if workouts == nil {
		workouts = []WorkoutSummary{}
	}

	if include != "" || exclude != "" {
		// Convert to maps for filtering
		data, _ := json.Marshal(workouts)
		var maps []map[string]interface{}
		json.Unmarshal(data, &maps)
		filtered := filterFieldsList(maps, include, exclude)
		writeJSON(w, http.StatusOK, filtered)
		return
	}
	writeJSON(w, http.StatusOK, workouts)
}

func (s *Server) handleGetWorkoutDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	include := r.URL.Query().Get("include")
	exclude := r.URL.Query().Get("exclude")

	detail, err := queryWorkoutDetail(s.db, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if detail == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Workout not found"})
		return
	}

	if include != "" || exclude != "" {
		data, _ := json.Marshal(detail)
		var m map[string]interface{}
		json.Unmarshal(data, &m)
		filtered := filterFields(m, include, exclude)
		writeJSON(w, http.StatusOK, filtered)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// writeJSON writes a JSON response with proper Content-Type.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// filterFieldsList applies include/exclude filtering to a slice of maps.
func filterFieldsList(rows []map[string]interface{}, include, exclude string) []map[string]interface{} {
	if include == "" && exclude == "" {
		return rows
	}
	result := make([]map[string]interface{}, len(rows))
	for i, row := range rows {
		result[i] = filterFields(row, include, exclude)
	}
	return result
}

func filterFields(obj map[string]interface{}, include, exclude string) map[string]interface{} {
	if include != "" {
		fields := strings.Split(include, ",")
		filtered := make(map[string]interface{}, len(fields))
		for _, f := range fields {
			f = strings.TrimSpace(f)
			if v, ok := obj[f]; ok {
				filtered[f] = v
			}
		}
		return filtered
	}
	if exclude != "" {
		fields := strings.Split(exclude, ",")
		filtered := make(map[string]interface{}, len(obj))
		for k, v := range obj {
			filtered[k] = v
		}
		for _, f := range fields {
			delete(filtered, strings.TrimSpace(f))
		}
		return filtered
	}
	return obj
}
