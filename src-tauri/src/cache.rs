use crate::models::{ManagerType, Package};
use rusqlite::{params, Connection};
use std::path::PathBuf;
use tokio::sync::Mutex;

/// 包缓存管理器
pub struct PackageCache {
    conn: Mutex<Connection>,
}

impl PackageCache {
    /// 创建或打开缓存数据库
    pub fn new() -> Result<Self, String> {
        let db_path = Self::get_db_path()?;
        let conn = Connection::open(&db_path)
            .map_err(|e| format!("Failed to open cache database: {}", e))?;

        // 初始化表结构
        conn.execute(
            "CREATE TABLE IF NOT EXISTS packages (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                name TEXT NOT NULL,
                fullname TEXT,
                version TEXT NOT NULL,
                latest_version TEXT NOT NULL,
                manager TEXT NOT NULL,
                installed INTEGER NOT NULL DEFAULT 1,
                outdated INTEGER NOT NULL DEFAULT 0,
                is_gui INTEGER NOT NULL DEFAULT 0,
                description TEXT,
                UNIQUE(name, manager) ON CONFLICT REPLACE
            )",
            [],
        )
        .map_err(|e| format!("Failed to create packages table: {}", e))?;

        Ok(Self {
            conn: Mutex::new(conn),
        })
    }

    /// 获取数据库文件路径
    fn get_db_path() -> Result<PathBuf, String> {
        let data_dir = dirs::data_local_dir()
            .ok_or_else(|| "Failed to get local data directory".to_string())?;
        Ok(data_dir.join("unipac").join("packages.db"))
    }

    /// 从缓存加载所有包（按 name 排序）
    pub async fn load_packages(&self) -> Result<Vec<Package>, String> {
        let conn = self.conn.lock().await;
        let mut stmt = conn
            .prepare(
                "SELECT name, fullname, version, latest_version, manager, installed, outdated, is_gui, description
                 FROM packages
                 ORDER BY name ASC",
            )
            .map_err(|e| format!("Failed to prepare statement: {}", e))?;

        let packages = stmt
            .query_map([], |row| {
                let manager_str: String = row.get(4)?;
                let manager = match manager_str.as_str() {
                    "brew" => ManagerType::Brew,
                    "npm" => ManagerType::Npm,
                    "pip" => ManagerType::Pip,
                    "cargo" => ManagerType::Cargo,
                    _ => ManagerType::Brew, // 默认值
                };

                Ok(Package {
                    name: row.get(0)?,
                    fullname: row.get(1)?,
                    version: row.get(2)?,
                    latest_version: row.get(3)?,
                    manager,
                    installed: row.get(5)?,
                    outdated: row.get(6)?,
                    is_gui: row.get(7)?,
                    description: row.get(8)?,
                })
            })
            .map_err(|e| format!("Failed to query packages: {}", e))?
            .collect::<Result<Vec<_>, _>>()
            .map_err(|e| format!("Failed to collect packages: {}", e))?;

        Ok(packages)
    }

    /// 增量更新缓存（只更新新增或变化的包）
    pub async fn update_packages(&self, packages: &[Package]) -> Result<(), String> {
        let conn = self.conn.lock().await;

        // 开启事务
        let tx = conn
            .unchecked_transaction()
            .map_err(|e| format!("Failed to start transaction: {}", e))?;

        // 使用 INSERT OR REPLACE 来更新包
        {
            let mut stmt = tx
                .prepare(
                    "INSERT INTO packages (name, fullname, version, latest_version, manager, installed, outdated, is_gui, description)
                     VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)
                     ON CONFLICT(name, manager) DO UPDATE SET
                        fullname = excluded.fullname,
                        version = excluded.version,
                        latest_version = excluded.latest_version,
                        installed = excluded.installed,
                        outdated = excluded.outdated,
                        is_gui = excluded.is_gui,
                        description = excluded.description",
                )
                .map_err(|e| format!("Failed to prepare insert statement: {}", e))?;

            for pkg in packages {
                stmt.execute(params![
                    pkg.name,
                    pkg.fullname,
                    pkg.version,
                    pkg.latest_version,
                    pkg.manager.as_str(),
                    pkg.installed as i32,
                    pkg.outdated as i32,
                    pkg.is_gui as i32,
                    pkg.description,
                ])
                .map_err(|e| format!("Failed to upsert package '{}': {}", pkg.name, e))?;
            }
        }

        tx.commit()
            .map_err(|e| format!("Failed to commit transaction: {}", e))?;

        Ok(())
    }

    /// 检查缓存是否存在数据
    pub async fn is_empty(&self) -> Result<bool, String> {
        let conn = self.conn.lock().await;
        let count: i64 = conn
            .query_row("SELECT COUNT(*) FROM packages", [], |row| row.get(0))
            .map_err(|e| format!("Failed to query count: {}", e))?;
        Ok(count == 0)
    }
}