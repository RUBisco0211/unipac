use crate::models::{ActionResult, ManagerCapabilities, Package};
use std::collections::HashMap;

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