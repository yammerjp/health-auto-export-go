package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadIngestPayloadFromJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "export.json")
	if err := os.WriteFile(path, []byte(`{"data":{"metrics":[{"name":"step_count","units":"count","data":[{"qty":123,"date":"2026-04-21 00:00:00 +0900","source":"Watch"}]}],"workouts":[]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	payload, err := loadIngestPayload(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Metrics) != 1 {
		t.Fatalf("expected 1 metric group, got %d", len(payload.Data.Metrics))
	}
}

func TestLoadIngestPayloadFromZIP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "export.zip")

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(file)
	w, err := zw.Create("HealthAutoExport.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(`{"data":{"metrics":[],"workouts":[{"id":"W-001","name":"Run","start":"2026-04-21 07:00:00 +0900","end":"2026-04-21 07:30:00 +0900","duration":1800}]}}`)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	payload, err := loadIngestPayload(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Workouts) != 1 {
		t.Fatalf("expected 1 workout, got %d", len(payload.Data.Workouts))
	}
}

func TestReadJSONFromZipRejectsMultipleJSONFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "export.zip")

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(file)
	for _, name := range []string{"a.json", "b.json"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(`{}`)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := readJSONFromZip(path); err == nil {
		t.Fatal("expected error for multiple json files")
	}
}
