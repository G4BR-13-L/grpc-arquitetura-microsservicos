package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type RequestLog struct {
	RunID         string
	Protocol      string
	RequestUUID   string
	InputValue    int64
	OutputValue   int64
	RoundTripNS   int64
	StartedAtUTC  time.Time
	FinishedAtUTC time.Time
}

type ProcessingLog struct {
	RunID          string
	Protocol       string
	RequestUUID    string
	InputValue     int64
	OutputValue    int64
	ProcessingNS   int64
	ReceivedAtUTC  time.Time
	CompletedAtUTC time.Time
}

type BenchmarkRun struct {
	RunID          string
	Protocol       string
	TotalRequests  int
	TotalElapsedNS int64
	StartedAtUTC   time.Time
	FinishedAtUTC  time.Time
}

type TableCounts struct {
	RequestLogs    int
	ProcessingLogs int
	BenchmarkRuns  int
}

type ProtocolStats struct {
	Protocol string
	Count    int
	AvgNS    int64
	MinNS    int64
	MaxNS    int64
}

type RoundTripStats struct {
	Protocol string
	Count    int
	AvgNS    int64
	MinNS    int64
	MaxNS    int64
}

type ProtocolSnapshot struct {
	RunID           string
	Protocol        string
	RequestStats    RoundTripStats
	ProcessingStats ProtocolStats
	Runs            []BenchmarkRunSnapshot
}

type BenchmarkRunSnapshot struct {
	BenchmarkRun
	RequestStats RoundTripStats
}

func OpenSQLite(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_busy_timeout=10000&_journal_mode=DELETE&_synchronous=NORMAL&_foreign_keys=on", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func InitSchema(ctx context.Context, db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS request_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id TEXT NOT NULL,
	protocol TEXT NOT NULL,
	request_uuid TEXT NOT NULL,
	input_value INTEGER NOT NULL,
	output_value INTEGER NOT NULL,
	round_trip_ns INTEGER NOT NULL,
	started_at_utc TEXT NOT NULL,
	finished_at_utc TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS processing_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id TEXT NOT NULL,
	protocol TEXT NOT NULL,
	request_uuid TEXT NOT NULL,
	input_value INTEGER NOT NULL,
	output_value INTEGER NOT NULL,
	processing_ns INTEGER NOT NULL,
	received_at_utc TEXT NOT NULL,
	completed_at_utc TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS benchmark_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id TEXT NOT NULL,
	protocol TEXT NOT NULL,
	total_requests INTEGER NOT NULL,
	total_elapsed_ns INTEGER NOT NULL,
	started_at_utc TEXT NOT NULL,
	finished_at_utc TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_request_logs_protocol ON request_logs(protocol);
CREATE INDEX IF NOT EXISTS idx_processing_logs_protocol ON processing_logs(protocol);
CREATE INDEX IF NOT EXISTS idx_request_logs_run_id ON request_logs(run_id);
CREATE INDEX IF NOT EXISTS idx_processing_logs_run_id ON processing_logs(run_id);
CREATE INDEX IF NOT EXISTS idx_benchmark_runs_run_id ON benchmark_runs(run_id);
CREATE INDEX IF NOT EXISTS idx_request_logs_uuid ON request_logs(request_uuid);
CREATE INDEX IF NOT EXISTS idx_processing_logs_uuid ON processing_logs(request_uuid);
`

	_, err := db.ExecContext(ctx, schema)
	return err
}

func InsertRequestLogs(ctx context.Context, db *sql.DB, logs []RequestLog) error {
	if len(logs) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO request_logs (
	run_id,
	protocol,
	request_uuid,
	input_value,
	output_value,
	round_trip_ns,
	started_at_utc,
	finished_at_utc
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, log := range logs {
		if _, err := stmt.ExecContext(
			ctx,
			log.RunID,
			log.Protocol,
			log.RequestUUID,
			log.InputValue,
			log.OutputValue,
			log.RoundTripNS,
			log.StartedAtUTC.UTC().Format(time.RFC3339Nano),
			log.FinishedAtUTC.UTC().Format(time.RFC3339Nano),
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func InsertProcessingLogs(ctx context.Context, db *sql.DB, logs []ProcessingLog) error {
	if len(logs) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO processing_logs (
	run_id,
	protocol,
	request_uuid,
	input_value,
	output_value,
	processing_ns,
	received_at_utc,
	completed_at_utc
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, log := range logs {
		if _, err := stmt.ExecContext(
			ctx,
			log.RunID,
			log.Protocol,
			log.RequestUUID,
			log.InputValue,
			log.OutputValue,
			log.ProcessingNS,
			log.ReceivedAtUTC.UTC().Format(time.RFC3339Nano),
			log.CompletedAtUTC.UTC().Format(time.RFC3339Nano),
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func InsertBenchmarkRun(ctx context.Context, db *sql.DB, run BenchmarkRun) error {
	_, err := db.ExecContext(ctx, `
INSERT INTO benchmark_runs (
	run_id,
	protocol,
	total_requests,
	total_elapsed_ns,
	started_at_utc,
	finished_at_utc
) VALUES (?, ?, ?, ?, ?, ?)
`,
		run.RunID,
		run.Protocol,
		run.TotalRequests,
		run.TotalElapsedNS,
		run.StartedAtUTC.UTC().Format(time.RFC3339Nano),
		run.FinishedAtUTC.UTC().Format(time.RFC3339Nano),
	)

	return err
}

func GetTableCounts(ctx context.Context, db *sql.DB, runID string) (TableCounts, error) {
	var counts TableCounts
	if err := countInto(ctx, db, "SELECT COUNT(*) FROM request_logs WHERE run_id = ?", &counts.RequestLogs, runID); err != nil {
		return counts, err
	}
	if err := countInto(ctx, db, "SELECT COUNT(*) FROM processing_logs WHERE run_id = ?", &counts.ProcessingLogs, runID); err != nil {
		return counts, err
	}
	if err := countInto(ctx, db, "SELECT COUNT(*) FROM benchmark_runs WHERE run_id = ?", &counts.BenchmarkRuns, runID); err != nil {
		return counts, err
	}
	return counts, nil
}

func GetTotalTableCounts(ctx context.Context, db *sql.DB) (TableCounts, error) {
	var counts TableCounts
	if err := countInto(ctx, db, "SELECT COUNT(*) FROM request_logs", &counts.RequestLogs); err != nil {
		return counts, err
	}
	if err := countInto(ctx, db, "SELECT COUNT(*) FROM processing_logs", &counts.ProcessingLogs); err != nil {
		return counts, err
	}
	if err := countInto(ctx, db, "SELECT COUNT(*) FROM benchmark_runs", &counts.BenchmarkRuns); err != nil {
		return counts, err
	}
	return counts, nil
}

func GetProcessingStats(ctx context.Context, db *sql.DB, runID string) (map[string]ProtocolStats, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	protocol,
	COUNT(*) AS total,
	CAST(AVG(processing_ns) AS INTEGER) AS avg_ns,
	MIN(processing_ns) AS min_ns,
	MAX(processing_ns) AS max_ns
FROM processing_logs
WHERE run_id = ?
GROUP BY protocol
`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]ProtocolStats)
	for rows.Next() {
		var item ProtocolStats
		if err := rows.Scan(&item.Protocol, &item.Count, &item.AvgNS, &item.MinNS, &item.MaxNS); err != nil {
			return nil, err
		}
		stats[item.Protocol] = item
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

func DeleteProtocolDataExceptRun(ctx context.Context, db *sql.DB, protocol string, keepRunID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)

	queries := []string{
		"DELETE FROM request_logs WHERE protocol = ? AND run_id <> ?",
		"DELETE FROM processing_logs WHERE protocol = ? AND run_id <> ?",
		"DELETE FROM benchmark_runs WHERE protocol = ? AND run_id <> ?",
	}
	for _, query := range queries {
		if _, err := tx.ExecContext(ctx, query, protocol, keepRunID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func GetLatestProtocolSnapshot(ctx context.Context, db *sql.DB, protocol string) (ProtocolSnapshot, bool, error) {
	runID, err := getLatestRunIDForProtocol(ctx, db, protocol)
	if err != nil {
		return ProtocolSnapshot{}, false, err
	}
	if runID == "" {
		return ProtocolSnapshot{}, false, nil
	}

	requestStats, err := getRoundTripStats(ctx, db, runID, protocol)
	if err != nil {
		return ProtocolSnapshot{}, false, err
	}

	processingStats, err := getProcessingStatsForRun(ctx, db, runID, protocol)
	if err != nil {
		return ProtocolSnapshot{}, false, err
	}

	runs, err := getBenchmarkRunsForProtocol(ctx, db, runID, protocol)
	if err != nil {
		return ProtocolSnapshot{}, false, err
	}

	return ProtocolSnapshot{
		RunID:           runID,
		Protocol:        protocol,
		RequestStats:    requestStats,
		ProcessingStats: processingStats,
		Runs:            runs,
	}, true, nil
}

func getLatestRunIDForProtocol(ctx context.Context, db *sql.DB, protocol string) (string, error) {
	var runID string
	err := db.QueryRowContext(ctx, `
SELECT run_id
FROM benchmark_runs
WHERE protocol = ?
ORDER BY finished_at_utc DESC, id DESC
LIMIT 1
`, protocol).Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return runID, err
}

func getRoundTripStats(ctx context.Context, db *sql.DB, runID string, protocol string) (RoundTripStats, error) {
	var stats RoundTripStats
	err := db.QueryRowContext(ctx, `
SELECT
	protocol,
	COUNT(*) AS total,
	CAST(AVG(round_trip_ns) AS INTEGER) AS avg_ns,
	MIN(round_trip_ns) AS min_ns,
	MAX(round_trip_ns) AS max_ns
FROM request_logs
WHERE run_id = ? AND protocol = ?
GROUP BY protocol
`, runID, protocol).Scan(&stats.Protocol, &stats.Count, &stats.AvgNS, &stats.MinNS, &stats.MaxNS)
	if errors.Is(err, sql.ErrNoRows) {
		return RoundTripStats{}, nil
	}
	return stats, err
}

func getProcessingStatsForRun(ctx context.Context, db *sql.DB, runID string, protocol string) (ProtocolStats, error) {
	var stats ProtocolStats
	err := db.QueryRowContext(ctx, `
SELECT
	protocol,
	COUNT(*) AS total,
	CAST(AVG(processing_ns) AS INTEGER) AS avg_ns,
	MIN(processing_ns) AS min_ns,
	MAX(processing_ns) AS max_ns
FROM processing_logs
WHERE run_id = ? AND protocol = ?
GROUP BY protocol
`, runID, protocol).Scan(&stats.Protocol, &stats.Count, &stats.AvgNS, &stats.MinNS, &stats.MaxNS)
	if errors.Is(err, sql.ErrNoRows) {
		return ProtocolStats{Protocol: protocol}, nil
	}
	return stats, err
}

func getBenchmarkRunsForProtocol(ctx context.Context, db *sql.DB, runID string, protocol string) ([]BenchmarkRunSnapshot, error) {
	rows, err := db.QueryContext(ctx, `
SELECT run_id, protocol, total_requests, total_elapsed_ns, started_at_utc, finished_at_utc
FROM benchmark_runs
WHERE run_id = ? AND protocol = ?
ORDER BY started_at_utc ASC, id ASC
`, runID, protocol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := make([]BenchmarkRunSnapshot, 0)
	for rows.Next() {
		var run BenchmarkRun
		var startedAt string
		var finishedAt string
		if err := rows.Scan(&run.RunID, &run.Protocol, &run.TotalRequests, &run.TotalElapsedNS, &startedAt, &finishedAt); err != nil {
			return nil, err
		}

		run.StartedAtUTC, err = time.Parse(time.RFC3339Nano, startedAt)
		if err != nil {
			return nil, err
		}
		run.FinishedAtUTC, err = time.Parse(time.RFC3339Nano, finishedAt)
		if err != nil {
			return nil, err
		}

		requestStats, err := getRoundTripStatsForWindow(ctx, db, run.RunID, protocol, run.StartedAtUTC, run.FinishedAtUTC)
		if err != nil {
			return nil, err
		}

		runs = append(runs, BenchmarkRunSnapshot{
			BenchmarkRun: run,
			RequestStats: requestStats,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return runs, nil
}

func getRoundTripStatsForWindow(ctx context.Context, db *sql.DB, runID string, protocol string, startedAt time.Time, finishedAt time.Time) (RoundTripStats, error) {
	var stats RoundTripStats
	err := db.QueryRowContext(ctx, `
SELECT
	protocol,
	COUNT(*) AS total,
	CAST(AVG(round_trip_ns) AS INTEGER) AS avg_ns,
	MIN(round_trip_ns) AS min_ns,
	MAX(round_trip_ns) AS max_ns
FROM request_logs
WHERE run_id = ?
  AND protocol = ?
  AND started_at_utc >= ?
  AND finished_at_utc <= ?
GROUP BY protocol
`,
		runID,
		protocol,
		startedAt.UTC().Format(time.RFC3339Nano),
		finishedAt.UTC().Format(time.RFC3339Nano),
	).Scan(&stats.Protocol, &stats.Count, &stats.AvgNS, &stats.MinNS, &stats.MaxNS)
	if errors.Is(err, sql.ErrNoRows) {
		return RoundTripStats{Protocol: protocol}, nil
	}
	return stats, err
}

func countInto(ctx context.Context, db *sql.DB, query string, target *int, args ...any) error {
	return db.QueryRowContext(ctx, query, args...).Scan(target)
}

func rollback(tx *sql.Tx) {
	if tx == nil {
		return
	}
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return
	}
}
