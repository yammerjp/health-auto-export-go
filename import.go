package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func runImport(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: health-auto-export import <export.json|export.zip>")
	}

	dbPath := envOr("DB_PATH", "health.db")
	db, err := openDB(dbPath)
	if err != nil {
		return fmt.Errorf("open db %q: %w", dbPath, err)
	}
	defer db.Close()

	payload, err := loadIngestPayload(args[0])
	if err != nil {
		return err
	}

	metricGroups, err := saveMetrics(db, payload.Data.Metrics)
	if err != nil {
		return fmt.Errorf("import metrics: %w", err)
	}

	workouts, routes, err := saveWorkouts(db, payload.Data.Workouts)
	if err != nil {
		return fmt.Errorf("import workouts: %w", err)
	}

	fmt.Printf(
		"Imported %d metric groups and %d workouts (%d routes) into %s\n",
		metricGroups, workouts, routes, dbPath,
	)
	return nil
}

func loadIngestPayload(path string) (*IngestPayload, error) {
	data, err := readExportFile(path)
	if err != nil {
		return nil, err
	}

	var payload IngestPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode export %q: %w", path, err)
	}
	return &payload, nil
}

func readExportFile(path string) ([]byte, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".zip":
		return readJSONFromZip(path)
	case ".json":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", path, err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported import file %q", path)
	}
}

func readJSONFromZip(path string) ([]byte, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip %q: %w", path, err)
	}
	defer zr.Close()

	var jsonFile *zip.File
	for _, f := range zr.File {
		if strings.EqualFold(filepath.Ext(f.Name), ".json") {
			if jsonFile != nil {
				return nil, fmt.Errorf("zip %q contains multiple json files", path)
			}
			jsonFile = f
		}
	}
	if jsonFile == nil {
		return nil, fmt.Errorf("zip %q does not contain a json export", path)
	}

	rc, err := jsonFile.Open()
	if err != nil {
		return nil, fmt.Errorf("open %q in zip: %w", jsonFile.Name, err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read %q in zip: %w", jsonFile.Name, err)
	}
	return data, nil
}
