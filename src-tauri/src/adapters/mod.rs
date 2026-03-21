use crate::models::{ActionResult, ManagerCapabilities, Package};
use std::collections::HashMap;
use tokio::process::Command;

/// 包管理器适配器 Trait
#[async_trait::async_trait]
pub trait PackageAdapter: Send + Sync {
    /// 获取适配器 ID
    fn id(&self) -> &str;

    /// 获取适配器名称
    fn name(&self) -> &str;

    /// 获取适配器能力
    fn capabilities(&self) -> ManagerCapabilities;

    /// 生命周期：预检
    /// 检查命令是否存在、依赖是否满足
    async fn preflight(&self) -> Result<(), String>;

    /// 生命周期：设置
    /// 安装缺失依赖（可选）
    #[allow(dead_code)]
    async fn setup(&self) -> Result<(), String> {
        Ok(())
    }

    /// 列出已安装的包
    async fn list_packages(&self) -> Result<Vec<Package>, String>;

    /// 安装包（支持多个）
    async fn install_packages(
        &self,
        names: &[&str],
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String>;

    /// 卸载包（支持多个）
    async fn uninstall_packages(
        &self,
        names: &[&str],
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String>;

    /// 升级包（支持多个）
    async fn upgrade_packages(
        &self,
        names: &[&str],
        options: Option<&HashMap<String, String>>,
    ) -> Result<ActionResult, String>;

    /// 搜索包
    async fn search_packages(&self, _keyword: &str) -> Result<Vec<Package>, String> {
        Ok(Vec::new())
    }
}

pub mod brew;
pub mod npm;
pub mod pip;
pub mod cargo;

pub(crate) async fn run_command(binary: &str, args: &[&str], label: &str) -> Result<String, String> {
    let output = Command::new(binary)
        .args(args)
        .output()
        .await
        .map_err(|e| format!("Failed to execute {}: {}", label, e))?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(format!("{} command failed: {}", label, stderr.trim()));
    }

    Ok(String::from_utf8_lossy(&output.stdout).to_string())
}

pub(crate) async fn ensure_command_in_path(
    binary: &str,
    missing_message: impl Into<String>,
) -> Result<(), String> {
    let result = Command::new("which")
        .arg(binary)
        .output()
        .await
        .map_err(|e| format!("Failed to check for {}: {}", binary, e))?;

    if result.status.success() {
        Ok(())
    } else {
        Err(missing_message.into())
    }
}

pub(crate) async fn ensure_command_healthy(
    binary: &str,
    version_args: &[&str],
    label: &str,
) -> Result<(), String> {
    let result = Command::new(binary).args(version_args).output().await;

    match result {
        Ok(output) if output.status.success() => Ok(()),
        Ok(_) => Err(format!("{} command exists but is not functional", label)),
        Err(error) => Err(format!("Failed to run {}: {}", label, error)),
    }
}
