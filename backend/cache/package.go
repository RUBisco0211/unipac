package cache

import (
	_ "embed"
	"log/slog"
	"unipac-wails/backend/core/manager"
)

//go:embed schema.sql
var schemaSQL string

func (m *Cache) createPackageCacheTable() error {
	if _, err := m.db.ExecContext(m.ctx, schemaSQL); err != nil {
		return err
	}
	return nil
}

func (m *Cache) List() ([]manager.Package, error) {
	result := make([]manager.Package, 0)
	err := m.db.SelectContext(m.ctx, &result, "SELECT * FROM installed_package_cache ORDER BY name")
	if err != nil {
		return nil, err
	}
	return result, nil
}

//func (m *Manager) InsertPackages(pkgs []pkg.Package) error {
//	err := m.DB.NamedExecContext(m.Ctx)
//}

func (m *Cache) UpdateCache(pkgs []manager.Package) (err error) {
	if len(pkgs) == 0 {
		return nil
	}
	// begin transaction
	tx, err := m.db.Beginx()
	if err != nil {
		slog.ErrorContext(m.ctx, "Failed to begin transaction for updating package cache", "error", err)
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		// rollback if error
		err := tx.Rollback()
		if err != nil {
			return
		}
	}()

	// delete all data from cache
	_, err = tx.Exec("DELETE FROM installed_package_cache")
	if err != nil {
		slog.ErrorContext(m.ctx, "Failed to delete from table installed_package_cache", "error", err)
		return err
	}
	// full update all scanned packages
	query := `INSERT INTO installed_package_cache 
    		(name, fullname, version, latest_version, manager, installed, outdated, is_gui, description) 
			VALUES (:name, :fullname, :version, :latest_version, :manager, :installed, :outdated, :is_gui, :description)`
	_, err = tx.NamedExecContext(m.ctx, query, pkgs)
	if err != nil {
		slog.ErrorContext(m.ctx, "Failed to insert into table installed_package_cache", "error", err)
		return err
	}
	return tx.Commit()
}
