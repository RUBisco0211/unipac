use crate::adapters::PackageAdapter;
use crate::models::{ActionResult, ManagerCapabilities, Package};
use async_trait::async_trait;
use std::collections::HashMap;
use tokio::process::Command;

/// npm 适配器
pub struct NpmAdapter;

impl NpmAdapter {
    pub fn new() -> Self {
        Self
    }
}

#[async_trait]
impl PackageAdapter for NpmAdapter {
    fn id(&self) -> &str {
        "npm"
    }

    fn name(&self) -> &str {
        "npm (global)"
    }

    fn capabilities(&self) -> ManagerCapabilities {
        ManagerCapabilities::default()
    }

    async fn preflight(&self) -> Result<(), String> {
        // 检查 npm 命令是否存在
        let result = Command::new("which")
            .arg("npm")
            .output()
            .await
            .map_err(|e| format!("Failed to check for npm: {}", e))?;

        if !result.status.success() {
            return Err(
                "npm is not installed or not in PATH. Please install Node.js from https://nodejs.org/"
                    .to_string(),
            );
        }

        // 检查 npm 是否可用
        let version_result = Command::new("npm")
            .arg("--version")
            .output()
            .await;

        match version_result {
            Ok(output) if output.status.success() => Ok(()),
            Ok(_) => Err("npm command exists but is not functional".to_string()),
            Err(e) => Err(format!("Failed to run npm: {}", e)),
        }
    }

    async fn list_packages(&self) -> Result<Vec<Package>, String> {
        // TODO: 实现 npm 列表获取
        Ok(Vec::new())
    }

    async fn install_packages(
        &self,
        _names: &[&str],
        _options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        // TODO: 实现 npm 安装
        Ok(ActionResult::error("Not implemented"))
    }

    async fn uninstall_packages(
        &self,
        _names: &[&str],
        _options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        // TODO: 实现 npm 卸载
        Ok(ActionResult::error("Not implemented"))
    }

    async fn upgrade_packages(
        &self,
        _names: &[&str],
        _options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        // TODO: 实现 npm 升级
        Ok(ActionResult::error("Not implemented"))
    }

    async fn search_packages(&self, _keyword: &str) -> Result<Vec<Package>, String> {
        // TODO: 实现 npm 搜索
        Ok(Vec::new())
    }
}