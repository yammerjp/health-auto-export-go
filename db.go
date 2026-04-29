package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS metrics (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    qty REAL NOT NULL,
    units TEXT NOT NULL,
    date TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    metadata TEXT,
    UNIQUE(name, date, source)
);

CREATE TABLE IF NOT EXISTS blood_pressure (
    id INTEGER PRIMARY KEY,
    systolic REAL NOT NULL,
    diastolic REAL NOT NULL,
    units TEXT NOT NULL,
    date TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    metadata TEXT,
    UNIQUE(date, source)
);

CREATE TABLE IF NOT EXISTS heart_rate (
    id INTEGER PRIMARY KEY,
    min REAL NOT NULL,
    avg REAL NOT NULL,
    max REAL NOT NULL,
    units TEXT NOT NULL,
    date TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    metadata TEXT,
    UNIQUE(date, source)
);

CREATE TABLE IF NOT EXISTS sleep_analysis (
    id INTEGER PRIMARY KEY,
    date TEXT NOT NULL,
    in_bed_start TEXT NOT NULL DEFAULT '',
    in_bed_end TEXT NOT NULL DEFAULT '',
    sleep_start TEXT NOT NULL DEFAULT '',
    sleep_end TEXT NOT NULL DEFAULT '',
    core REAL NOT NULL DEFAULT 0,
    rem REAL NOT NULL DEFAULT 0,
    deep REAL NOT NULL DEFAULT 0,
    awake REAL NOT NULL DEFAULT 0,
    in_bed REAL NOT NULL DEFAULT 0,
    asleep REAL NOT NULL DEFAULT 0,
    total_sleep REAL NOT NULL DEFAULT 0,
    units TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    metadata TEXT,
    UNIQUE(date, source)
);

CREATE TABLE IF NOT EXISTS workouts (
    id INTEGER PRIMARY KEY,
    workout_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    start TEXT NOT NULL,
    end TEXT NOT NULL,
    duration REAL NOT NULL,
    active_energy_burned_qty REAL,
    active_energy_burned_units TEXT,
    distance_qty REAL,
    distance_units TEXT,
    heart_rate_data TEXT,
    heart_rate_recovery TEXT,
    step_count TEXT,
    temperature_qty REAL,
    humidity_qty REAL,
    intensity_qty REAL,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS workout_routes (
    id INTEGER PRIMARY KEY,
    workout_id TEXT NOT NULL UNIQUE,
    locations TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
`)
	return err
}
