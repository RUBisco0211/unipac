CREATE TABLE IF NOT EXISTS installed_package_cache (
    name TEXT NOT NULL,
    fullname TEXT,
    version TEXT NOT NULL,
    latest_version TEXT,
    manager TEXT NOT NULL,
    installed INTEGER NOT NULL DEFAULT 1,
    outdated INTEGER NOT NULL DEFAULT 0,
    description TEXT,
    -- The package's own page, as JSON, and when it was read: kept across scans
    -- while the installed version is the same, cleared when it changes.
    detail TEXT,
    detail_fetched_at INTEGER,
    PRIMARY KEY(name, manager)
)
