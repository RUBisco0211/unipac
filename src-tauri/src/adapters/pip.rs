use crate::adapters::PackageAdapter;
use crate::models::{ActionResult, ManagerCapabilities, Package};
use async_trait::async_trait;
use std::collections::HashMap;
use tokio::process::Command;

/// pip 适配器
pub struct PipAdapter;

impl PipAdapter {
    pub fn new() -> Self {
        Self
    }
}

#[async_trait]
impl PackageAdapter for PipAdapter {
    fn id(&self) -> &str {
        "pip"
    }

    fn name(&self) -> &str {
        "pip (global)"
    }

    fn capabilities(&self) -> ManagerCapabilities {
        ManagerCapabilities::default()
    }

    async fn preflight(&self) -> Result<(), String> {
        // 检查 pip3 命令是否存在
        let pip3_result = Command::new("which")
            .arg("pip3")
            .output()
            .await;

        let pip_result = Command::new("which")
            .arg("pip")
            .output()
            .await;

        let pip_cmd = if pip3_result
            .as_ref()
            .map(|o| o.status.success())
            .unwrap_or(false)
        {
            Some("pip3")
        } else if pip_result
            .as_ref()
            .map(|o| o.status.success())
            .unwrap_or(false)
        {
            Some("pip")
        } else {
            None
        };

        if pip_cmd.is_none() {
            return Err(
                "pip is not installed or not in PATH. Please install Python from https://python.org/"
                    .to_string(),
            );
        }

        // 检查 pip 是否可用
        let cmd = pip_cmd.unwrap();
        let version_result = Command::new(cmd)
            .arg("--version")
            .output()
            .await;

        match version_result {
            Ok(output) if output.status.success() => Ok(()),
            Ok(_) => Err(format!("{} command exists but is not functional", cmd)),
            Err(e) => Err(format!("Failed to run {}: {}", cmd, e)),
        }
    }

    async fn list_packages(&self) -> Result<Vec<Package>, String> {
        // TODO: 实现 pip 列表获取
        Ok(Vec::new())
    }

    async fn install_packages(
        &self,
        _names: &[&str],
        _options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        // TODO: 实现 pip 安装
        Ok(ActionResult::error("Not implemented"))
    }

    async fn uninstall_packages(
        &self,
        _names: &[&str],
        _options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        // TODO: 实现 pip 卸载
        Ok(ActionResult::error("Not implemented"))
    }

    async fn upgrade_packages(
        &self,
        _names: &[&str],
        _options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String> {
        // TODO: 实现 pip 升级
        Ok(ActionResult::error("Not implemented"))
    }

    async fn search_packages(&self, _keyword: &str) -> Result<Vec<Package>, String> {
        // TODO: 实现 pip 搜索
        Ok(Vec::new())
    }
}