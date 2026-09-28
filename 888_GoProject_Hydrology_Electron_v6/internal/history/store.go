package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"modbus-tcp-driver-v3/internal/io"
	"modbus-tcp-driver-v3/internal/model"

	_ "modernc.org/sqlite"
)

const defaultQueryMaxPoints = 2000
const archiveBatchSize = 500

type Store struct {
	db          *sql.DB
	archiveDB   *sql.DB
	cfg         model.HistoryConfig
	maintenance sync.Mutex
}

func defaultHistoryConfig() model.HistoryConfig {
	return model.HistoryConfig{
		Enabled:               true,
		SampleIntervalMinutes: 1,
		RetentionDays:         730,
		ArchiveEnabled:        true,
		ArchiveDir:            "data/archive",
		ArchiveRetentionDays:  3650,
		CleanupIntervalHours:  24,
		QueryMaxPoints:        defaultQueryMaxPoints,
	}
}

func normalizeHistoryConfig(cfg model.HistoryConfig) model.HistoryConfig {
	d := defaultHistoryConfig()
	if cfg.SampleIntervalMinutes <= 0 {
		cfg.SampleIntervalMinutes = d.SampleIntervalMinutes
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = d.RetentionDays
	}
	if cfg.ArchiveDir == "" {
		cfg.ArchiveDir = d.ArchiveDir
	}
	if cfg.ArchiveRetentionDays <= 0 {
		cfg.ArchiveRetentionDays = d.ArchiveRetentionDays
	}
	if cfg.CleanupIntervalHours <= 0 {
		cfg.CleanupIntervalHours = d.CleanupIntervalHours
	}
	if cfg.QueryMaxPoints <= 0 {
		cfg.QueryMaxPoints = d.QueryMaxPoints
	}
	if cfg.ArchiveRetentionDays < cfg.RetentionDays {
		cfg.ArchiveRetentionDays = cfg.RetentionDays
	}
	return cfg
}

func Open(path string) (*Store, error) {
	return OpenWithConfig(path, defaultHistoryConfig())
}

func OpenWithConfig(path string, cfg model.HistoryConfig) (*Store, error) {
	cfg = normalizeHistoryConfig(cfg)
	path = strings.TrimSpace(path)
	if path == "" {
		path = filepath.Join("data", "history.db")
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create history db directory %s: %w", dir, err)
		}
	}

	db, err := openDatabase(path)
	if err != nil {
		return nil, err
	}
	if err := ensureSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	var archiveDB *sql.DB
	if cfg.ArchiveEnabled {
		if err := os.MkdirAll(cfg.ArchiveDir, 0o755); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("create history archive directory %s: %w", cfg.ArchiveDir, err)
		}
		archivePath := filepath.Join(cfg.ArchiveDir, "history_archive.db")
		archiveDB, err = openDatabase(archivePath)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("open history archive database: %w", err)
		}
		if err := ensureSchema(archiveDB); err != nil {
			_ = archiveDB.Close()
			_ = db.Close()
			return nil, fmt.Errorf("create history archive schema: %w", err)
		}
	}

	if err := db.Ping(); err != nil {
		if archiveDB != nil {
			_ = archiveDB.Close()
		}
		_ = db.Close()
		return nil, fmt.Errorf("ping history database: %w", err)
	}
	return &Store{db: db, archiveDB: archiveDB, cfg: cfg}, nil
}

func openDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open history database %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	for _, pragma := range []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA synchronous=NORMAL`,
		`PRAGMA busy_timeout=5000`,
		`PRAGMA foreign_keys=ON`,
	} {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("configure history database: %w", err)
		}
	}
	return db, nil
}

func ensureSchema(db *sql.DB) error {
	if _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS point_history (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            tag TEXT NOT NULL,
            timestamp TEXT NOT NULL,
            value_json TEXT,
            quality TEXT NOT NULL,
            error TEXT
        )
    `); err != nil {
		return fmt.Errorf("create point_history table: %w", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_point_history_tag_time ON point_history(tag, timestamp DESC)`); err != nil {
		return fmt.Errorf("create history index: %w", err)
	}
	if _, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS alarm_events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            timestamp TEXT NOT NULL,
            type TEXT NOT NULL,
            tag TEXT NOT NULL,
            message TEXT,
            value_json TEXT,
            limit_value REAL,
            expected_value INTEGER,
            unit TEXT
        )
    `); err != nil {
		return fmt.Errorf("create alarm_events table: %w", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_alarm_events_tag_time ON alarm_events(tag, timestamp DESC)`); err != nil {
		return fmt.Errorf("create alarm event tag index: %w", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_alarm_events_time ON alarm_events(timestamp DESC)`); err != nil {
		return fmt.Errorf("create alarm event time index: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	var firstErr error
	if s.archiveDB != nil {
		if err := s.archiveDB.Close(); err != nil {
			firstErr = err
		}
	}
	if s.db != nil {
		if err := s.db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *Store) Append(tag string, sample io.Sample) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("history database is not initialized")
	}
	valueJSON, err := json.Marshal(sample.Value)
	if err != nil {
		return fmt.Errorf("marshal history value for %s: %w", tag, err)
	}
	_, err = s.db.Exec(
		`INSERT INTO point_history(tag, timestamp, value_json, quality, error) VALUES(?, ?, ?, ?, ?)`,
		tag,
		sample.Timestamp.UTC().Format(time.RFC3339Nano),
		string(valueJSON),
		string(sample.Quality),
		sample.Error,
	)
	if err != nil {
		return fmt.Errorf("insert history for %s: %w", tag, err)
	}
	return nil
}

func (s *Store) AppendAlarmEvent(event io.Event) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("history database is not initialized")
	}
	valueJSON, err := json.Marshal(event.Value)
	if err != nil {
		return fmt.Errorf("marshal alarm event value for %s: %w", event.Tag, err)
	}
	var expected any
	if event.Expected != nil {
		if *event.Expected {
			expected = 1
		} else {
			expected = 0
		}
	}
	_, err = s.db.Exec(
		`INSERT INTO alarm_events(timestamp, type, tag, message, value_json, limit_value, expected_value, unit) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		event.Timestamp.UTC().Format(time.RFC3339Nano),
		event.Type,
		event.Tag,
		event.Message,
		string(valueJSON),
		event.Limit,
		expected,
		event.Unit,
	)
	if err != nil {
		return fmt.Errorf("insert alarm event for %s: %w", event.Tag, err)
	}
	return nil
}

func (s *Store) QueryAlarmEvents(tag string, start, end time.Time) ([]io.Event, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("history database is not initialized")
	}
	if !start.Before(end) {
		return nil, fmt.Errorf("invalid alarm event time range")
	}

	result := make([]io.Event, 0)
	for _, db := range s.queryDBs(start, end) {
		events, err := queryAlarmEventsDB(db, "alarm_events", tag, start, end)
		if err != nil {
			return nil, err
		}
		result = append(result, events...)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Timestamp.Equal(result[j].Timestamp) {
			return result[i].ID < result[j].ID
		}
		return result[i].Timestamp.Before(result[j].Timestamp)
	})
	return result, nil
}

func queryAlarmEventsDB(db *sql.DB, table, tag string, start, end time.Time) ([]io.Event, error) {
	query := fmt.Sprintf(`SELECT id, timestamp, type, tag, COALESCE(message, ''), COALESCE(value_json, ''), limit_value, expected_value, COALESCE(unit, '')
             FROM %s
             WHERE timestamp >= ? AND timestamp < ?`, table)
	args := []any{start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano)}
	if strings.TrimSpace(tag) != "" {
		query += ` AND tag = ?`
		args = append(args, tag)
	}
	query += ` ORDER BY timestamp ASC, id ASC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query alarm events: %w", err)
	}
	defer rows.Close()

	result := make([]io.Event, 0)
	for rows.Next() {
		var event io.Event
		var timestampText, valueText string
		var limit sql.NullFloat64
		var expected sql.NullInt64
		if err := rows.Scan(&event.ID, &timestampText, &event.Type, &event.Tag, &event.Message, &valueText, &limit, &expected, &event.Unit); err != nil {
			return nil, fmt.Errorf("scan alarm event: %w", err)
		}
		event.Timestamp, err = time.Parse(time.RFC3339Nano, timestampText)
		if err != nil {
			return nil, fmt.Errorf("parse alarm event timestamp: %w", err)
		}
		if limit.Valid {
			v := limit.Float64
			event.Limit = &v
		}
		if expected.Valid {
			v := expected.Int64 != 0
			event.Expected = &v
		}
		if strings.TrimSpace(valueText) != "" && valueText != "null" {
			if err := json.Unmarshal([]byte(valueText), &event.Value); err != nil {
				return nil, fmt.Errorf("decode alarm event value: %w", err)
			}
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alarm events: %w", err)
	}
	return result, nil
}

func (s *Store) QueryRange(tag string, start, end time.Time, maxPoints int) ([]io.Sample, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("history database is not initialized")
	}
	if strings.TrimSpace(tag) == "" {
		return nil, fmt.Errorf("tag is required")
	}
	if !start.Before(end) {
		return nil, fmt.Errorf("invalid history time range")
	}
	if maxPoints <= 0 {
		maxPoints = s.cfg.QueryMaxPoints
	}
	if maxPoints <= 0 {
		maxPoints = defaultQueryMaxPoints
	}

	result := make([]io.Sample, 0)
	for _, db := range s.queryDBs(start, end) {
		samples, err := queryPointRangeDB(db, "point_history", tag, start, end, maxPoints)
		if err != nil {
			return nil, err
		}
		result = append(result, samples...)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Timestamp.Before(result[j].Timestamp) })
	if len(result) > maxPoints {
		result = downsampleSamples(result, maxPoints)
	}
	return result, nil
}

func (s *Store) queryDBs(start, end time.Time) []*sql.DB {
	result := make([]*sql.DB, 0, 2)
	if s.archiveDB != nil {
		result = append(result, s.archiveDB)
	}
	result = append(result, s.db)
	return result
}

func queryPointRangeDB(db *sql.DB, table, tag string, start, end time.Time, maxPoints int) ([]io.Sample, error) {
	var count int
	if err := db.QueryRow(
		fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE tag = ? AND timestamp >= ? AND timestamp < ?`, table),
		tag, start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano),
	).Scan(&count); err != nil {
		return nil, fmt.Errorf("count history range for %s: %w", tag, err)
	}
	step := 1
	if count > maxPoints {
		step = (count + maxPoints - 1) / maxPoints
	}

	query := fmt.Sprintf(`
        WITH numbered AS (
            SELECT timestamp, value_json, quality, COALESCE(error, '') AS error,
                   ROW_NUMBER() OVER (ORDER BY timestamp ASC, id ASC) AS rn,
                   COUNT(*) OVER () AS total
            FROM %s
            WHERE tag = ? AND timestamp >= ? AND timestamp < ?
        )
        SELECT timestamp, value_json, quality, error
        FROM numbered
        WHERE rn = 1 OR rn = total OR (rn %% ? = 0)
        ORDER BY timestamp ASC`, table)
	rows, err := db.Query(query, tag, start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano), step)
	if err != nil {
		return nil, fmt.Errorf("query history range for %s: %w", tag, err)
	}
	defer rows.Close()

	result := make([]io.Sample, 0, min(count, maxPoints+2))
	for rows.Next() {
		var timestampText, valueText, qualityText, errorText string
		if err := rows.Scan(&timestampText, &valueText, &qualityText, &errorText); err != nil {
			return nil, fmt.Errorf("scan history range for %s: %w", tag, err)
		}
		timestamp, err := time.Parse(time.RFC3339Nano, timestampText)
		if err != nil {
			return nil, fmt.Errorf("parse history timestamp for %s: %w", tag, err)
		}
		var value any
		if strings.TrimSpace(valueText) != "" && valueText != "null" {
			if err := json.Unmarshal([]byte(valueText), &value); err != nil {
				return nil, fmt.Errorf("decode history value for %s: %w", tag, err)
			}
		}
		result = append(result, io.Sample{Timestamp: timestamp, Value: value, Quality: io.Quality(qualityText), Error: errorText})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate history range for %s: %w", tag, err)
	}
	return result, nil
}

func downsampleSamples(samples []io.Sample, maxPoints int) []io.Sample {
	if maxPoints <= 0 || len(samples) <= maxPoints {
		return samples
	}
	result := make([]io.Sample, 0, maxPoints)
	step := float64(len(samples)-1) / float64(maxPoints-1)
	last := -1
	for i := 0; i < maxPoints; i++ {
		idx := int(float64(i) * step)
		if idx == last {
			continue
		}
		result = append(result, samples[idx])
		last = idx
	}
	if len(result) > maxPoints {
		result = result[:maxPoints]
	}
	return result
}

func (s *Store) Query(tag string, limit int) ([]io.Sample, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("history database is not initialized")
	}
	if limit <= 0 {
		limit = 60
	}
	if limit > 5000 {
		limit = 5000
	}

	rows, err := s.db.Query(
		`SELECT timestamp, value_json, quality, COALESCE(error, '')
         FROM point_history
         WHERE tag = ?
         ORDER BY timestamp DESC
         LIMIT ?`,
		tag, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query history for %s: %w", tag, err)
	}
	defer rows.Close()

	result := make([]io.Sample, 0, limit)
	for rows.Next() {
		var timestampText, valueText, qualityText, errorText string
		if err := rows.Scan(&timestampText, &valueText, &qualityText, &errorText); err != nil {
			return nil, fmt.Errorf("scan history for %s: %w", tag, err)
		}
		timestamp, err := time.Parse(time.RFC3339Nano, timestampText)
		if err != nil {
			return nil, fmt.Errorf("parse history timestamp for %s: %w", tag, err)
		}
		var value any
		if strings.TrimSpace(valueText) != "" && valueText != "null" {
			if err := json.Unmarshal([]byte(valueText), &value); err != nil {
				return nil, fmt.Errorf("decode history value for %s: %w", tag, err)
			}
		}
		result = append(result, io.Sample{Timestamp: timestamp, Value: value, Quality: io.Quality(qualityText), Error: errorText})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate history for %s: %w", tag, err)
	}
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}

func (s *Store) StartMaintenance(ctx context.Context) {
	interval := time.Duration(s.cfg.CleanupIntervalHours) * time.Hour
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	if err := s.RunMaintenance(time.Now().UTC()); err != nil {
		log.Printf("history maintenance failed: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := s.RunMaintenance(now.UTC()); err != nil {
				log.Printf("history maintenance failed: %v", err)
			}
		}
	}
}

func (s *Store) RunMaintenance(now time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("history database is not initialized")
	}
	s.maintenance.Lock()
	defer s.maintenance.Unlock()

	now = now.UTC()
	cutoff := now.Add(-time.Duration(s.cfg.RetentionDays) * 24 * time.Hour)
	archiveBefore := cutoff.Format(time.RFC3339Nano)
	var pointArchived, alarmArchived, pointDeleted, alarmDeleted int64

	var err error
	if s.cfg.ArchiveEnabled && s.archiveDB != nil {
		pointArchived, err = s.archiveTable("point_history", archiveBefore)
		if err != nil {
			return err
		}
		alarmArchived, err = s.archiveTable("alarm_events", archiveBefore)
		if err != nil {
			return err
		}
	}

	pointDeleted, err = deleteBefore(s.db, "point_history", archiveBefore)
	if err != nil {
		return err
	}
	alarmDeleted, err = deleteBefore(s.db, "alarm_events", archiveBefore)
	if err != nil {
		return err
	}

	if s.cfg.ArchiveEnabled && s.archiveDB != nil {
		archiveCutoff := now.Add(-time.Duration(s.cfg.ArchiveRetentionDays) * 24 * time.Hour)
		if _, err := deleteBefore(s.archiveDB, "point_history", archiveCutoff.Format(time.RFC3339Nano)); err != nil {
			return err
		}
		if _, err := deleteBefore(s.archiveDB, "alarm_events", archiveCutoff.Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}

	if pointArchived > 0 || alarmArchived > 0 || pointDeleted > 0 || alarmDeleted > 0 {
		log.Printf("history maintenance: archived point=%d alarm=%d, deleted main point=%d alarm=%d", pointArchived, alarmArchived, pointDeleted, alarmDeleted)
	}
	return nil
}

func (s *Store) archiveTable(table string, before string) (int64, error) {
	if s.archiveDB == nil {
		return 0, nil
	}
	var total int64
	for {
		var rows *sql.Rows
		var err error
		switch table {
		case "point_history":
			rows, err = s.db.Query(`SELECT id, tag, timestamp, value_json, quality, COALESCE(error, '') FROM point_history WHERE timestamp < ? ORDER BY id LIMIT ?`, before, archiveBatchSize)
		case "alarm_events":
			rows, err = s.db.Query(`SELECT id, timestamp, type, tag, COALESCE(message, ''), value_json, limit_value, expected_value, COALESCE(unit, '') FROM alarm_events WHERE timestamp < ? ORDER BY id LIMIT ?`, before, archiveBatchSize)
		default:
			return total, fmt.Errorf("unsupported history archive table %q", table)
		}
		if err != nil {
			return total, fmt.Errorf("read rows for archive %s: %w", table, err)
		}

		ids := make([]int64, 0, archiveBatchSize)
		tx, err := s.archiveDB.Begin()
		if err != nil {
			rows.Close()
			return total, fmt.Errorf("begin archive transaction %s: %w", table, err)
		}
		var insertErr error
		if table == "point_history" {
			stmt, err := tx.Prepare(`INSERT OR IGNORE INTO point_history(id, tag, timestamp, value_json, quality, error) VALUES(?, ?, ?, ?, ?, ?)`)
			if err != nil {
				rows.Close()
				_ = tx.Rollback()
				return total, fmt.Errorf("prepare point archive: %w", err)
			}
			for rows.Next() {
				var id int64
				var tag, timestamp, valueJSON, quality, errorText string
				if err := rows.Scan(&id, &tag, &timestamp, &valueJSON, &quality, &errorText); err != nil {
					insertErr = err
					break
				}
				if _, err := stmt.Exec(id, tag, timestamp, valueJSON, quality, errorText); err != nil {
					insertErr = err
					break
				}
				ids = append(ids, id)
			}
			_ = stmt.Close()
		} else {
			stmt, err := tx.Prepare(`INSERT OR IGNORE INTO alarm_events(id, timestamp, type, tag, message, value_json, limit_value, expected_value, unit) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`)
			if err != nil {
				rows.Close()
				_ = tx.Rollback()
				return total, fmt.Errorf("prepare alarm archive: %w", err)
			}
			for rows.Next() {
				var id int64
				var timestamp, typ, tag, message, valueJSON, unit string
				var limit sql.NullFloat64
				var expected sql.NullInt64
				if err := rows.Scan(&id, &timestamp, &typ, &tag, &message, &valueJSON, &limit, &expected, &unit); err != nil {
					insertErr = err
					break
				}
				if _, err := stmt.Exec(id, timestamp, typ, tag, message, valueJSON, nullableFloat(limit), nullableInt(expected), unit); err != nil {
					insertErr = err
					break
				}
				ids = append(ids, id)
			}
			_ = stmt.Close()
		}
		closeErr := rows.Close()
		if insertErr == nil && closeErr != nil {
			insertErr = closeErr
		}
		if insertErr != nil {
			_ = tx.Rollback()
			return total, fmt.Errorf("archive %s rows: %w", table, insertErr)
		}
		if err := tx.Commit(); err != nil {
			return total, fmt.Errorf("commit archive %s: %w", table, err)
		}
		if len(ids) == 0 {
			break
		}
		if err := deleteIDs(s.db, table, ids); err != nil {
			return total, err
		}
		total += int64(len(ids))
		if len(ids) < archiveBatchSize {
			break
		}
	}
	return total, nil
}

func nullableFloat(v sql.NullFloat64) any {
	if !v.Valid {
		return nil
	}
	return v.Float64
}

func nullableInt(v sql.NullInt64) any {
	if !v.Valid {
		return nil
	}
	return v.Int64
}

func deleteIDs(db *sql.DB, table string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", table, strings.Join(placeholders, ","))
	if _, err := db.Exec(query, args...); err != nil {
		return fmt.Errorf("delete archived rows from %s: %w", table, err)
	}
	return nil
}

func deleteBefore(db *sql.DB, table, before string) (int64, error) {
	result, err := db.Exec(fmt.Sprintf("DELETE FROM %s WHERE timestamp < ?", table), before)
	if err != nil {
		return 0, fmt.Errorf("cleanup %s: %w", table, err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
