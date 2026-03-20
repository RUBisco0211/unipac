use crate::adapters::PackageAdapter;
use crate::models::{ActionResult, ManagerCapabilities, Package};
use async_trait::async_trait;
use std::collections::HashMap;
use tokio::process::Command;

/// Cargo 适配器
pub struct CargoAdapter;

impl CargoAdapter {
    pub fn new() -> Self {
        Self
    }
}

#[async_trait]
impl PackageAdapter for CargoAdapter {
    fn id(&self) -> &str {
        "cargo"
    }

    fn name(&self) -> &str {
        "Cargo"
    }

    fn capabilities(&self) -> ManagerCapabilities {
        ManagerCapabilities::default()
    }

    async fn preflight(&self) -> Result<(), String> {
        // 检查 cargo 命令是否存在
        let result = Command::new("which")
            .arg("cargo")
            .output()
            .await
            .map_err(|e| format!("Failed to check for cargo: {}", e))?;

        if !result.status.success() {
            return Err(
                "cargo is not installed or not in PATH. Please install Rust from https://rustup.rs/"
                    .to_string(),
            );
        }

        // 检查 cargo 是否可用
        let version_result = Command::new("cargo")
            .arg("--version")
            .output()
            .await;

        match version_result {
            Ok(output) if output.status.success() => Ok(()),
            Ok(_) => Err("cargo command exists but is not functional".to_string()),
            Err(e) => Err(format!("Failed to run cargo: {}", e)),
        }
    }

    async fn list_packages(&self) -> Result<Vec<Package>, String> {
        // TODO: 实现 Cargo 列表获取
        Ok(Vec::new())
    }

    async fn install_packages(
        &self,
        _names: &[&str],
        _options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        // TODO: 实现 Cargo 安装
        Ok(ActionResult::error("Not implemented"))
    }

    async fn uninstall_packages(
        &self,
        _names: &[&str],
        _options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        // TODO: 实现 Cargo 卸载
        Ok(ActionResult::error("Not implemented"))
    }

    async fn upgrade_packages(
        &self,
        _names: &[&str],
        _options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        // TODO: 实现 Cargo 升级
        Ok(ActionResult::error("Not implemented"))
    }

    async fn search_packages(&self, _keyword: &str) -> Result<Vec<Package>, String> {
        // TODO: 实现 Cargo 搜索
        Ok(Vec::new())
    }
}