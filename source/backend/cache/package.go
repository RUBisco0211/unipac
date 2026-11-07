package cache

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
)

//go:embed schema.sql
var schemaSQL string

func (m *Cache) createPackageCacheTable() error {
	if _, err := m.db.ExecContext(m.ctx, schemaSQL); err != nil {
		return err
	}
	return m.addDetailColumns()
}

// addDetailColumns gives a database written before the package page existed its
// two columns: SQLite has no "ADD COLUMN IF NOT EXISTS", so the table's own shape
// decides.
func (m *Cache) addDetailColumns() error {
	var names []string
	if err := m.db.SelectContext(m.ctx, &names,
		"SELECT name FROM pragma_table_info('installed_package_cache')"); err != nil {
		return err
	}
	have := make(map[string]bool, len(names))
	for _, name := range names {
		have[name] = true
	}
	for _, column := range []struct{ name, kind string }{
		{"detail", "TEXT"},
		{"detail_fetched_at", "INTEGER"},
	} {
		if have[column.name] {
			continue
		}
		if _, err := m.db.ExecContext(m.ctx,
			"ALTER TABLE installed_package_cache ADD COLUMN "+column.name+" "+column.kind); err != nil {
			return err
		}
	}
	return nil
}

func (m *Cache) List() ([]manager.Package, error) {
	result := make([]manager.Package, 0)
	err := m.db.SelectContext(m.ctx, &result, `SELECT name, fullname, version, latest_version, manager, installed, outdated, description
		FROM installed_package_cache ORDER BY name`)
	if err != nil {
		return nil, err
	}
	return result, nil
}

//func (m *Manager) InsertPackages(pkgs []pkg.Package) error {
//	err := m.DB.NamedExecContext(m.Ctx)
//}

// UpdateCache installs a scan's snapshot. A package that is still installed keeps
// its row and, when its version has not changed, the page read for it; one that is
// gone loses its row. An empty list clears the cache: callers decide whether an
// empty scan is trustworthy, so that a failed scan can keep the previous snapshot
// while a real "nothing is installed" clears it.
func (m *Cache) UpdateCache(pkgs []manager.Package) (err error) {
	tx, err := m.db.Beginx()
	if err != nil {
		slog.ErrorContext(m.ctx, "Failed to begin transaction for updating package cache", "error", err)
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			slog.ErrorContext(m.ctx, "Failed to roll back the package cache update", "error", rollbackErr)
		}
	}()

	// The scan's own keys decide what is gone, which is what lets the rest of a
	// row survive: a table of the scan, then the rows it does not name.
	if _, err = tx.Exec(`CREATE TEMP TABLE IF NOT EXISTS scan_keys (
		name TEXT NOT NULL, manager TEXT NOT NULL, PRIMARY KEY(name, manager))`); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM scan_keys"); err != nil {
		return err
	}
	if len(pkgs) > 0 {
		if _, err = tx.NamedExecContext(m.ctx,
			"INSERT OR IGNORE INTO scan_keys (name, manager) VALUES (:name, :manager)", pkgs); err != nil {
			return err
		}
	}

	// A package already cached is updated in place, and a version that changed
	// takes its page with it: the facts describe a version, not a name.
	query := `INSERT INTO installed_package_cache
			(name, fullname, version, latest_version, manager, installed, outdated, description)
			VALUES (:name, :fullname, :version, :latest_version, :manager, :installed, :outdated, :description)
			ON CONFLICT(name, manager) DO UPDATE SET
				fullname = excluded.fullname,
				latest_version = excluded.latest_version,
				installed = excluded.installed,
				outdated = excluded.outdated,
				description = excluded.description,
				detail = CASE WHEN installed_package_cache.version = excluded.version
					THEN installed_package_cache.detail ELSE NULL END,
				detail_fetched_at = CASE WHEN installed_package_cache.version = excluded.version
					THEN installed_package_cache.detail_fetched_at ELSE NULL END,
				version = excluded.version`
	if len(pkgs) > 0 {
		if _, err = tx.NamedExecContext(m.ctx, query, pkgs); err != nil {
			slog.ErrorContext(m.ctx, "Failed to write the package cache", "error", err)
			return err
		}
	}
	if _, err = tx.Exec(`DELETE FROM installed_package_cache WHERE NOT EXISTS (
		SELECT 1 FROM scan_keys WHERE scan_keys.name = installed_package_cache.name
			AND scan_keys.manager = installed_package_cache.manager)`); err != nil {
		return err
	}
	return tx.Commit()
}

// PackageDetail is the page kept for a package, and whether it is fresh enough to
// show without reading it again.
func (m *Cache) PackageDetail(managerID, name, version string) (manager.Detail, bool, error) {
	var row struct {
		Detail    *string `db:"detail"`
		FetchedAt *int64  `db:"detail_fetched_at"`
	}
	err := m.db.GetContext(m.ctx, &row, `SELECT detail, detail_fetched_at FROM installed_package_cache
		WHERE name = ? AND manager = ? AND version = ?`, name, managerID, version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return manager.Detail{}, false, nil
		}
		return manager.Detail{}, false, err
	}
	if row.Detail == nil || row.FetchedAt == nil {
		return manager.Detail{}, false, nil
	}
	if time.Since(time.Unix(*row.FetchedAt, 0)) > detailTTL {
		return manager.Detail{}, false, nil
	}
	var detail manager.Detail
	if err := json.Unmarshal([]byte(*row.Detail), &detail); err != nil {
		return manager.Detail{}, false, nil
	}
	return detail, true, nil
}

// SavePackageDetail keeps a package's page, which survives the scans that follow
// while the version is the same.
func (m *Cache) SavePackageDetail(managerID, name, version string, detail manager.Detail) error {
	data, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = m.db.ExecContext(m.ctx, `UPDATE installed_package_cache SET detail = ?, detail_fetched_at = ?
		WHERE name = ? AND manager = ? AND version = ?`,
		string(data), time.Now().Unix(), name, managerID, version)
	return err
}

// detailTTL is how long a package's page is shown without reading it again: a
// project's page changes rarely, and a version that does is a different entry.
const detailTTL = 10 * time.Minute
