package tasks

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/util"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// sqliteDriver is the pure-Go driver, so the app builds and cross-compiles with
// CGO_ENABLED=0.
const sqliteDriver = "sqlite"

func sqliteDSN(path string) string {
	return fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", path)
}

//go:embed schema.sql
var schemaSQL string

// store keeps task metadata and console output. Output lives in its own table,
// one row per line, so appending a line never rewrites a whole log.
type store struct {
	ctx context.Context
	db  *sqlx.DB
}

// taskRow is one row of the task table.
type taskRow struct {
	ID          int64  `db:"id"`
	Kind        Kind   `db:"kind"`
	State       State  `db:"state"`
	ManagerID   string `db:"manager_id"`
	ManagerName string `db:"manager_name"`
	Package     string `db:"package"`
	Version     string `db:"version"`
	NewVersion  string `db:"new_version"`
	Command     string `db:"command"`
	ExecPath    string `db:"exec_path"`
	Message     string `db:"message"`
	StartedAt   int64  `db:"started_at"`
	FinishedAt  int64  `db:"finished_at"`
	Truncated   int    `db:"truncated"`
}

func openStore(ctx context.Context) (*store, error) {
	path, err := util.SafeJoin(util.AppRoot, "cache/tasks.db")
	if err != nil {
		return nil, err
	}
	db, err := sqlx.Open(sqliteDriver, sqliteDSN(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &store{ctx: ctx, db: db}, nil
}

func (s *store) close() {
	if s.db != nil {
		_ = s.db.Close()
	}
}

func (s *store) migrate() error {
	if _, err := s.db.ExecContext(s.ctx, schemaSQL); err != nil {
		return err
	}
	// The task table predates the version pair and the truncation flag.
	for _, column := range []struct{ name, ddl string }{
		{"new_version", "new_version TEXT NOT NULL DEFAULT ''"},
		{"truncated", "truncated INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := s.ensureColumn("package_tasks", column.name, column.ddl); err != nil {
			return err
		}
	}
	return nil
}

// ensureColumn adds column to table when an older database lacks it.
func (s *store) ensureColumn(table, column, ddl string) error {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?", table)
	if err := s.db.GetContext(s.ctx, &count, query, column); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := s.db.ExecContext(s.ctx, "ALTER TABLE "+table+" ADD COLUMN "+ddl)
	return err
}

func (s *store) markInterrupted(message string, at time.Time) error {
	_, err := s.db.ExecContext(s.ctx,
		`UPDATE package_tasks SET state = ?, message = ?, finished_at = ?
		 WHERE state IN (?, ?)`,
		StateCancelled, message, at.UnixNano(), StateQueued, StateRunning)
	return err
}

// prune keeps the newest keep tasks, plus any older unfinished row, and drops
// the console lines of the tasks it removed.
func (s *store) prune(keep int) error {
	if _, err := s.db.ExecContext(s.ctx,
		`DELETE FROM package_tasks
		 WHERE id NOT IN (SELECT id FROM package_tasks ORDER BY id DESC LIMIT ?)
		   AND state NOT IN (?, ?)`,
		keep, StateQueued, StateRunning); err != nil {
		return err
	}
	_, err := s.db.ExecContext(s.ctx,
		`DELETE FROM package_task_output
		 WHERE task_id NOT IN (SELECT id FROM package_tasks)`)
	return err
}

func (s *store) load(limit int) ([]taskRow, error) {
	rows := make([]taskRow, 0)
	// The columns are named, not taken with *: an older database still has the
	// is_gui column that is no longer read.
	err := s.db.SelectContext(s.ctx, &rows,
		`SELECT id, kind, state, manager_id, manager_name, package, version,
		        new_version, command, exec_path, message, started_at, finished_at, truncated
		   FROM package_tasks ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *store) insert(row taskRow) (int64, error) {
	res, err := s.db.ExecContext(s.ctx,
		`INSERT INTO package_tasks
		 (kind, state, manager_id, manager_name, package, version, new_version,
		  command, exec_path, message, started_at, finished_at, truncated)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.Kind, row.State, row.ManagerID, row.ManagerName, row.Package, row.Version,
		row.NewVersion, row.Command, row.ExecPath, row.Message, row.StartedAt,
		row.FinishedAt, row.Truncated)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// markRunning moves a queued row to running. The state predicate makes a
// cancel that wins the race with Start win the write as well.
func (s *store) markRunning(id int64) error {
	_, err := s.db.ExecContext(s.ctx,
		`UPDATE package_tasks SET state = ? WHERE id = ? AND state = ?`,
		StateRunning, id, StateQueued)
	return err
}

func (s *store) setCommand(id int64, command, execPath string) error {
	_, err := s.db.ExecContext(s.ctx,
		`UPDATE package_tasks SET command = ?, exec_path = ? WHERE id = ?`, command, execPath, id)
	return err
}

func (s *store) finish(id int64, state State, message string, at time.Time) error {
	_, err := s.db.ExecContext(s.ctx,
		`UPDATE package_tasks SET state = ?, message = ?, finished_at = ? WHERE id = ?`,
		state, message, at.UnixNano(), id)
	return err
}

func (s *store) setTruncated(id int64) error {
	_, err := s.db.ExecContext(s.ctx,
		`UPDATE package_tasks SET truncated = 1 WHERE id = ?`, id)
	return err
}

func (s *store) appendOutput(id int64, seq int, line string) error {
	if _, err := s.db.ExecContext(s.ctx,
		`INSERT OR REPLACE INTO package_task_output (task_id, seq, line) VALUES (?, ?, ?)`,
		id, seq, line); err != nil {
		return err
	}
	if seq < maxOutput {
		return nil
	}
	// Keep the newest window in the store too, matching the in-memory buffer.
	_, err := s.db.ExecContext(s.ctx,
		`DELETE FROM package_task_output WHERE task_id = ? AND seq = ?`, id, seq-maxOutput)
	return err
}

func (s *store) loadOutput(id int64) ([]Line, error) {
	rows := make([]string, 0)
	if err := s.db.SelectContext(s.ctx, &rows,
		`SELECT line FROM package_task_output WHERE task_id = ? ORDER BY seq`, id); err != nil {
		return nil, err
	}
	lines := make([]Line, len(rows))
	for i, row := range rows {
		lines[i] = Line{Text: row}
	}
	return lines, nil
}

// taskFromRow rebuilds a task loaded from the store. Its console output comes
// from the output table on demand.
func taskFromRow(row taskRow, m *Manager) *Task {
	return &Task{
		id:   row.ID,
		kind: row.Kind,
		target: Target{
			ManagerID:   row.ManagerID,
			ManagerName: row.ManagerName,
			Name:        row.Package,
			Version:     row.Version,
			NewVersion:  row.NewVersion,
		},
		state:      row.State,
		started:    time.Unix(0, row.StartedAt),
		finished:   time.Unix(0, row.FinishedAt),
		message:    row.Message,
		commands:   splitCommands(row.Command),
		execPath:   row.ExecPath,
		truncated:  row.Truncated != 0,
		outputKept: false,
		manager:    m,
	}
}

func splitCommands(command string) []string {
	if command == "" {
		return nil
	}
	return strings.Split(command, "\n")
}
