package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vpsfreecz/vpsf-status/config"

	_ "modernc.org/sqlite"
)

const (
	historyDefaultDays           = config.DefaultHistoryDays
	probeIncidentThreshold       = 5 * time.Minute
	historyDBFilename            = "history.sqlite3"
	historyOutagesFilename       = "outages.json"
	historyProbeFilename         = "probe_history.json"
	historySchemaVersion         = 1
	historyProbeStateOperational = "ok"
	historyProbeStateMaintenance = "maintenance"
	historyProbeStateDegraded    = "degraded"
	historyProbeStateDown        = "down"
	historyProbeStateError       = "error"
	historyEntityNode            = "node"
	historyEntityVpsAdmin        = "vpsadmin"
	historyEntityDnsResolver     = "dns_resolver"
	historyEntityWebService      = "web_service"
	historyEntityNameServer      = "nameserver"
	historySeverityOperational   = "ok"
	historySeverityMaintenance   = "maintenance"
	historySeverityOutage        = "outage"
)

type HistoryStore struct {
	dir string
	db  *sql.DB
	mu  sync.Mutex
}

type ProbeTarget struct {
	EntityKind  string `json:"entity_kind"`
	EntityID    string `json:"entity_id"`
	EntityLabel string `json:"entity_label"`
	Method      string `json:"method"`
}

type ProbeState struct {
	ProbeTarget
	Status           string    `json:"status"`
	Message          string    `json:"message"`
	Since            time.Time `json:"since"`
	LastSeen         time.Time `json:"last_seen"`
	IncidentPromoted bool      `json:"incident_promoted"`
}

type ProbeEvent struct {
	ProbeTarget
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	ChangedAt time.Time `json:"changed_at"`
}

type ProbeLogEvent struct {
	ProbeEvent
	EndsAt time.Time
}

type ProbeLogPage struct {
	Events []ProbeLogEvent
	Total  int
}

type ProbeIncident struct {
	ProbeTarget
	Status   string     `json:"status"`
	Message  string     `json:"message"`
	StartsAt time.Time  `json:"starts_at"`
	EndsAt   *time.Time `json:"ends_at,omitempty"`
}

type HistoryEntitySnapshot struct {
	EntityKind         string
	EntityID           string
	EntityLabel        string
	NodeID             int64
	GroupKind          string
	GroupID            int
	GroupLabel         string
	VpsAdminLocationID int64
	LastSeen           time.Time
}

var historyMigrations = []func(*sql.Tx) error{
	nil,
	migrateHistorySchemaV1,
}

func openHistoryStore(dir string) (*HistoryStore, error) {
	if dir == "" {
		return nil, errors.New("history directory is empty")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create history directory: %w", err)
	}

	if err := verifyHistoryDirectoryWritable(dir); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", filepath.Join(dir, historyDBFilename))
	if err != nil {
		return nil, fmt.Errorf("open history database: %w", err)
	}
	db.SetMaxOpenConns(1)

	hs := &HistoryStore{dir: dir, db: db}
	if err := hs.initializeDB(); err != nil {
		db.Close()
		return nil, err
	}

	return hs, nil
}

func (hs *HistoryStore) Close() error {
	if hs == nil || hs.db == nil {
		return nil
	}
	return hs.db.Close()
}

func verifyHistoryDirectoryWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return fmt.Errorf("history directory is not writable: %w", err)
	}

	path := f.Name()
	closeErr := f.Close()
	removeErr := os.Remove(path)

	if closeErr != nil {
		return fmt.Errorf("close history write test: %w", closeErr)
	}
	if removeErr != nil {
		return fmt.Errorf("remove history write test: %w", removeErr)
	}

	return nil
}

func (hs *HistoryStore) initializeDB() error {
	for _, stmt := range []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA foreign_keys = ON`,
	} {
		if _, err := hs.db.Exec(stmt); err != nil {
			return fmt.Errorf("configure history database: %w", err)
		}
	}

	if err := initializeHistorySchema(hs.db); err != nil {
		return fmt.Errorf("initialize history schema: %w", err)
	}

	return nil
}

func initializeHistorySchema(db *sql.DB) error {
	version, err := readHistorySchemaVersion(db)
	if err != nil {
		return err
	}

	if version > historySchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", version, historySchemaVersion)
	}

	for version < historySchemaVersion {
		next := version + 1
		if next >= len(historyMigrations) || historyMigrations[next] == nil {
			return fmt.Errorf("missing migration for history schema version %d", next)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin schema migration %d: %w", next, err)
		}

		if err := historyMigrations[next](tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrate history schema to version %d: %w", next, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", next)); err != nil {
			tx.Rollback()
			return fmt.Errorf("set history schema version %d: %w", next, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit history schema migration %d: %w", next, err)
		}

		version = next
	}

	if err := ensureHistorySchemaObjects(db); err != nil {
		return err
	}

	return nil
}

func readHistorySchemaVersion(db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read history schema version: %w", err)
	}
	return version, nil
}

func migrateHistorySchemaV1(tx *sql.Tx) error {
	for _, stmt := range historySchemaStatements() {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureHistorySchemaObjects(db *sql.DB) error {
	for _, stmt := range historySchemaStatements() {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("ensure history schema object: %w", err)
		}
	}
	return nil
}

func historySchemaStatements() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS outages (
			id INTEGER PRIMARY KEY,
			begins_at TEXT NOT NULL,
			finished_at TEXT,
			duration_minutes INTEGER NOT NULL,
			type TEXT NOT NULL,
			state TEXT NOT NULL,
			impact TEXT NOT NULL,
			cs_summary TEXT NOT NULL,
			cs_description TEXT NOT NULL,
			en_summary TEXT NOT NULL,
			en_description TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS outage_entities (
			outage_id INTEGER NOT NULL REFERENCES outages(id) ON DELETE CASCADE,
			position INTEGER NOT NULL,
			name TEXT NOT NULL,
			entity_id INTEGER NOT NULL,
			label TEXT NOT NULL,
			PRIMARY KEY (outage_id, position)
		)`,
		`CREATE TABLE IF NOT EXISTS probe_states (
			entity_kind TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			entity_label TEXT NOT NULL,
			method TEXT NOT NULL,
			status TEXT NOT NULL,
			message TEXT NOT NULL,
			since TEXT NOT NULL,
			last_seen TEXT NOT NULL,
			incident_promoted INTEGER NOT NULL,
			PRIMARY KEY (entity_kind, entity_id, method)
		)`,
		`CREATE TABLE IF NOT EXISTS probe_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entity_kind TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			entity_label TEXT NOT NULL,
			method TEXT NOT NULL,
			status TEXT NOT NULL,
			message TEXT NOT NULL,
			changed_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS probe_events_entity_changed_idx
			ON probe_events (entity_kind, entity_id, changed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS probe_events_entity_method_changed_idx
			ON probe_events (entity_kind, entity_id, method, changed_at DESC)`,
		`CREATE TABLE IF NOT EXISTS probe_incidents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entity_kind TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			entity_label TEXT NOT NULL,
			method TEXT NOT NULL,
			status TEXT NOT NULL,
			message TEXT NOT NULL,
			starts_at TEXT NOT NULL,
			ends_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS probe_incidents_entity_starts_idx
			ON probe_incidents (entity_kind, entity_id, starts_at)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS probe_incidents_open_unique_idx
			ON probe_incidents (entity_kind, entity_id, method, starts_at)
			WHERE ends_at IS NULL`,
		`CREATE TABLE IF NOT EXISTS entity_snapshots (
			entity_kind TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			entity_label TEXT NOT NULL,
			node_id INTEGER NOT NULL,
			group_kind TEXT NOT NULL,
			group_id INTEGER NOT NULL,
			group_label TEXT NOT NULL,
			vpsadmin_location_id INTEGER NOT NULL,
			last_seen TEXT NOT NULL,
			PRIMARY KEY (entity_kind, entity_id)
		)`,
		`CREATE INDEX IF NOT EXISTS entity_snapshots_node_id_idx
			ON entity_snapshots (entity_kind, node_id)`,
	}
}

func (hs *HistoryStore) ReplaceOutages(reports []*OutageReport, _ time.Time) error {
	if hs == nil {
		return nil
	}

	hs.mu.Lock()
	defer hs.mu.Unlock()

	tx, err := hs.db.Begin()
	if err != nil {
		return fmt.Errorf("begin outage history update: %w", err)
	}
	defer tx.Rollback()

	outageStmt, err := tx.Prepare(`
		INSERT INTO outages (
			id, begins_at, finished_at, duration_minutes, type, state, impact,
			cs_summary, cs_description, en_summary, en_description
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			begins_at = excluded.begins_at,
			finished_at = excluded.finished_at,
			duration_minutes = excluded.duration_minutes,
			type = excluded.type,
			state = excluded.state,
			impact = excluded.impact,
			cs_summary = excluded.cs_summary,
			cs_description = excluded.cs_description,
			en_summary = excluded.en_summary,
			en_description = excluded.en_description
	`)
	if err != nil {
		return fmt.Errorf("prepare outage insert: %w", err)
	}
	defer outageStmt.Close()

	entityStmt, err := tx.Prepare(`
		INSERT INTO outage_entities (outage_id, position, name, entity_id, label)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare outage entity insert: %w", err)
	}
	defer entityStmt.Close()

	for _, report := range reports {
		if report == nil || report.BeginsAt.IsZero() {
			continue
		}

		if _, err := outageStmt.Exec(
			report.Id,
			formatHistoryTime(report.BeginsAt),
			nullableHistoryTime(report.FinishedAt),
			int(report.Duration.Minutes()),
			report.Type,
			report.State,
			report.Impact,
			report.CsSummary,
			report.CsDescription,
			report.EnSummary,
			report.EnDescription,
		); err != nil {
			return fmt.Errorf("insert outage %d: %w", report.Id, err)
		}

		if _, err := tx.Exec(`DELETE FROM outage_entities WHERE outage_id = ?`, report.Id); err != nil {
			return fmt.Errorf("replace outage %d entities: %w", report.Id, err)
		}

		for i, entity := range report.AffectedEntities {
			if _, err := entityStmt.Exec(report.Id, i, entity.Name, entity.Id, entity.Label); err != nil {
				return fmt.Errorf("insert outage %d entity %d: %w", report.Id, i, err)
			}
		}
	}

	return tx.Commit()
}

func (hs *HistoryStore) OutageReports() []*OutageReport {
	if hs == nil {
		return nil
	}

	hs.mu.Lock()
	defer hs.mu.Unlock()

	rows, err := hs.db.Query(`
		SELECT id, begins_at, finished_at, duration_minutes, type, state, impact,
			cs_summary, cs_description, en_summary, en_description
		FROM outages
		ORDER BY begins_at ASC, id ASC
	`)
	if err != nil {
		return nil
	}

	type outageRow struct {
		report     *OutageReport
		finishedAt sql.NullString
	}
	outages := make([]outageRow, 0)

	for rows.Next() {
		var beginsAt string
		var durationMinutes int
		row := outageRow{report: &OutageReport{}}
		if err := rows.Scan(
			&row.report.Id,
			&beginsAt,
			&row.finishedAt,
			&durationMinutes,
			&row.report.Type,
			&row.report.State,
			&row.report.Impact,
			&row.report.CsSummary,
			&row.report.CsDescription,
			&row.report.EnSummary,
			&row.report.EnDescription,
		); err != nil {
			rows.Close()
			return nil
		}

		row.report.BeginsAt, err = parseHistoryTime(beginsAt)
		if err != nil {
			rows.Close()
			return nil
		}
		if row.finishedAt.Valid {
			row.report.FinishedAt, err = parseHistoryTime(row.finishedAt.String)
			if err != nil {
				rows.Close()
				return nil
			}
		}
		row.report.Duration = time.Duration(durationMinutes) * time.Minute
		outages = append(outages, row)
	}
	if err := rows.Close(); err != nil {
		return nil
	}
	if err := rows.Err(); err != nil {
		return nil
	}

	ids := make([]int64, 0, len(outages))
	for _, row := range outages {
		ids = append(ids, row.report.Id)
	}
	entities := hs.outageEntitiesForReportsLocked(ids)

	ret := make([]*OutageReport, 0, len(outages))
	for _, row := range outages {
		row.report.AffectedEntities = entities[row.report.Id]
		if row.report.AffectedEntities == nil {
			row.report.AffectedEntities = make([]OutageEntity, 0)
		}
		ret = append(ret, row.report)
	}

	return ret
}

func (hs *HistoryStore) outageEntitiesForReportsLocked(outageIDs []int64) map[int64][]OutageEntity {
	ret := make(map[int64][]OutageEntity, len(outageIDs))
	if len(outageIDs) == 0 {
		return ret
	}

	args := make([]any, len(outageIDs))
	for i, outageID := range outageIDs {
		args[i] = outageID
		ret[outageID] = make([]OutageEntity, 0)
	}

	rows, err := hs.db.Query(`
		SELECT outage_id, name, entity_id, label
		FROM outage_entities
		WHERE outage_id IN (`+sqlPlaceholders(len(outageIDs))+`)
		ORDER BY outage_id ASC, position ASC
	`, args...)
	if err != nil {
		return ret
	}
	defer rows.Close()

	for rows.Next() {
		var outageID int64
		var entity OutageEntity
		if err := rows.Scan(&outageID, &entity.Name, &entity.Id, &entity.Label); err != nil {
			return ret
		}
		ret[outageID] = append(ret[outageID], entity)
	}

	return ret
}

func (hs *HistoryStore) RecordEntitySnapshots(snapshots []HistoryEntitySnapshot) error {
	if hs == nil || len(snapshots) == 0 {
		return nil
	}

	hs.mu.Lock()
	defer hs.mu.Unlock()

	tx, err := hs.db.Begin()
	if err != nil {
		return fmt.Errorf("begin entity snapshot update: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO entity_snapshots (
			entity_kind, entity_id, entity_label, node_id, group_kind, group_id,
			group_label, vpsadmin_location_id, last_seen
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (entity_kind, entity_id) DO UPDATE SET
			entity_label = excluded.entity_label,
			node_id = excluded.node_id,
			group_kind = excluded.group_kind,
			group_id = excluded.group_id,
			group_label = excluded.group_label,
			vpsadmin_location_id = excluded.vpsadmin_location_id,
			last_seen = excluded.last_seen
	`)
	if err != nil {
		return fmt.Errorf("prepare entity snapshot upsert: %w", err)
	}
	defer stmt.Close()

	for _, snapshot := range snapshots {
		if snapshot.EntityKind == "" || snapshot.EntityID == "" || snapshot.GroupKind == "" || snapshot.GroupID == 0 {
			continue
		}
		if snapshot.EntityLabel == "" {
			snapshot.EntityLabel = snapshot.EntityID
		}
		if snapshot.LastSeen.IsZero() {
			snapshot.LastSeen = time.Now()
		}

		if _, err := stmt.Exec(
			snapshot.EntityKind,
			snapshot.EntityID,
			snapshot.EntityLabel,
			snapshot.NodeID,
			snapshot.GroupKind,
			snapshot.GroupID,
			snapshot.GroupLabel,
			snapshot.VpsAdminLocationID,
			formatHistoryTime(snapshot.LastSeen),
		); err != nil {
			return fmt.Errorf("upsert entity snapshot %s/%s: %w", snapshot.EntityKind, snapshot.EntityID, err)
		}
	}

	return tx.Commit()
}

func (hs *HistoryStore) EntitySnapshots() []HistoryEntitySnapshot {
	if hs == nil {
		return nil
	}

	hs.mu.Lock()
	defer hs.mu.Unlock()

	rows, err := hs.db.Query(`
		SELECT entity_kind, entity_id, entity_label, node_id, group_kind, group_id,
			group_label, vpsadmin_location_id, last_seen
		FROM entity_snapshots
		ORDER BY group_id ASC, entity_label ASC, entity_id ASC
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	ret := make([]HistoryEntitySnapshot, 0)
	for rows.Next() {
		var lastSeen string
		snapshot := HistoryEntitySnapshot{}
		if err := rows.Scan(
			&snapshot.EntityKind,
			&snapshot.EntityID,
			&snapshot.EntityLabel,
			&snapshot.NodeID,
			&snapshot.GroupKind,
			&snapshot.GroupID,
			&snapshot.GroupLabel,
			&snapshot.VpsAdminLocationID,
			&lastSeen,
		); err != nil {
			return nil
		}

		t, err := parseHistoryTime(lastSeen)
		if err != nil {
			return nil
		}
		snapshot.LastSeen = t
		ret = append(ret, snapshot)
	}

	return ret
}

func (hs *HistoryStore) RecordProbeStatus(target ProbeTarget, status string, message string, now time.Time) error {
	if hs == nil {
		return nil
	}
	if target.EntityKind == "" || target.EntityID == "" || target.Method == "" {
		return nil
	}
	if status == "" {
		status = historyProbeStateOperational
	}

	hs.mu.Lock()
	defer hs.mu.Unlock()

	tx, err := hs.db.Begin()
	if err != nil {
		return fmt.Errorf("begin probe history update: %w", err)
	}
	defer tx.Rollback()

	state, exists, err := queryProbeStateTx(tx, target)
	if err != nil {
		return err
	}

	changed := false
	if !exists {
		state = ProbeState{
			ProbeTarget: target,
			Status:      status,
			Message:     message,
			Since:       now,
			LastSeen:    now,
		}
		if err := upsertProbeStateTx(tx, state); err != nil {
			return err
		}
		if err := appendProbeEventTx(tx, target, status, message, now); err != nil {
			return err
		}
		changed = true
	} else if state.Status != status {
		if !isOperationalProbeState(state.Status) {
			closed, err := closeProbeStateIncidentTx(tx, state, now)
			if err != nil {
				return err
			}
			changed = closed || changed
		}

		state.ProbeTarget = target
		state.Status = status
		state.Message = message
		state.Since = now
		state.LastSeen = now
		state.IncidentPromoted = false
		if err := upsertProbeStateTx(tx, state); err != nil {
			return err
		}
		if err := appendProbeEventTx(tx, target, status, message, now); err != nil {
			return err
		}
		changed = true
	} else if !isOperationalProbeState(status) && !state.IncidentPromoted && !now.Before(state.Since.Add(probeIncidentThreshold)) {
		state.ProbeTarget = target
		state.Message = message
		state.LastSeen = now
		if err := openProbeIncidentTx(tx, state); err != nil {
			return err
		}
		state.IncidentPromoted = true
		if err := upsertProbeStateTx(tx, state); err != nil {
			return err
		}
		changed = true
	}

	if !changed {
		return nil
	}

	return tx.Commit()
}

func queryProbeStateTx(tx *sql.Tx, target ProbeTarget) (ProbeState, bool, error) {
	row := tx.QueryRow(`
		SELECT entity_kind, entity_id, entity_label, method, status, message, since,
			last_seen, incident_promoted
		FROM probe_states
		WHERE entity_kind = ? AND entity_id = ? AND method = ?
	`, target.EntityKind, target.EntityID, target.Method)

	state, err := scanProbeState(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ProbeState{}, false, nil
	}
	if err != nil {
		return ProbeState{}, false, err
	}

	return state, true, nil
}

func upsertProbeStateTx(tx *sql.Tx, state ProbeState) error {
	promoted := 0
	if state.IncidentPromoted {
		promoted = 1
	}

	_, err := tx.Exec(`
		INSERT INTO probe_states (
			entity_kind, entity_id, entity_label, method, status, message,
			since, last_seen, incident_promoted
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (entity_kind, entity_id, method) DO UPDATE SET
			entity_label = excluded.entity_label,
			status = excluded.status,
			message = excluded.message,
			since = excluded.since,
			last_seen = excluded.last_seen,
			incident_promoted = excluded.incident_promoted
	`,
		state.EntityKind,
		state.EntityID,
		state.EntityLabel,
		state.Method,
		state.Status,
		state.Message,
		formatHistoryTime(state.Since),
		formatHistoryTime(state.LastSeen),
		promoted,
	)
	if err != nil {
		return fmt.Errorf("store probe state: %w", err)
	}
	return nil
}

func appendProbeEventTx(tx *sql.Tx, target ProbeTarget, status string, message string, now time.Time) error {
	_, err := tx.Exec(`
		INSERT INTO probe_events (
			entity_kind, entity_id, entity_label, method, status, message, changed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		target.EntityKind,
		target.EntityID,
		target.EntityLabel,
		target.Method,
		status,
		message,
		formatHistoryTime(now),
	)
	if err != nil {
		return fmt.Errorf("append probe event: %w", err)
	}
	return nil
}

func closeProbeStateIncidentTx(tx *sql.Tx, state ProbeState, now time.Time) (bool, error) {
	if now.Before(state.Since.Add(probeIncidentThreshold)) {
		return false, nil
	}

	if !state.IncidentPromoted {
		end := now
		return true, insertProbeIncidentTx(tx, state.ProbeTarget, state.Status, state.Message, state.Since, &end)
	}

	res, err := tx.Exec(`
		UPDATE probe_incidents
		SET ends_at = ?
		WHERE entity_kind = ? AND entity_id = ? AND method = ? AND starts_at = ? AND ends_at IS NULL
	`,
		formatHistoryTime(now),
		state.EntityKind,
		state.EntityID,
		state.Method,
		formatHistoryTime(state.Since),
	)
	if err != nil {
		return false, fmt.Errorf("close probe incident: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("check closed probe incident: %w", err)
	}
	if affected > 0 {
		return true, nil
	}

	end := now
	return true, insertProbeIncidentTx(tx, state.ProbeTarget, state.Status, state.Message, state.Since, &end)
}

func openProbeIncidentTx(tx *sql.Tx, state ProbeState) error {
	_, err := tx.Exec(`
		INSERT OR IGNORE INTO probe_incidents (
			entity_kind, entity_id, entity_label, method, status, message, starts_at, ends_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
	`,
		state.EntityKind,
		state.EntityID,
		state.EntityLabel,
		state.Method,
		state.Status,
		state.Message,
		formatHistoryTime(state.Since),
	)
	if err != nil {
		return fmt.Errorf("open probe incident: %w", err)
	}
	return nil
}

func insertProbeIncidentTx(tx *sql.Tx, target ProbeTarget, status string, message string, startsAt time.Time, endsAt *time.Time) error {
	_, err := tx.Exec(`
		INSERT INTO probe_incidents (
			entity_kind, entity_id, entity_label, method, status, message, starts_at, ends_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		target.EntityKind,
		target.EntityID,
		target.EntityLabel,
		target.Method,
		status,
		message,
		formatHistoryTime(startsAt),
		nullableHistoryTimePtr(endsAt),
	)
	if err != nil {
		return fmt.Errorf("insert probe incident: %w", err)
	}
	return nil
}

func (hs *HistoryStore) ProbeIncidents(now time.Time, days int) []ProbeIncident {
	if hs == nil {
		return nil
	}

	hs.mu.Lock()
	defer hs.mu.Unlock()

	cutoff := formatHistoryTime(historyStartDay(now, days))
	rows, err := hs.db.Query(`
		SELECT entity_kind, entity_id, entity_label, method, status, message, starts_at, ends_at
		FROM probe_incidents
		WHERE ends_at IS NULL OR ends_at >= ? OR starts_at >= ?
		ORDER BY starts_at ASC, id ASC
	`, cutoff, cutoff)
	if err != nil {
		return nil
	}

	ret := make([]ProbeIncident, 0)
	openIncidents := make(map[string]struct{})
	for rows.Next() {
		incident, err := scanProbeIncident(rows)
		if err != nil {
			rows.Close()
			return nil
		}
		if incident.EndsAt == nil {
			openIncidents[probeIncidentKey(incident.ProbeTarget, incident.StartsAt)] = struct{}{}
		}
		ret = append(ret, incident)
	}
	if err := rows.Close(); err != nil {
		return nil
	}
	if err := rows.Err(); err != nil {
		return nil
	}

	stateRows, err := hs.db.Query(`
		SELECT entity_kind, entity_id, entity_label, method, status, message, since,
			last_seen, incident_promoted
		FROM probe_states
		WHERE status != ?
	`, historyProbeStateOperational)
	if err != nil {
		return ret
	}
	defer stateRows.Close()

	for stateRows.Next() {
		state, err := scanProbeState(stateRows)
		if err != nil {
			return ret
		}
		if isOperationalProbeState(state.Status) || now.Before(state.Since.Add(probeIncidentThreshold)) {
			continue
		}

		if _, ok := openIncidents[probeIncidentKey(state.ProbeTarget, state.Since)]; ok {
			continue
		}

		ret = append(ret, ProbeIncident{
			ProbeTarget: state.ProbeTarget,
			Status:      state.Status,
			Message:     state.Message,
			StartsAt:    state.Since,
		})
	}

	return ret
}

func (hs *HistoryStore) ProbeEventsFor(kind string, id string, now time.Time, days int) []ProbeEvent {
	if hs == nil {
		return nil
	}

	hs.mu.Lock()
	defer hs.mu.Unlock()

	rows, err := hs.db.Query(`
		SELECT entity_kind, entity_id, entity_label, method, status, message, changed_at
		FROM probe_events
		WHERE entity_kind = ? AND entity_id = ? AND changed_at >= ?
		ORDER BY changed_at DESC, id DESC
	`, kind, id, formatHistoryTime(historyStartDay(now, days)))
	if err != nil {
		return nil
	}
	defer rows.Close()

	ret := make([]ProbeEvent, 0)
	for rows.Next() {
		var changedAt string
		event := ProbeEvent{}
		if err := rows.Scan(
			&event.EntityKind,
			&event.EntityID,
			&event.EntityLabel,
			&event.Method,
			&event.Status,
			&event.Message,
			&changedAt,
		); err != nil {
			return nil
		}

		t, err := parseHistoryTime(changedAt)
		if err != nil {
			return nil
		}
		event.ChangedAt = t
		ret = append(ret, event)
	}

	return ret
}

func (hs *HistoryStore) ProbeEventsForTargets(targets []historyEntityInfo, now time.Time, days int) []ProbeEvent {
	if hs == nil || len(targets) == 0 {
		return nil
	}

	queryTargets := probeEventQueryTargets(targets, false)
	if len(queryTargets) == 0 {
		return nil
	}
	targetWhere, targetArgs := probeEventTargetWhere(queryTargets)

	hs.mu.Lock()
	defer hs.mu.Unlock()

	args := append([]any{formatHistoryTime(historyStartDay(now, days))}, targetArgs...)
	rows, err := hs.db.Query(`
		SELECT entity_kind, entity_id, entity_label, method, status, message, changed_at
		FROM probe_events
		WHERE changed_at >= ? AND (`+targetWhere+`)
		ORDER BY changed_at DESC, id DESC
	`, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	ret := make([]ProbeEvent, 0)
	for rows.Next() {
		event, err := scanProbeEvent(rows)
		if err != nil {
			return nil
		}
		ret = append(ret, event)
	}

	if err := rows.Err(); err != nil {
		return nil
	}

	return ret
}

func (hs *HistoryStore) ProbeLogFor(kind string, id string, now time.Time, days int, limit int, offset int) ProbeLogPage {
	if hs == nil || kind == "" || id == "" {
		return ProbeLogPage{}
	}
	limit, offset = normalizeProbeLogLimitOffset(limit, offset)
	if limit == 0 {
		return ProbeLogPage{}
	}

	hs.mu.Lock()
	defer hs.mu.Unlock()

	cutoff := formatHistoryTime(historyStartDay(now, days))
	total := hs.countProbeEventsLocked(
		`entity_kind = ? AND entity_id = ? AND changed_at >= ?`,
		kind,
		id,
		cutoff,
	)
	events := hs.queryProbeLogEventsLocked(`
		SELECT
			p.entity_kind, p.entity_id, p.entity_label, p.method, p.status,
			p.message, p.changed_at,
			(
				SELECT p2.changed_at
				FROM probe_events p2
				WHERE p2.entity_kind = p.entity_kind
					AND p2.entity_id = p.entity_id
					AND p2.method = p.method
					AND (
						p2.changed_at > p.changed_at
						OR (p2.changed_at = p.changed_at AND p2.id > p.id)
					)
				ORDER BY p2.changed_at ASC, p2.id ASC
				LIMIT 1
			) AS ends_at
		FROM probe_events p
		WHERE p.entity_kind = ? AND p.entity_id = ? AND p.changed_at >= ?
		ORDER BY p.changed_at DESC, p.id DESC
		LIMIT ? OFFSET ?
	`, kind, id, cutoff, limit, offset)

	return ProbeLogPage{Events: events, Total: total}
}

func (hs *HistoryStore) ProbeLogForTargets(targets []historyEntityInfo, now time.Time, days int, limit int, offset int) ProbeLogPage {
	if hs == nil || len(targets) == 0 {
		return ProbeLogPage{}
	}
	limit, offset = normalizeProbeLogLimitOffset(limit, offset)
	if limit == 0 {
		return ProbeLogPage{}
	}

	queryTargets := probeEventQueryTargets(targets, false)
	if len(queryTargets) == 0 {
		return ProbeLogPage{}
	}
	targetWhere, targetArgs := probeEventTargetWhere(queryTargets)

	hs.mu.Lock()
	defer hs.mu.Unlock()

	cutoff := formatHistoryTime(historyStartDay(now, days))
	countArgs := append([]any{cutoff}, targetArgs...)
	total := hs.countProbeEventsLocked(`changed_at >= ? AND (`+targetWhere+`)`, countArgs...)

	queryArgs := append([]any{cutoff}, targetArgs...)
	queryArgs = append(queryArgs, limit, offset)
	events := hs.queryProbeLogEventsLocked(`
		SELECT
			p.entity_kind, p.entity_id, p.entity_label, p.method, p.status,
			p.message, p.changed_at,
			(
				SELECT p2.changed_at
				FROM probe_events p2
				WHERE p2.entity_kind = p.entity_kind
					AND p2.entity_id = p.entity_id
					AND p2.method = p.method
					AND (
						p2.changed_at > p.changed_at
						OR (p2.changed_at = p.changed_at AND p2.id > p.id)
					)
				ORDER BY p2.changed_at ASC, p2.id ASC
				LIMIT 1
			) AS ends_at
		FROM probe_events p
		WHERE p.changed_at >= ? AND (`+targetWhere+`)
		ORDER BY p.changed_at DESC, p.entity_label ASC, p.method ASC, p.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)

	return ProbeLogPage{Events: events, Total: total}
}

func (hs *HistoryStore) ProbeEventsForAvailability(kind string, id string, method string, start time.Time, end time.Time) []ProbeEvent {
	if hs == nil || kind == "" || id == "" || method == "" || !start.Before(end) {
		return nil
	}

	hs.mu.Lock()
	defer hs.mu.Unlock()

	ret := make([]ProbeEvent, 0)
	row := hs.db.QueryRow(`
		SELECT entity_kind, entity_id, entity_label, method, status, message, changed_at
		FROM probe_events
		WHERE entity_kind = ? AND entity_id = ? AND method = ? AND changed_at < ?
		ORDER BY changed_at DESC, id DESC
		LIMIT 1
	`, kind, id, method, formatHistoryTime(start))
	if event, err := scanProbeEvent(row); err == nil {
		ret = append(ret, event)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil
	}

	rows, err := hs.db.Query(`
		SELECT entity_kind, entity_id, entity_label, method, status, message, changed_at
		FROM probe_events
		WHERE entity_kind = ? AND entity_id = ? AND method = ? AND changed_at >= ? AND changed_at <= ?
		ORDER BY changed_at ASC, id ASC
	`, kind, id, method, formatHistoryTime(start), formatHistoryTime(end))
	if err != nil {
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		event, err := scanProbeEvent(rows)
		if err != nil {
			return nil
		}
		ret = append(ret, event)
	}

	if err := rows.Err(); err != nil {
		return nil
	}

	return ret
}

func (hs *HistoryStore) ProbeEventsForAvailabilityTargets(targets []historyEntityInfo, start time.Time, end time.Time) map[string][]ProbeEvent {
	ret := make(map[string][]ProbeEvent, len(targets))
	if hs == nil || len(targets) == 0 || !start.Before(end) {
		return ret
	}

	queryTargets := probeEventQueryTargets(targets, true)
	if len(queryTargets) == 0 {
		return ret
	}
	targetWhere, targetArgs := probeEventTargetWhere(queryTargets)

	hs.mu.Lock()
	defer hs.mu.Unlock()

	args := append([]any{formatHistoryTime(end)}, targetArgs...)
	rows, err := hs.db.Query(`
		SELECT entity_kind, entity_id, entity_label, method, status, message, changed_at
		FROM probe_events
		WHERE changed_at <= ? AND (`+targetWhere+`)
		ORDER BY entity_kind ASC, entity_id ASC, method ASC, changed_at ASC, id ASC
	`, args...)
	if err != nil {
		return ret
	}
	defer rows.Close()

	previous := make(map[string]ProbeEvent, len(queryTargets))
	for rows.Next() {
		event, err := scanProbeEvent(rows)
		if err != nil {
			return ret
		}

		key := historyKey(event.EntityKind, event.EntityID)
		if event.ChangedAt.Before(start) {
			previous[key] = event
			continue
		}

		ret[key] = append(ret[key], event)
	}
	if err := rows.Err(); err != nil {
		return ret
	}

	for key, event := range previous {
		ret[key] = append([]ProbeEvent{event}, ret[key]...)
	}

	return ret
}

type probeEventQueryTarget struct {
	kind   string
	id     string
	method string
}

func probeEventQueryTargets(targets []historyEntityInfo, includeMethod bool) []probeEventQueryTarget {
	ret := make([]probeEventQueryTarget, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))

	for _, target := range targets {
		if target.Kind == "" || target.ID == "" {
			continue
		}

		method := ""
		if includeMethod {
			var ok bool
			method, ok = availabilityProbeMethod(target.Kind)
			if !ok {
				continue
			}
		}

		key := target.Kind + "\t" + target.ID + "\t" + method
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ret = append(ret, probeEventQueryTarget{
			kind:   target.Kind,
			id:     target.ID,
			method: method,
		})
	}

	return ret
}

func probeEventTargetWhere(targets []probeEventQueryTarget) (string, []any) {
	parts := make([]string, 0, len(targets))
	args := make([]any, 0, len(targets)*3)

	for _, target := range targets {
		if target.method == "" {
			parts = append(parts, `(entity_kind = ? AND entity_id = ?)`)
			args = append(args, target.kind, target.id)
		} else {
			parts = append(parts, `(entity_kind = ? AND entity_id = ? AND method = ?)`)
			args = append(args, target.kind, target.id, target.method)
		}
	}

	return strings.Join(parts, " OR "), args
}

func normalizeProbeLogLimitOffset(limit int, offset int) (int, int) {
	if limit < 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (hs *HistoryStore) countProbeEventsLocked(where string, args ...any) int {
	if where == "" {
		return 0
	}

	var ret int
	if err := hs.db.QueryRow(`SELECT COUNT(*) FROM probe_events WHERE `+where, args...).Scan(&ret); err != nil {
		return 0
	}
	return ret
}

func (hs *HistoryStore) queryProbeLogEventsLocked(query string, args ...any) []ProbeLogEvent {
	rows, err := hs.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	ret := make([]ProbeLogEvent, 0)
	for rows.Next() {
		event, err := scanProbeLogEvent(rows)
		if err != nil {
			return nil
		}
		ret = append(ret, event)
	}

	if err := rows.Err(); err != nil {
		return nil
	}
	return ret
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProbeEvent(row scanner) (ProbeEvent, error) {
	var changedAt string
	event := ProbeEvent{}
	if err := row.Scan(
		&event.EntityKind,
		&event.EntityID,
		&event.EntityLabel,
		&event.Method,
		&event.Status,
		&event.Message,
		&changedAt,
	); err != nil {
		return ProbeEvent{}, err
	}

	t, err := parseHistoryTime(changedAt)
	if err != nil {
		return ProbeEvent{}, err
	}
	event.ChangedAt = t
	return event, nil
}

func scanProbeLogEvent(row scanner) (ProbeLogEvent, error) {
	var changedAt string
	var endsAt sql.NullString
	event := ProbeLogEvent{}
	if err := row.Scan(
		&event.EntityKind,
		&event.EntityID,
		&event.EntityLabel,
		&event.Method,
		&event.Status,
		&event.Message,
		&changedAt,
		&endsAt,
	); err != nil {
		return ProbeLogEvent{}, err
	}

	t, err := parseHistoryTime(changedAt)
	if err != nil {
		return ProbeLogEvent{}, err
	}
	event.ChangedAt = t

	if endsAt.Valid {
		t, err = parseHistoryTime(endsAt.String)
		if err != nil {
			return ProbeLogEvent{}, err
		}
		event.EndsAt = t
	}

	return event, nil
}

func scanProbeState(row scanner) (ProbeState, error) {
	var since string
	var lastSeen string
	var promoted int
	state := ProbeState{}

	if err := row.Scan(
		&state.EntityKind,
		&state.EntityID,
		&state.EntityLabel,
		&state.Method,
		&state.Status,
		&state.Message,
		&since,
		&lastSeen,
		&promoted,
	); err != nil {
		return ProbeState{}, err
	}

	var err error
	state.Since, err = parseHistoryTime(since)
	if err != nil {
		return ProbeState{}, err
	}
	state.LastSeen, err = parseHistoryTime(lastSeen)
	if err != nil {
		return ProbeState{}, err
	}
	state.IncidentPromoted = promoted != 0

	return state, nil
}

func scanProbeIncident(row scanner) (ProbeIncident, error) {
	var startsAt string
	var endsAt sql.NullString
	incident := ProbeIncident{}

	if err := row.Scan(
		&incident.EntityKind,
		&incident.EntityID,
		&incident.EntityLabel,
		&incident.Method,
		&incident.Status,
		&incident.Message,
		&startsAt,
		&endsAt,
	); err != nil {
		return ProbeIncident{}, err
	}

	var err error
	incident.StartsAt, err = parseHistoryTime(startsAt)
	if err != nil {
		return ProbeIncident{}, err
	}
	if endsAt.Valid {
		end, err := parseHistoryTime(endsAt.String)
		if err != nil {
			return ProbeIncident{}, err
		}
		incident.EndsAt = &end
	}

	return incident, nil
}

func nullableHistoryTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return formatHistoryTime(t)
}

func nullableHistoryTimePtr(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return formatHistoryTime(*t)
}

func sqlPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

func formatHistoryTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseHistoryTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

func probeStateKey(target ProbeTarget) string {
	return target.EntityKind + "\t" + target.EntityID + "\t" + target.Method
}

func probeIncidentKey(target ProbeTarget, startsAt time.Time) string {
	return probeStateKey(target) + "\t" + formatHistoryTime(startsAt)
}

func isOperationalProbeState(status string) bool {
	return status == "" || status == historyProbeStateOperational
}

func historyKey(kind string, id string) string {
	return kind + "\t" + id
}

func historyWindowDays(days int) int {
	if days <= 0 {
		return historyDefaultDays
	}
	return days
}

func historyDaysForStatus(st *Status) int {
	if st == nil {
		return historyDefaultDays
	}
	return historyWindowDays(st.HistoryDays)
}

func historyStartDay(now time.Time, days int) time.Time {
	days = historyWindowDays(days)
	local := now.Local()
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location()).AddDate(0, 0, -(days - 1))
}

func outageEndsAt(report *OutageReport) time.Time {
	if report == nil {
		return time.Time{}
	}
	if !report.FinishedAt.IsZero() {
		return report.FinishedAt
	}
	if report.BeginsAt.IsZero() {
		return time.Time{}
	}
	end := report.BeginsAt.Add(report.Duration)
	if end.Before(report.BeginsAt) {
		return report.BeginsAt
	}
	return end
}
