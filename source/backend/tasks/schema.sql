CREATE TABLE IF NOT EXISTS package_tasks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    kind         TEXT    NOT NULL,
    state        TEXT    NOT NULL,
    manager_id   TEXT    NOT NULL,
    manager_name TEXT    NOT NULL,
    package      TEXT    NOT NULL,
    version      TEXT    NOT NULL DEFAULT '',
    new_version  TEXT    NOT NULL DEFAULT '',
    is_gui       INTEGER NOT NULL DEFAULT 0,
    command      TEXT    NOT NULL DEFAULT '',
    exec_path    TEXT    NOT NULL DEFAULT '',
    message      TEXT    NOT NULL DEFAULT '',
    started_at   INTEGER NOT NULL,
    finished_at  INTEGER NOT NULL DEFAULT 0,
    truncated    INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS package_tasks_started ON package_tasks (id DESC);

-- One row per console line: appending a line never rewrites the whole log.
CREATE TABLE IF NOT EXISTS package_task_output (
    task_id INTEGER NOT NULL,
    seq     INTEGER NOT NULL,
    line    TEXT    NOT NULL,
    PRIMARY KEY (task_id, seq)
);
