mod adapters;
mod cache;
mod logging;
mod models;
mod registry;

use cache::PackageCache;
use models::{ActionResult, ManagerInfo, Package, PackageTarget};
use registry::ManagerRegistry;
use std::collections::HashMap;
use std::sync::Arc;
use tauri::Manager;

/// 应用状态
#[derive(Clone)]
pub struct AppState {
    pub registry: Arc<ManagerRegistry>,
    pub cache: Arc<PackageCache>,
}

/// 获取所有包管理器信息
#[tauri::command]
async fn list_managers(state: tauri::State<'_, AppState>) -> Result<Vec<ManagerInfo>, String> {
    state.registry.list_managers().await
}

/// 列出所有已安装包（直接从缓存读取，快速加载）
#[tauri::command]
async fn load_cached_packages(
    state: tauri::State<'_, AppState>,
) -> Result<Vec<Package>, String> {
    state.cache.load_packages().await
}

/// 列出所有已安装包（扫描并更新缓存后返回）
#[tauri::command]
async fn reload_packages(
    state: tauri::State<'_, AppState>,
) -> Result<Vec<Package>, String> {
    // 扫描并获取最新数据
    let packages = state.registry.list_installed_packages().await?;

    // 更新缓存
    state.cache.update_cache(&packages).await?;

    Ok(packages)
}

/// 列出所有已安装包（保留原接口用于兼容）
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

#[tauri::command]
async fn batch_uninstall_packages(
    state: tauri::State<'_, AppState>,
    packages: Vec<PackageTarget>,
    options: Option<HashMap<String, String>>,
) -> Result<ActionResult, String> {
    state
        .registry
        .batch_uninstall_packages(&packages, options.as_ref())
        .await
}

#[tauri::command]
async fn batch_upgrade_packages(
    state: tauri::State<'_, AppState>,
    packages: Vec<PackageTarget>,
    options: Option<HashMap<String, String>>,
) -> Result<ActionResult, String> {
    state
        .registry
        .batch_upgrade_packages(&packages, options.as_ref())
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

            // 初始化缓存
            let cache = PackageCache::new()
                .map_err(|e| format!("Failed to initialize package cache: {}", e))?;
            let cache = Arc::new(cache);

            let app_state = AppState { registry, cache };

            app.manage(app_state);

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            list_managers,
            load_cached_packages,
            reload_packages,
            list_installed_packages,
            install_package,
            uninstall_package,
            upgrade_package,
            batch_uninstall_packages,
            batch_upgrade_packages,
            search_packages,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
