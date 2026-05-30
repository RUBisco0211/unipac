package cache

import (
	"log/slog"
	"unipac-wails/internal/modules/pkg"
)

func (m *Manager) ListPackages() ([]pkg.Package, error) {
	result := make([]pkg.Package, 0)
	err := m.DB.SelectContext(m.Ctx, &result, "SELECT * FROM installed_package_cache ORDER BY name")
	if err != nil {
		return nil, err
	}
	return result, nil
}

//func (m *Manager) InsertPackages(pkgs []pkg.Package) error {
//	err := m.DB.NamedExecContext(m.Ctx)
//}

func (m *Manager) UpdateCache(pkgs []pkg.Package) (err error) {
	if len(pkgs) == 0 {
		return nil
	}
	tx, err := m.DB.Beginx()
	if err != nil {
		slog.ErrorContext(m.Ctx, "Failed to begin transaction for updating package cache", "error", err)
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		err := tx.Rollback()
		if err != nil {
			return
		}
	}()

	_, err = tx.Exec("DELETE FROM installed_package_cache")
	if err != nil {
		slog.ErrorContext(m.Ctx, "Failed to delete from table installed_package_cache", "error", err)
		return err
	}
	query := `INSERT INTO installed_package_cache 
    		(name, fullname, version, latest_version, manager, installed, outdated, is_gui, description) 
			VALUES (:name, :fullname, :version, :latest_version, :manager, :installed, :outdated, :is_gui, :description)`
	_, err = tx.NamedExecContext(m.Ctx, query, pkgs)
	if err != nil {
		slog.ErrorContext(m.Ctx, "Failed to insert into table installed_package_cache", "error", err)
		return err
	}
	return tx.Commit()
}
