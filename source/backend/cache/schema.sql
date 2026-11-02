CREATE TABLE IF NOT EXISTS installed_package_cache (
    name TEXT NOT NULL,
    fullname TEXT,
    version TEXT NOT NULL,
    latest_version TEXT,
    manager TEXT NOT NULL,
    installed INTEGER NOT NULL DEFAULT 1,
    outdated INTEGER NOT NULL DEFAULT 0,
    is_gui INTEGER NOT NULL DEFAULT 0,
    description TEXT,
    PRIMARY KEY(name, manager)
)