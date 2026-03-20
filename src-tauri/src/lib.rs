mod adapters;
mod logging;
mod models;
mod registry;

use models::{ActionResult, ManagerInfo, Package};
use registry::ManagerRegistry;
use std::collections::HashMap;
use std::sync::Arc;
use tauri::Manager;

/// 应用状态
#[derive(Clone)]
pub struct AppState {
    pub registry: Arc<ManagerRegistry>,
}

/// 获取所有包管理器信息
#[tauri::command]
async fn list_managers(state: tauri::State<'_, AppState>) -> Result<Vec<ManagerInfo>, String> {
    state.registry.list_managers().await
}

/// 列出所有已安装包
#[tauri::command]
async fn list_installed_packages(
    state: tauri::State<'_, AppState>,
) -> Result<Vec<Package>, String> {
    state.registry.list_installed_packages().await
}

/// 安装包
#[tauri::command]
async fn install_package(
    state: tauri::State<'_, AppState>,
    manager: String,
    name: String,
    options: Option<HashMap<String, String>>,
) -> Result<ActionResult, String> {
    state
        .registry
        .install_package(&manager, &name, options.as_ref())
        .await
}

/// 卸载包
#[tauri::command]
async fn uninstall_package(
    state: tauri::State<'_, AppState>,
    manager: String,
    name: String,
    options: Option<HashMap<String, String>>,
) -> Result<ActionResult, String> {
    state
        .registry
        .uninstall_package(&manager, &name, options.as_ref())
        .await
}

/// 升级包
#[tauri::command]
async fn upgrade_package(
    state: tauri::State<'_, AppState>,
    manager: String,
    name: String,
    options: Option<HashMap<String, String>>,
) -> Result<ActionResult, String> {
    state
        .registry
        .upgrade_package(&manager, &name, options.as_ref())
        .await
}

/// 搜索包
#[tauri::command]
async fn search_packages(
    state: tauri::State<'_, AppState>,
    keyword: String,
) -> Result<Vec<Package>, String> {
    state.registry.search_packages(&keyword).await
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    // 初始化日志系统
    let log_manager = logging::LogManager::new()
        .expect("Failed to create LogManager");
    log_manager.init().expect("Failed to initialize logging");

    tracing::info!("Starting unipac...");

    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .setup(|app| {
            // 在 Tokio runtime 中阻塞初始化注册表
            let registry = tauri::async_runtime::block_on(ManagerRegistry::new());
            let registry = Arc::new(registry);
            let app_state = AppState { registry };

            app.manage(app_state);

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            list_managers,
            list_installed_packages,
            install_package,
            uninstall_package,
            upgrade_package,
            search_packages,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}